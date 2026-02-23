package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/monitor"
	"github.com/mccloud/subgen/orchestrator/internal/observability"
	"github.com/mccloud/subgen/orchestrator/internal/plex"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/mccloud/subgen/orchestrator/internal/skip"
	"github.com/mccloud/subgen/orchestrator/internal/util"
	"github.com/mccloud/subgen/orchestrator/pkg/formats"
	pb "github.com/mccloud/subgen/orchestrator/pkg/pb"
	"github.com/sirupsen/logrus"
)

// QueueInterface defines the interface for task queueing
// Will be implemented by STORY_04 (Priority Queue System)
type QueueInterface interface {
	Enqueue(task Task) error
	Size() int
	ProcessingCount() int
	IsIdle() bool
	GetTaskInfo(taskID string) *queue.TaskInfo
	GetAllProcessingTaskInfo() []queue.TaskInfo
	GetHistory(limit, offset int) []queue.TaskInfo
	GetHistoryTotal() int
}

// GRPCClientInterface defines the interface for gRPC worker communication
type GRPCClientInterface interface {
	DetectLanguage(ctx context.Context, workerAddr string, filePath string, audioContent []byte, offset float64, length float64) (*pb.DetectLanguageResponse, error)
}

// WorkerPoolInterface defines the interface for worker pool
type WorkerPoolInterface interface {
	SelectWorker() (*Worker, error)
}

// Worker represents a discovered worker
type Worker struct {
	Address string
	Healthy bool
}

// Task represents a transcription task to be queued
type Task struct {
	FilePath          string
	TranscriptionType string // "transcribe" or "translate"
	ForceLanguage     string
	PlexItemID        string
	PlexServer        string
	PlexToken         string
	JellyfinItemID    string
	JellyfinServer    string
	JellyfinToken     string
	AudioContent      []byte                          // For ASR tasks (Bazarr upload)
	ASROptions        map[string]string               // ASR query parameters
	ResultChan        chan *queue.TranscriptionResult // For blocking operations (ASR)
}

// Server represents the webhook HTTP server
type Server struct {
	app           *fiber.App
	config        *config.Config
	queue         QueueInterface
	scanner       monitor.Scanner
	pathMapper    *util.PathMapper
	grpcClient    GRPCClientInterface    // For direct worker communication (language detection, etc.)
	workerPool    WorkerPoolInterface    // For worker selection
	skipChecker   skip.Checker           // For skip logic integration (STORY_07)
	metrics       *observability.Metrics // For observability metrics (STORY_07)
	plexClient    *plex.Client           // Plex API client (STORY_03)
	episodeQueuer *plex.EpisodeQueuer    // Episode queueing (STORY_03)
	startTime     time.Time              // Server start time for uptime calculation
	log           *logrus.Logger
}

// NewServer creates a new webhook server instance
// Note: Middleware should be registered BEFORE calling this via app.Use()
func NewServer(cfg *config.Config, queue QueueInterface, log *logrus.Logger) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Initialize path mapper
	pathMapper, err := util.NewPathMapper(
		cfg.PathMapping.Enabled,
		cfg.PathMapping.From,
		cfg.PathMapping.To,
	)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize path mapper")
	}

	if pathMapper.Enabled() {
		log.WithFields(logrus.Fields{
			"mappings": pathMapper.Mappings(),
		}).Info("Path mapping enabled")
	}

	s := &Server{
		app:        app,
		config:     cfg,
		queue:      queue,
		scanner:    nil, // Set via SetScanner() - optional dependency
		pathMapper: pathMapper,
		startTime:  time.Now(),
		log:        log,
	}

	// Initialize Plex client and episode queuer if Plex is configured (STORY_03)
	if cfg.Plex.Server != "" && cfg.Plex.Token != "" {
		s.plexClient = plex.NewClient(cfg.Plex.Server, cfg.Plex.Token)
		s.episodeQueuer = plex.NewEpisodeQueuer(s.plexClient, log)
		log.WithField("plex_server", cfg.Plex.Server).Info("Plex client initialized")
	}

	s.setupRoutes()
	return s
}

