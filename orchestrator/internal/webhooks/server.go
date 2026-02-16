package webhooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/monitor"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/mccloud/subgen/orchestrator/internal/util"
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
	DetectLanguage(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error)
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
	AudioContent      []byte            // For ASR tasks (Bazarr upload)
	ASROptions        map[string]string // ASR query parameters
}

// Server represents the webhook HTTP server
type Server struct {
	app        *fiber.App
	config     *config.Config
	queue      QueueInterface
	scanner    monitor.Scanner
	pathMapper *util.PathMapper
	grpcClient GRPCClientInterface // For direct worker communication (language detection, etc.)
	workerPool WorkerPoolInterface // For worker selection
	log        *logrus.Logger
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
		log:        log,
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
	s.app.Post("/jellyfin", s.handleJellyfin)
	s.app.Post("/emby", s.handleEmby)
	s.app.Post("/tautulli", s.handleTautulli)
	s.app.Post("/asr", s.handleASR)
	s.app.Post("/batch", s.handleBatch)
	s.app.Post("/detect-language", s.handleDetectLanguage)

	// Queue status and monitoring endpoints (STORY_07)
	s.app.Get("/queue/status", s.handleQueueStatus())
	s.app.Get("/queue/processing", s.handleQueueProcessing())
	s.app.Get("/queue/history", s.handleQueueHistory())
	s.app.Get("/tasks/:id", s.handleTaskStatus())
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

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		s.log.WithError(err).Error("Failed to enqueue Plex task")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to queue task",
		})
	}

	s.log.WithField("rating_key", ratingKey).Info("Plex task queued")
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

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		s.log.WithError(err).Error("Failed to enqueue Jellyfin task")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to queue task",
		})
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

	// Create task
	task := Task{
		FilePath: mappedPath,
	}

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		s.log.WithError(err).Error("Failed to enqueue Emby task")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to queue task",
		})
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

	// Create task
	task := Task{
		FilePath: mappedPath,
	}

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		s.log.WithError(err).Error("Failed to enqueue Tautulli task")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to queue task",
		})
	}

	s.log.WithField("file_path", file).Info("Tautulli task queued")
	return c.SendString("")
}

// handleASR processes direct audio transcription requests (for Bazarr integration)
func (s *Server) handleASR(c *fiber.Ctx) error {
	// Parse query parameters
	taskType := c.Query("task", "transcribe")
	language := c.Query("language", "")
	videoFile := c.Query("video_file", "")
	output := c.Query("output", "srt")

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

	// Create ASR task
	// In a real implementation, this would:
	// 1. Generate hash from audio content for deduplication
	// 2. Check if identical task is already processing
	// 3. Queue the task and block until completion
	// 4. Return the transcription result
	//
	// For now, we just queue it as a placeholder
	task := Task{
		FilePath:          videoFile,
		TranscriptionType: taskType,
		ForceLanguage:     language,
		AudioContent:      audioContent,
		ASROptions: map[string]string{
			"output": output,
		},
	}

	// Queue task
	if err := s.queue.Enqueue(task); err != nil {
		s.log.WithError(err).Error("Failed to enqueue ASR task")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to queue task",
		})
	}

	s.log.WithField("video_file", videoFile).Info("ASR task queued")

	// In real implementation, would block and return subtitle content
	// For now, return success
	return c.SendString("ASR task queued successfully (placeholder response)")
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