// SetScanner sets the scanner instance for batch processing
func (s *Server) SetScanner(scanner monitor.Scanner) {
	s.scanner = scanner
}

// SetGRPCClient sets the gRPC client for direct worker communication
func (s *Server) SetGRPCClient(client GRPCClientInterface) {
	s.grpcClient = client
}

// SetWorkerPool sets the worker pool for worker selection
func (s *Server) SetWorkerPool(pool WorkerPoolInterface) {
	s.workerPool = pool
}

// SetSkipChecker sets the skip checker for skip logic integration
func (s *Server) SetSkipChecker(checker skip.Checker) {
	s.skipChecker = checker
}

// SetMetrics sets the metrics for observability
func (s *Server) SetMetrics(metrics *observability.Metrics) {
	s.metrics = metrics
}

// App returns the underlying Fiber app for middleware registration
func (s *Server) App() *fiber.App {
	return s.app
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// GET handlers (return error messages)
	s.app.Get("/plex", s.handleGetError)
	s.app.Get("/webhook", s.handleGetError)
	s.app.Get("/jellyfin", s.handleGetError)
	s.app.Get("/emby", s.handleGetError)
	s.app.Get("/tautulli", s.handleGetError)
	s.app.Get("/asr", s.handleGetError)

	// Status endpoints
	s.app.Get("/", s.handleRoot)
	s.app.Get("/status", s.handleStatus)

	// POST handlers
	s.app.Post("/plex", s.handlePlex)
	s.app.Post("/webhook/plex", s.handlePlex) // Plex calls /webhook/plex directly
	s.app.Post("/jellyfin", s.handleJellyfin)
	s.app.Post("/webhook/jellyfin", s.handleJellyfin)
	s.app.Post("/emby", s.handleEmby)
	s.app.Post("/webhook/emby", s.handleEmby)
	s.app.Post("/tautulli", s.handleTautulli)
	s.app.Post("/webhook/tautulli", s.handleTautulli)
	s.app.Post("/asr", s.handleASR)
	s.app.Post("/batch", s.handleBatch)
	s.app.Post("/detect-language", s.handleDetectLanguage)

	// Queue status and monitoring endpoints (STORY_07)
	s.app.Get("/queue/status", s.handleQueueStatus())
	s.app.Get("/queue/processing", s.handleQueueProcessing())
	s.app.Get("/queue/history", s.handleQueueHistory())
	s.app.Get("/tasks/:id", s.handleTaskStatus())

	// Health check endpoints (Kubernetes/Docker probes)
	s.app.Get("/health", s.handleHealth)
	s.app.Get("/ready", s.handleReady)
	s.app.Get("/live", s.handleLive)

	// K8s-friendly health check aliases
	s.app.Get("/healthz", s.handleHealth) // K8s liveness standard
	s.app.Get("/livez", s.handleLive)     // K8s liveness standard
	s.app.Get("/readyz", s.handleReady)   // K8s readiness standard
}

// Start begins listening for webhook requests
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.WebhookPort)
	s.log.Infof("Starting webhook server on %s", addr)
	return s.app.Listen(addr)
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}

// handleGetError returns an error message for GET requests
func (s *Server) handleGetError(c *fiber.Ctx) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": "You accessed this request incorrectly via a GET request. See https://github.com/McCloudS/subgen for proper configuration",
	})
}

// handleRoot returns a message about the removed webui
func (s *Server) handleRoot(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "The webui for configuration was removed on 1 October 2024, please configure via environment variables or in your Docker settings.",
	})
}

// handleStatus returns server status and version information
func (s *Server) handleStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"version": "Subgen Go Orchestrator v0.1.0",
		"status":  "operational",
	})
}

// PlexPayload represents the Plex webhook payload structure
type PlexPayload struct {
	Event    string `json:"event"`
	Metadata struct {
		RatingKey string `json:"ratingKey"`
	} `json:"Metadata"`
}

// handlePlex processes Plex webhook notifications
func (s *Server) handlePlex(c *fiber.Ctx) error {
	// Validate User-Agent
	userAgent := c.Get("User-Agent")
	if userAgent == "" || !contains(userAgent, "PlexMediaServer") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "This doesn't appear to be a properly configured Plex webhook, please review the instructions again",
		})
	}

	// Parse form-encoded payload
	payloadStr := c.FormValue("payload")
	if payloadStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing payload field",
		})
	}

	var payload PlexPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		s.log.WithError(err).Error("Failed to parse Plex payload")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON payload",
		})
	}

	s.log.WithField("event", payload.Event).Debug("Plex event detected")

	// Check if event should be processed
	shouldProcess := false
	if payload.Event == "library.new" && s.config.ProcessAddedMedia {
		shouldProcess = true
	} else if payload.Event == "media.play" && s.config.ProcessMediaOnPlay {
		shouldProcess = true
	}

	if !shouldProcess {
		s.log.WithField("event", payload.Event).Debug("Event filtered, not processing")
		return c.SendString("")
	}

	// Extract rating key
	ratingKey := payload.Metadata.RatingKey
	if ratingKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing ratingKey in metadata",
		})
	}

	// Create task (in real implementation, would fetch file path from Plex API)
	task := Task{
		PlexItemID: ratingKey,
		PlexServer: s.config.Plex.Server,
		PlexToken:  s.config.Plex.Token,
	}

	// STORY_07 LIMITATION: Skip checking for Plex requires fetching file path from Plex API first
	// The Plex webhook only provides a ratingKey (item ID), not a file path.
	// To implement skip checking here, we would need to:
	// 1. Call Plex API with ratingKey to get file path: GET /library/metadata/{ratingKey}
	// 2. Extract file path from the API response
	// 3. Apply path mapping to convert Plex's path to local path
	// 4. Run skip check on the mapped path
	// 5. If skipped, increment metrics and return early
	//
	// This is deferred because it requires:
	// - Plex API client implementation
	// - Authentication token management
	// - Error handling for API failures
	// - Testing with live Plex instances
	//
	// For now, skip checking is only available for Emby/Tautulli webhooks which
	// provide file paths directly in their payloads.
	//
	// if s.skipChecker != nil && s.plexClient != nil {
	//     filePath, err := s.plexClient.GetFilePath(c.Context(), ratingKey)
	//     if err != nil {
	//         s.log.WithError(err).Warn("Failed to fetch file path from Plex")
	//     } else {
	//         mappedPath, err := s.pathMapper.Map(filePath)
	//         if err != nil {
	//             s.log.WithError(err).Warn("Path mapping failed for Plex file")
	//         } else {
	//             result, err := s.skipChecker.Check(c.Context(), mappedPath)
	//             if err != nil {
	//                 s.log.WithError(err).Warn("Skip check failed, continuing with queue")
	//             } else if result.ShouldSkip {
	//                 s.log.WithFields(logrus.Fields{
	//                     "reason":  result.Reason,
	//                     "details": result.Details,
	//                 }).Info("File skipped")
	//                 if s.metrics != nil {
	//                     s.metrics.FilesSkipped.WithLabelValues(string(result.Reason)).Inc()
	//                 }
	//                 return c.SendString("")
	//             }
	//         }
	//     }
	// }

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		// Handle different error types with appropriate HTTP status codes
		if err == queue.ErrDuplicateTask {
			// Duplicate task - idempotent behavior, return success
			s.log.WithError(err).Debug("Task already queued, returning OK")
			return c.SendString("OK")
		} else if err == queue.ErrQueueFull {
			// Queue full - rate limiting
			s.log.WithError(err).Warn("Queue is full, cannot accept task")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Queue is full, please try again later",
			})
		} else {
			// Other errors - internal server error
			s.log.WithError(err).Error("Failed to enqueue Plex task")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to queue task",
			})
		}
	}

	s.log.WithField("rating_key", ratingKey).Info("Plex task queued")

	// STORY_03: Queue additional episodes if episode queueing is configured
	if s.episodeQueuer != nil && s.plexClient != nil {
		queueMode := s.getPlexQueueMode()
		if queueMode != "" {
			s.log.WithFields(logrus.Fields{
				"rating_key": ratingKey,
				"mode":       queueMode,
			}).Debug("Attempting to queue additional episodes")

			itemIDs, err := s.episodeQueuer.QueueEpisodes(c.Context(), ratingKey, queueMode)
			if err != nil {
				s.log.WithError(err).Warn("Failed to queue additional episodes")
			} else if len(itemIDs) > 0 {
				// Queue each additional episode
				for _, itemID := range itemIDs {
					filePath, err := s.episodeQueuer.GetFilePath(c.Context(), itemID)
					if err != nil {
						s.log.WithError(err).WithField("item_id", itemID).Warn("Failed to get file path for episode")
						continue
					}

					// Apply path mapping
					mappedPath, err := s.pathMapper.Map(filePath)
					if err != nil {
						s.log.WithError(err).WithField("item_id", itemID).Warn("Failed to map path for episode")
						continue
					}

					episodeTask := Task{
						FilePath:   mappedPath,
						PlexItemID: itemID,
						PlexServer: s.config.Plex.Server,
						PlexToken:  s.config.Plex.Token,
					}

					if err := s.queue.Enqueue(episodeTask); err != nil {
						s.log.WithError(err).WithField("item_id", itemID).Warn("Failed to enqueue additional episode")
					} else {
						s.log.WithField("item_id", itemID).Debug("Additional episode queued")
					}
				}
			}
		}
	}

	return c.SendString("")
}

// handleJellyfin processes Jellyfin webhook notifications
func (s *Server) handleJellyfin(c *fiber.Ctx) error {
	// Validate User-Agent
	userAgent := c.Get("User-Agent")
	if userAgent == "" || !contains(userAgent, "Jellyfin-Server") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "This doesn't appear to be a properly configured Jellyfin webhook, please review the instructions again",
		})
	}

	// Parse form fields
	notificationType := c.FormValue("NotificationType")
	itemID := c.FormValue("ItemId")

	if itemID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing ItemId field",
		})
	}

	s.log.WithField("event", notificationType).Debug("Jellyfin event detected")

	// Check if event should be processed
	shouldProcess := false
	if notificationType == "ItemAdded" && s.config.ProcessAddedMedia {
		shouldProcess = true
	} else if notificationType == "PlaybackStart" && s.config.ProcessMediaOnPlay {
		shouldProcess = true
	}

	if !shouldProcess {
		s.log.WithField("event", notificationType).Debug("Event filtered, not processing")
		return c.SendString("")
	}

	// Create task (in real implementation, would fetch file path from Jellyfin API)
	task := Task{
		JellyfinItemID: itemID,
		JellyfinServer: s.config.Jellyfin.Server,
		JellyfinToken:  s.config.Jellyfin.Token,
	}

	// STORY_07 LIMITATION: Skip checking for Jellyfin requires fetching file path from Jellyfin API first
	// The Jellyfin webhook only provides an ItemId, not a file path.
	// To implement skip checking here, we would need to:
	// 1. Call Jellyfin API with ItemId to get file path: GET /Items/{itemId}
	// 2. Extract file path from the API response (Path field in MediaSources)
	// 3. Apply path mapping to convert Jellyfin's path to local path
	// 4. Run skip check on the mapped path
	// 5. If skipped, increment metrics and return early
	//
	// This is deferred because it requires:
	// - Jellyfin API client implementation
	// - Authentication token management
	// - Error handling for API failures
	// - Testing with live Jellyfin instances
	//
	// For now, skip checking is only available for Emby/Tautulli webhooks which
	// provide file paths directly in their payloads.
	//
	// if s.skipChecker != nil && s.jellyfinClient != nil {
	//     filePath, err := s.jellyfinClient.GetFilePath(c.Context(), itemID)
	//     if err != nil {
	//         s.log.WithError(err).Warn("Failed to fetch file path from Jellyfin")
	//     } else {
	//         mappedPath, err := s.pathMapper.Map(filePath)
	//         if err != nil {
	//             s.log.WithError(err).Warn("Path mapping failed for Jellyfin file")
	//         } else {
	//             result, err := s.skipChecker.Check(c.Context(), mappedPath)
	//             if err != nil {
	//                 s.log.WithError(err).Warn("Skip check failed, continuing with queue")
	//             } else if result.ShouldSkip {
	//                 s.log.WithFields(logrus.Fields{
	//                     "reason":  result.Reason,
	//                     "details": result.Details,
	//                 }).Info("File skipped")
	//                 if s.metrics != nil {
	//                     s.metrics.FilesSkipped.WithLabelValues(string(result.Reason)).Inc()
	//                 }
	//                 return c.SendString("")
	//             }
	//         }
	//     }
	// }

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		// Handle different error types with appropriate HTTP status codes
		if err == queue.ErrDuplicateTask {
			s.log.WithError(err).Debug("Task already queued, returning OK")
			return c.SendString("")
		} else if err == queue.ErrQueueFull {
			s.log.WithError(err).Warn("Queue is full, cannot accept task")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Queue is full, please try again later",
			})
		} else {
			s.log.WithError(err).Error("Failed to enqueue Jellyfin task")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to queue task",
			})
		}
	}

	s.log.WithField("item_id", itemID).Info("Jellyfin task queued")
	return c.SendString("")
}

// EmbyPayload represents the Emby webhook payload structure
type EmbyPayload struct {
	Event string `json:"Event"`
	Item  struct {
		Path string `json:"Path"`
	} `json:"Item"`
}

// handleEmby processes Emby webhook notifications
func (s *Server) handleEmby(c *fiber.Ctx) error {
	// Parse form-encoded data field
	dataStr := c.FormValue("data")
	if dataStr == "" {
		// Empty data is acceptable per legacy implementation
		return c.SendString("")
	}

	var payload EmbyPayload
	if err := json.Unmarshal([]byte(dataStr), &payload); err != nil {
		s.log.WithError(err).Error("Failed to parse Emby payload")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid JSON payload",
		})
	}

	s.log.WithField("event", payload.Event).Debug("Emby event detected")

	// Handle test notification
	if payload.Event == "system.notificationtest" {
		s.log.Info("Emby test message received!")
		return c.JSON(fiber.Map{
			"message": "Notification test received successfully!",
		})
	}

	// Check if event should be processed
	shouldProcess := false
	if payload.Event == "library.new" && s.config.ProcessAddedMedia {
		shouldProcess = true
	} else if payload.Event == "playback.start" && s.config.ProcessMediaOnPlay {
		shouldProcess = true
	}

	if !shouldProcess {
		s.log.WithField("event", payload.Event).Debug("Event filtered, not processing")
		return c.SendString("")
	}

	// Extract file path
	filePath := payload.Item.Path
	if filePath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing file path in Item",
		})
	}

	// Apply path mapping
	mappedPath, err := s.pathMapper.Map(filePath)
	if err != nil {
		s.log.WithError(err).WithFields(logrus.Fields{
			"original_path": filePath,
		}).Error("Path mapping failed")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Path mapping failed: %v", err),
		})
	}

	s.log.WithFields(logrus.Fields{
		"original_path": filePath,
		"mapped_path":   mappedPath,
	}).Debug("Path mapping applied")

	// STORY_07: Check if file should be skipped
	if s.skipChecker != nil {
		result, err := s.skipChecker.Check(c.Context(), mappedPath)
		if err != nil {
			s.log.WithError(err).Warn("Skip check failed, continuing with queue")
		} else if result.ShouldSkip {
			s.log.WithFields(logrus.Fields{
				"reason":  result.Reason,
				"details": result.Details,
			}).Info("File skipped")

			// Record skip metric
			if s.metrics != nil {
				s.metrics.FilesSkipped.WithLabelValues(string(result.Reason)).Inc()
			}

			return c.SendString("OK")
		}
	}

	// Create task
	task := Task{
		FilePath: mappedPath,
	}

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		// Handle different error types with appropriate HTTP status codes
		if err == queue.ErrDuplicateTask {
			s.log.WithError(err).Debug("Task already queued, returning OK")
			return c.SendString("")
		} else if err == queue.ErrQueueFull {
			s.log.WithError(err).Warn("Queue is full, cannot accept task")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Queue is full, please try again later",
			})
		} else {
			s.log.WithError(err).Error("Failed to enqueue Emby task")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to queue task",
			})
		}
	}

	s.log.WithField("file_path", filePath).Info("Emby task queued")
	return c.SendString("")
}

// handleTautulli processes Tautulli webhook notifications
func (s *Server) handleTautulli(c *fiber.Ctx) error {
	// Validate source header
	source := c.Get("source")
	if source != "Tautulli" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "This doesn't appear to be a properly configured Tautulli webhook, please review the instructions again",
		})
	}

	// Parse form fields
	event := c.FormValue("event")
	file := c.FormValue("file")

	s.log.WithField("event", event).Debug("Tautulli event detected")

	// Check if event should be processed
	shouldProcess := false
	if event == "added" && s.config.ProcessAddedMedia {
		shouldProcess = true
	} else if event == "played" && s.config.ProcessMediaOnPlay {
		shouldProcess = true
	}

	if !shouldProcess {
		s.log.WithField("event", event).Debug("Event filtered, not processing")
		return c.SendString("")
	}

	if file == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing file field",
		})
	}

	// Apply path mapping
	mappedPath, err := s.pathMapper.Map(file)
	if err != nil {
		s.log.WithError(err).WithFields(logrus.Fields{
			"original_path": file,
		}).Error("Path mapping failed")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Path mapping failed: %v", err),
		})
	}

	s.log.WithFields(logrus.Fields{
		"original_path": file,
		"mapped_path":   mappedPath,
	}).Debug("Path mapping applied")

	// STORY_07: Check if file should be skipped
	if s.skipChecker != nil {
		result, err := s.skipChecker.Check(c.Context(), mappedPath)
		if err != nil {
			s.log.WithError(err).Warn("Skip check failed, continuing with queue")
		} else if result.ShouldSkip {
			s.log.WithFields(logrus.Fields{
				"reason":  result.Reason,
				"details": result.Details,
			}).Info("File skipped")

			// Record skip metric
			if s.metrics != nil {
				s.metrics.FilesSkipped.WithLabelValues(string(result.Reason)).Inc()
			}

			return c.SendString("OK")
		}
	}

	// Create task
	task := Task{
		FilePath: mappedPath,
	}

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		// Handle different error types with appropriate HTTP status codes
		if err == queue.ErrDuplicateTask {
			s.log.WithError(err).Debug("Task already queued, returning OK")
			return c.SendString("")
		} else if err == queue.ErrQueueFull {
			s.log.WithError(err).Warn("Queue is full, cannot accept task")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Queue is full, please try again later",
			})
		} else {
			s.log.WithError(err).Error("Failed to enqueue Tautulli task")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to queue task",
			})
		}
	}

	s.log.WithField("file_path", file).Info("Tautulli task queued")
	return c.SendString("")
}

// handleASR processes direct audio transcription requests (for Bazarr integration)
func (s *Server) handleASR(c *fiber.Ctx) error {
	// Parse parameters (can be query params or form values)
	taskType := c.FormValue("task", c.Query("task", "transcribe"))
	language := c.FormValue("language", c.Query("language", ""))
	videoFile := c.FormValue("video_file", c.Query("video_file", ""))
	output := c.FormValue("output", c.Query("output", "srt"))

	// Parse form data for ASR options
	wordLevelHighlight := c.FormValue("word_level_highlight", "false") == "true"
	customRegroup := c.FormValue("custom_regroup", "")
	customPrompt := c.FormValue("custom_prompt", "")
	appendFooter := c.FormValue("append_footer", "false") == "true"
	subtitleLanguageName := c.FormValue("subtitle_language_name", "")
	showModelInFilename := c.FormValue("show_model_in_filename", "true") == "true"
	showSubgenInFilename := c.FormValue("show_subgen_in_filename", "true") == "true"

	// STORY_05: Normalize format to lowercase for case-insensitive comparison
	output = strings.ToLower(strings.TrimSpace(output))

	// STORY_05: Validate format
	validFormats := map[string]bool{
		"srt":  true,
		"vtt":  true,
		"lrc":  true,
		"txt":  true,
		"tsv":  true,
		"json": true,
	}
	if !validFormats[output] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("invalid format: %s (supported: srt, vtt, lrc, txt, tsv, json)", output),
		})
	}

	// Apply path mapping if video_file is provided
	if videoFile != "" {
		mappedPath, err := s.pathMapper.Map(videoFile)
		if err != nil {
			s.log.WithError(err).WithFields(logrus.Fields{
				"original_path": videoFile,
			}).Error("Path mapping failed for ASR video_file")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Path mapping failed: %v", err),
			})
		}

		s.log.WithFields(logrus.Fields{
			"original_path": videoFile,
			"mapped_path":   mappedPath,
		}).Debug("Path mapping applied for ASR")

		videoFile = mappedPath
	}

	s.log.WithFields(map[string]interface{}{
		"task":       taskType,
		"language":   language,
		"video_file": videoFile,
		"output":     output,
	}).Info("ASR request received")

	// Get uploaded audio file
	file, err := c.FormFile("audio_file")
	if err != nil {
		s.log.WithError(err).Error("Failed to get audio file")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing or invalid audio_file field",
		})
	}

	// Validate file size (GAP #4: prevent OOM from huge uploads)
	if file.Size > s.config.Queue.MaxAudioContentSize {
		s.log.WithFields(map[string]interface{}{
			"file_size": file.Size,
			"max_size":  s.config.Queue.MaxAudioContentSize,
		}).Error("Audio file exceeds maximum size")
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
			"error": fmt.Sprintf("Audio file exceeds maximum size of %d bytes", s.config.Queue.MaxAudioContentSize),
		})
	}

	// Validate file is not empty
	if file.Size == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Audio file is empty",
		})
	}

	// Read audio content
	fileHandle, err := file.Open()
	if err != nil {
		s.log.WithError(err).Error("Failed to open audio file")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read audio file",
		})
	}
	defer fileHandle.Close()

	// Read file content
	audioContent := make([]byte, file.Size)
	n, err := fileHandle.Read(audioContent)
	if err != nil && n == 0 {
		s.log.Error("Failed to read audio file content")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read audio file content",
		})
	}

	// Trim to actual bytes read
	audioContent = audioContent[:n]

	// Create buffered result channel for blocking operation
	resultChan := make(chan *queue.TranscriptionResult, 1)

	// Create ASR task with result channel
	task := Task{
		FilePath:          videoFile,
		TranscriptionType: taskType,
		ForceLanguage:     language,
		AudioContent:      audioContent,
		ASROptions: map[string]string{
			"output":                  output,
			"word_level_highlight":    strconv.FormatBool(wordLevelHighlight),
			"custom_regroup":          customRegroup,
			"custom_prompt":           customPrompt,
			"append_footer":           strconv.FormatBool(appendFooter),
			"subtitle_language_name":  subtitleLanguageName,
			"show_model_in_filename":  strconv.FormatBool(showModelInFilename),
			"show_subgen_in_filename": strconv.FormatBool(showSubgenInFilename),
		},
		ResultChan: resultChan, // Enable blocking
	}

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		close(resultChan) // Clean up channel
		// Handle different error types with appropriate HTTP status codes
		if err == queue.ErrDuplicateTask {
			s.log.WithError(err).Debug("Task already queued, returning conflict")
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Task already queued or processing",
			})
		} else if err == queue.ErrQueueFull {
			s.log.WithError(err).Warn("Queue is full, cannot accept task")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Queue is full, please try again later",
			})
		} else {
			s.log.WithError(err).Error("Failed to enqueue ASR task")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to queue task",
			})
		}
	}

	s.log.WithFields(map[string]interface{}{
		"video_file": videoFile,
		"format":     output,
	}).Info("ASR task queued, waiting for result")

	// Block until result ready or timeout
	timeout := 30 * time.Second
	if s.config.ASR.Timeout > 0 {
		timeout = s.config.ASR.Timeout
	}

	select {
	case result := <-resultChan:
		// Handle transcription error
		if result.Error != nil {
			s.log.WithError(result.Error).Error("ASR transcription failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("transcription failed: %v", result.Error),
			})
		}

		// STORY_05: Convert segments to requested format
		s.log.WithFields(map[string]interface{}{
			"segments": len(result.Segments),
			"language": result.Metadata.Language,
			"duration": result.Metadata.Duration,
			"format":   output,
		}).Info("ASR transcription completed, converting to format")

		// Convert queue.Segment to formats.Segment
		formatSegments := make([]formats.Segment, len(result.Segments))
		for i, seg := range result.Segments {
			formatSegments[i] = formats.Segment{
				Start: seg.Start,
				End:   seg.End,
				Text:  seg.Text,
			}
		}

		// Convert queue.Metadata to formats.Metadata
		formatMetadata := formats.Metadata{
			Language: result.Metadata.Language,
			Duration: result.Metadata.Duration,
		}

		// Use format writer to convert segments
		var buffer bytes.Buffer
		writer, err := formats.NewWriter(output)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("unsupported format: %s", output),
			})
		}

		if err := writer.Write(&buffer, formatSegments, formatMetadata); err != nil {
			s.log.WithError(err).Error("Format conversion failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("format conversion failed: %v", err),
			})
		}

		// Set Content-Type header based on format
		c.Set("Content-Type", getContentType(output))

		// Return formatted subtitles
		return c.SendString(buffer.String())

	case <-time.After(timeout):
		s.log.WithField("timeout", timeout).Warn("ASR transcription timeout")
		return c.Status(fiber.StatusGatewayTimeout).JSON(fiber.Map{
			"error": fmt.Sprintf("transcription timeout after %v", timeout),
		})
	}
}

// handleHealth is the liveness probe endpoint
// Returns 200 if the orchestrator process is alive
// This endpoint should never return 5xx (otherwise K8s will restart the pod)
func (s *Server) handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "alive",
		"timestamp": time.Now().Unix(),
	})
}

// handleReady is the readiness probe endpoint
// Returns 200 if orchestrator is ready to accept traffic
// Returns 503 if alive but not ready (don't send traffic)
func (s *Server) handleReady(c *fiber.Ctx) error {
	// Check if worker pool is initialized
	if s.workerPool == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not_ready",
			"reason": "worker_pool_not_initialized",
		})
	}

	// Check if any workers are available
	_, err := s.workerPool.SelectWorker()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not_ready",
			"reason": "no_workers_available",
		})
	}

	// Check if queue is overloaded (more than 10000 tasks)
	queueSize := s.queue.Size()
	if queueSize > 10000 {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status":     "not_ready",
			"reason":     "queue_overloaded",
			"queue_size": queueSize,
		})
	}

	// Orchestrator is ready
	return c.JSON(fiber.Map{
		"status":            "ready",
		"queue_size":        queueSize,
		"workers_available": 1, // At least 1 worker is available if SelectWorker succeeded
	})
}

// handleLive is an alternative liveness probe endpoint
// Same as /health but includes uptime information
func (s *Server) handleLive(c *fiber.Ctx) error {
	uptimeSeconds := int64(time.Since(s.startTime).Seconds())
	return c.JSON(fiber.Map{
		"status":         "alive",
		"uptime_seconds": uptimeSeconds,
		"timestamp":      time.Now().Unix(),
	})
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// getPlexQueueMode returns the configured Plex queue mode (STORY_03)
func (s *Server) getPlexQueueMode() plex.QueueMode {
	if s.config.Plex.QueueNextEpisode {
		return plex.QueueModeNext
	}
	if s.config.Plex.QueueSeason {
		return plex.QueueModeSeason
	}
	if s.config.Plex.QueueSeries {
		return plex.QueueModeSeries
	}
	return ""
}

// getContentType returns the appropriate Content-Type header for a subtitle format (STORY_05)
func getContentType(format string) string {
	switch format {
	case "vtt":
		return "text/vtt; charset=utf-8"
	case "json":
		return "application/json; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}
