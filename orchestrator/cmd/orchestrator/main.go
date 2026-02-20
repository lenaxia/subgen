package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/discovery"
	"github.com/mccloud/subgen/orchestrator/internal/grpc_client"
	"github.com/mccloud/subgen/orchestrator/internal/mediaserver"
	"github.com/mccloud/subgen/orchestrator/internal/middleware"
	"github.com/mccloud/subgen/orchestrator/internal/monitor"
	"github.com/mccloud/subgen/orchestrator/internal/observability"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/mccloud/subgen/orchestrator/internal/skip"
	"github.com/mccloud/subgen/orchestrator/internal/webhooks"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"net/http"
)

// Version is the orchestrator version (set at build time)
var Version = "dev"

// BuildTime is the build timestamp (set at build time via ldflags)
var BuildTime = "unknown"

// GitCommit is the git commit hash (set at build time via ldflags)
var GitCommit = "unknown"

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	GoVersion string
	BuildTime string
	GitCommit string
}

// GetBuildInfo returns build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		GoVersion: runtime.Version(),
		BuildTime: BuildTime,
		GitCommit: GitCommit,
	}
}

func main() {
	// Check for --health flag (Docker health check)
	if len(os.Args) > 1 && os.Args[1] == "--health" {
		if err := CheckHealth(); err != nil {
			fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("OK")
		os.Exit(0)
	}

	// Check for --version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		PrintVersion()
		return
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logging with config log level
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Print startup banner
	buildInfo := GetBuildInfo()
	printStartupBanner(cfg, buildInfo, log)

	// Log structured startup info
	log.WithFields(logrus.Fields{
		"version":    buildInfo.Version,
		"go_version": buildInfo.GoVersion,
		"build_time": buildInfo.BuildTime,
		"git_commit": buildInfo.GitCommit,
	}).Info("Subgen Orchestrator starting")

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.WithField("signal", sig).Info("Received shutdown signal")
		cancel()
	}()

	// Track start time for uptime
	startTime := time.Now()

	// Initialize observability metrics
	obsMetrics := observability.NewMetrics()
	obsMetrics.SetUp()

	// Initialize queue with metrics
	queueMetrics := queue.NewQueueMetrics()
	taskQueue := queue.NewQueue(cfg.Queue.MaxSize, queueMetrics, log)

	log.WithField("max_size", cfg.Queue.MaxSize).Info("Queue initialized")

	// Initialize media server clients
	var plexClient *mediaserver.PlexClient
	var jellyfinClient *mediaserver.JellyfinClient

	clientConfig := mediaserver.DefaultClientConfig()

	if cfg.Plex.Enabled {
		plexClient = mediaserver.NewPlexClient(
			cfg.Plex.Server,
			cfg.Plex.Token,
			clientConfig,
			log,
		)
		log.WithField("server", cfg.Plex.Server).Info("Plex client initialized")
	}

	if cfg.Jellyfin.Enabled {
		jellyfinClient = mediaserver.NewJellyfinClient(
			cfg.Jellyfin.Server,
			cfg.Jellyfin.Token,
			clientConfig,
			log,
		)
		log.WithField("server", cfg.Jellyfin.Server).Info("Jellyfin client initialized")
	}

	// Initialize worker discovery and pool
	workerDiscovery, err := discovery.NewDiscovery(cfg, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create worker discovery")
	}

	// Use round-robin strategy (TODO: make configurable)
	workerPool := discovery.NewPool(
		workerDiscovery,
		discovery.RoundRobin,
		log,
	)

	// Start worker pool
	if err := workerPool.Start(ctx); err != nil {
		log.WithError(err).Fatal("Failed to start worker pool")
	}

	log.WithFields(logrus.Fields{
		"discovery": cfg.Worker.Discovery,
		"strategy":  "round_robin",
	}).Info("Worker pool started")

	// Initialize gRPC client
	grpcMetrics := grpc_client.NewClientMetrics()
	grpcClient := grpc_client.NewClient(
		time.Duration(cfg.Worker.Timeout)*time.Second, // Transcribe timeout
		5*time.Second, // Health check timeout
		3,             // Max retries
		1*time.Second, // Retry delay
		grpcMetrics,
		log,
	)
	defer grpcClient.Close()

	log.Info("gRPC client initialized")

	// Create worker pool adapter for observability
	workerPoolAdapter := &WorkerPoolAdapter{pool: workerPool}

	// Create queue adapter for webhook server
	queueAdapter := webhooks.NewQueueAdapter(taskQueue)

	// Initialize webhook server
	webhookServer := webhooks.NewServer(cfg, queueAdapter, log)

	// Set gRPC client and worker pool for language detection endpoint
	webhookServer.SetGRPCClient(grpcClient)

	// Create worker pool adapter for webhooks
	webhookWorkerPool := &WebhookWorkerPoolAdapter{pool: workerPool}
	webhookServer.SetWorkerPool(webhookWorkerPool)

	// Register observability middleware
	app := webhookServer.App()
	app.Use(middleware.RequestID(log))
	app.Use(observability.PanicRecoveryMiddleware(log))
	app.Use(observability.RequestLoggerMiddleware(obsMetrics, log))

	// Register health endpoints
	observability.RegisterHealthEndpoints(
		app,
		obsMetrics,
		workerPoolAdapter,
		taskQueue,
		startTime,
		log,
	)

	// Start webhook server in goroutine
	go func() {
		if err := webhookServer.Start(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Webhook server failed")
		}
	}()

	log.WithField("port", cfg.WebhookPort).Info("Webhook server started")

	// Start Prometheus metrics server
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.MetricsPort),
		Handler: promhttp.Handler(),
	}

	go func() {
		log.WithField("port", cfg.MetricsPort).Info("Metrics server starting")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("Metrics server failed")
		}
	}()

	// Start task dispatcher (dequeues and dispatches to workers)
	go func() {
		taskDispatcher := &TaskDispatcher{
			queue:          taskQueue,
			workerPool:     workerPool,
			grpcClient:     grpcClient,
			plexClient:     plexClient,
			jellyfinClient: jellyfinClient,
			log:            log,
		}
		taskDispatcher.Run(ctx)
	}()

	log.Info("Task dispatcher started")

	// Start stale task cleanup goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				timeout := time.Duration(cfg.Transcription.ModelCleanupDelay) * time.Second
				if timeout == 0 {
					timeout = 1 * time.Hour // Default 1 hour
				}
				cleaned := taskQueue.CleanupStaleTasks(timeout)
				if cleaned > 0 {
					log.WithField("count", cleaned).Warn("Cleaned up stale tasks")
				}
			}
		}
	}()

	// Start file monitoring if enabled
	if cfg.Monitor.Enabled {
		log.WithFields(logrus.Fields{
			"folders":      cfg.Monitor.TranscribeFolders,
			"scan_startup": cfg.Monitor.ScanOnStartup,
		}).Info("File monitoring enabled")

		// Create monitoring configuration
		monitorConfig := &monitor.Config{
			Enabled:          cfg.Monitor.Enabled,
			Folders:          cfg.Monitor.TranscribeFolders,
			StabilityChecks:  cfg.Monitor.StabilityChecks,
			StabilityWait:    time.Duration(cfg.Monitor.StabilityWait) * time.Second,
			StabilityTimeout: time.Duration(cfg.Monitor.StabilityTimeout) * time.Second,
		}

		// Create skip checker FIRST (before file watcher callback needs it)
		queueAdapter := &QueueAdapter{queue: taskQueue}

		// Convert config.SkipConfig to skip.Config
		skipConfig := &skip.Config{
			SkipIfTargetSubtitleExists:      cfg.Skip.IfTargetSubtitlesExist,
			CheckEmbeddedSubtitles:          true, // Default to true
			SkipIfInternalSubtitlesLanguage: cfg.Skip.IfInternalSubtitlesLang,
			SkipIfExternalSubtitlesExist:    cfg.Skip.IfExternalSubtitlesExist,
			SkipOnlySubgenSubtitles:         cfg.Skip.OnlySubgenSubtitles,
			SkipSubtitleLanguages:           cfg.Skip.SubtitleLanguages,
			SkipIfAudioLanguages:            cfg.Skip.AudioLanguages,
			PreferredAudioLanguages:         cfg.Skip.PreferredAudioLanguages,
			LimitToPreferredAudioLanguage:   cfg.Skip.LimitToPreferredAudioLanguage,
		}

		skipChecker, err := skip.NewBasicChecker(skipConfig)
		if err != nil {
			log.WithError(err).Fatal("Failed to create skip checker")
		}
		scanner := monitor.NewScannerWithLogger(queueAdapter, skipChecker, log)
		webhookServer.SetScanner(scanner)

		// Create callback for file watcher (NOW has access to skipChecker)
		fileCallback := func(filePath string) {
			// Check skip logic first
			result, checkErr := skipChecker.Check(ctx, filePath)
			if checkErr != nil {
				log.WithError(checkErr).Warnf("Skip check failed for %s, will process anyway", filePath)
			} else if result != nil && result.ShouldSkip {
				log.WithFields(logrus.Fields{
					"file":    filePath,
					"reason":  result.Reason,
					"details": result.Details,
				}).Info("Skipping monitored file (skip logic)")
				return
			}

			// Create transcription task
			task := queue.NewTask(filePath, queue.TaskTypeTranscribe)

			if err := taskQueue.Enqueue(task); err != nil {
				log.WithError(err).Errorf("Failed to enqueue monitored file: %s", filePath)
			} else {
				log.WithField("file", filePath).Info("Queued monitored file for transcription")
			}
		}

		// Create file watcher
		watcher, err := monitor.NewFileWatcher(
			cfg.Monitor.TranscribeFolders,
			fileCallback,
			monitorConfig,
			log,
		)
		if err != nil {
			log.WithError(err).Fatal("Failed to create file watcher")
		}

		// Perform startup scan if enabled
		if cfg.Monitor.ScanOnStartup {
			log.Info("Performing startup scan...")

			for _, folder := range cfg.Monitor.TranscribeFolders {
				result, err := scanner.ScanDirectory(folder, true, cfg.Transcription.SubtitleLanguageName)
				if err != nil {
					log.WithError(err).Warnf("Startup scan failed for folder: %s", folder)
					continue
				}

				log.WithFields(logrus.Fields{
					"folder":  folder,
					"scanned": result.Scanned,
					"queued":  result.Queued,
					"skipped": result.Skipped,
				}).Info("Startup scan completed")
			}
		}

		// Start file watcher in background
		go func() {
			if err := watcher.Watch(ctx); err != nil && err != context.Canceled {
				log.WithError(err).Error("File watcher stopped unexpectedly")
			}
		}()

		log.Info("File monitoring started")
	}

	log.Info("Orchestrator initialized successfully")

	// Wait for shutdown signal
	<-ctx.Done()

	log.Info("Shutting down gracefully")

	// Shutdown webhook server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := webhookServer.Shutdown(); err != nil {
		log.WithError(err).Error("Error shutting down webhook server")
	}

	// Shutdown metrics server
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("Error shutting down metrics server")
	}

	log.Info("Shutdown complete")
}

// FormatVersion formats version information to the writer
func FormatVersion(w io.Writer) error {
	info := GetBuildInfo()
	_, err := fmt.Fprintf(w, "Subgen Orchestrator\n")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  Version:    %s\n", info.Version)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  Go Version: %s\n", info.GoVersion)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  Build Time: %s\n", info.BuildTime)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "  Git Commit: %s\n", info.GitCommit)
	return err
}

// printStartupBanner prints a formatted startup banner with configuration summary
func printStartupBanner(cfg *config.Config, info BuildInfo, log *logrus.Logger) {
	banner := "============================================================"
	log.Info(banner)
	log.Infof("          Subgen Orchestrator v%s", info.Version)
	log.Info(banner)
	log.Info("Configuration:")
	log.Infof("  - Whisper Model: %s", cfg.Transcription.WhisperModel)
	log.Infof("  - Device: %s", cfg.Transcription.Device)
	log.Infof("  - Worker Discovery: %s", cfg.Worker.Discovery)
	log.Infof("  - Queue Size: %d", cfg.Queue.MaxSize)
	log.Infof("  - Log Level: %s", cfg.LogLevel)

	// Skip logic summary
	skipEnabled := cfg.Skip.IfTargetSubtitlesExist || cfg.Skip.IfExternalSubtitlesExist ||
		len(cfg.Skip.SubtitleLanguages) > 0 || len(cfg.Skip.AudioLanguages) > 0
	if skipEnabled {
		log.Info("  - Skip Logic: enabled")
	} else {
		log.Info("  - Skip Logic: disabled")
	}

	// Monitoring summary
	if cfg.Monitor.Enabled {
		log.Infof("  - Monitoring: enabled (%d folders)", len(cfg.Monitor.TranscribeFolders))
	} else {
		log.Info("  - Monitoring: disabled")
	}

	// Media server integration
	integrations := []string{}
	if cfg.Plex.Enabled {
		integrations = append(integrations, "Plex")
	}
	if cfg.Jellyfin.Enabled {
		integrations = append(integrations, "Jellyfin")
	}
	if len(integrations) > 0 {
		log.Infof("  - Media Servers: %s", strings.Join(integrations, ", "))
	}

	log.Info(banner)
	log.Infof("Webhook server listening on :%d", cfg.WebhookPort)
	log.Infof("Metrics server listening on :%d", cfg.MetricsPort)
	log.Info(banner)
}

// PrintVersion prints the version information and exits
func PrintVersion() {
	_ = FormatVersion(os.Stdout)
	os.Exit(0)
}

// WorkerPoolAdapter adapts discovery.Pool to observability.WorkerPool interface
type WorkerPoolAdapter struct {
	pool *discovery.Pool
}

// GetWorkers implements observability.WorkerPool
func (w *WorkerPoolAdapter) GetWorkers() ([]observability.Worker, error) {
	_ = w.pool.Refresh(context.Background())

	// Access workers (need to add GetAll method to Pool)
	// For now, return empty slice - this will be improved
	return []observability.Worker{}, nil
}

// TaskDispatcher continuously dequeues tasks and dispatches them to workers
type TaskDispatcher struct {
	queue          *queue.Queue
	workerPool     *discovery.Pool
	grpcClient     *grpc_client.Client
	plexClient     *mediaserver.PlexClient
	jellyfinClient *mediaserver.JellyfinClient
	log            *logrus.Logger
}

// Run starts the task dispatcher loop
func (td *TaskDispatcher) Run(ctx context.Context) {
	td.log.Info("Task dispatcher running")

	for {
		select {
		case <-ctx.Done():
			td.log.Info("Task dispatcher stopping")
			return
		default:
			// Try to dequeue a task
			task, err := td.queue.Dequeue()
			if err != nil {
				// Queue empty - wait a bit before trying again
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Dispatch task to worker
			go td.dispatchTask(ctx, task)
		}
	}
}

// dispatchTask sends a single task to a worker via gRPC
func (td *TaskDispatcher) dispatchTask(ctx context.Context, task *queue.Task) {
	defer func() {
		_ = td.queue.MarkDone(task.ID)
	}()

	// Helper to send result to result channel if present
	sendResult := func(result *queue.TranscriptionResult) {
		if task.ResultChan != nil {
			defer close(task.ResultChan)
			task.ResultChan <- result
		}
	}

	td.log.WithFields(logrus.Fields{
		"task_id":   task.ID,
		"file_path": task.FilePath,
		"task_type": task.TaskType,
	}).Info("Dispatching task")

	// Fetch file path from Plex if needed
	if task.PlexItemID != "" && td.plexClient != nil {
		filePath, err := td.plexClient.GetFilePath(ctx, task.PlexItemID)
		if err != nil {
			td.log.WithError(err).WithField("plex_item_id", task.PlexItemID).Error("Failed to fetch file path from Plex")
			sendResult(&queue.TranscriptionResult{
				Error: fmt.Errorf("failed to fetch file path from Plex: %w", err),
			})
			return
		}
		task.FilePath = filePath
		td.log.WithFields(logrus.Fields{
			"plex_item_id": task.PlexItemID,
			"file_path":    filePath,
		}).Info("Fetched file path from Plex")
	}

	// Fetch file path from Jellyfin if needed
	if task.JellyfinItemID != "" && td.jellyfinClient != nil {
		filePath, err := td.jellyfinClient.GetFilePath(ctx, task.JellyfinItemID)
		if err != nil {
			td.log.WithError(err).WithField("jellyfin_item_id", task.JellyfinItemID).Error("Failed to fetch file path from Jellyfin")
			sendResult(&queue.TranscriptionResult{
				Error: fmt.Errorf("failed to fetch file path from Jellyfin: %w", err),
			})
			return
		}
		task.FilePath = filePath
		td.log.WithFields(logrus.Fields{
			"jellyfin_item_id": task.JellyfinItemID,
			"file_path":        filePath,
		}).Info("Fetched file path from Jellyfin")
	}

	// ASR tasks with AudioContent are now sent directly via gRPC
	// No need to create temp files since byte content is sent directly
	if len(task.AudioContent) > 0 && task.FilePath == "" {
		td.log.WithField("audio_bytes", len(task.AudioContent)).Debug("ASR task with audio content, will send via gRPC")
	}

	// Select a worker
	worker, err := td.workerPool.SelectWorker()
	if err != nil {
		td.log.WithError(err).Error("Failed to select worker")
		sendResult(&queue.TranscriptionResult{
			Error: fmt.Errorf("failed to select worker: %w", err),
		})
		return
	}

	// Call transcribe RPC
	resp, err := td.grpcClient.Transcribe(ctx, worker.Address, task)
	if err != nil {
		td.log.WithError(err).Error("Transcription failed")
		sendResult(&queue.TranscriptionResult{
			Error: fmt.Errorf("transcription failed: %w", err),
		})
		return
	}

	if !resp.Success {
		td.log.WithField("error", resp.ErrorMessage).Error("Transcription unsuccessful")
		sendResult(&queue.TranscriptionResult{
			Error: fmt.Errorf("transcription unsuccessful: %s", resp.ErrorMessage),
		})
		return
	}

	td.log.WithFields(logrus.Fields{
		"subtitle_path":     resp.SubtitlePath,
		"detected_language": resp.DetectedLanguage,
	}).Info("Transcription completed successfully")

	// Get segments from response or parse from file
	var segments []queue.Segment
	if task.ResultChan != nil {
		// Prefer segments from response (for ASR/bytes input)
		if len(resp.Segments) > 0 {
			// Convert protobuf segments to queue segments
			for _, pbSegment := range resp.Segments {
				segment := queue.Segment{
					Start: float64(pbSegment.Start),
					End:   float64(pbSegment.End),
					Text:  pbSegment.Text,
				}
				segments = append(segments, segment)
			}
			td.log.WithField("segment_count", len(segments)).Debug("Using segments from response")
		} else if resp.SubtitlePath != "" {
			// Fallback: Read the subtitle file (for file-based workflows with shared storage)
			parsedSegments, parseErr := td.parseSubtitleFile(resp.SubtitlePath)
			if parseErr != nil {
				td.log.WithError(parseErr).Warn("Failed to parse subtitle file for result channel")
				// Continue with empty segments rather than failing
			} else {
				segments = parsedSegments
				td.log.WithField("segment_count", len(segments)).Debug("Parsed segments from subtitle file")
			}
		}
	}

	// Send result to result channel if present
	sendResult(&queue.TranscriptionResult{
		Segments: segments,
		Metadata: queue.Metadata{
			Language: resp.DetectedLanguage,
			Duration: float64(resp.Stats.GetDurationSeconds()),
			Model:    "", // Not available in current response
		},
		Error: nil,
	})

	// Refresh metadata if needed
	if task.PlexItemID != "" && td.plexClient != nil {
		if err := td.plexClient.RefreshMetadata(ctx, task.PlexItemID); err != nil {
			td.log.WithError(err).Warn("Failed to refresh Plex metadata")
		}
	}

	if task.JellyfinItemID != "" && td.jellyfinClient != nil {
		if err := td.jellyfinClient.RefreshMetadata(ctx, task.JellyfinItemID); err != nil {
			td.log.WithError(err).Warn("Failed to refresh Jellyfin metadata")
		}
	}
}

// parseSubtitleFile reads a subtitle file and parses it into segments
func (td *TaskDispatcher) parseSubtitleFile(filePath string) ([]queue.Segment, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read subtitle file: %w", err)
	}

	// Determine format from file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	format := strings.TrimPrefix(ext, ".")

	// Parse based on format
	switch format {
	case "srt":
		return td.parseSRT(string(content))
	case "lrc":
		return td.parseLRC(string(content))
	case "vtt":
		return td.parseVTT(string(content))
	default:
		return nil, fmt.Errorf("unsupported subtitle format: %s", format)
	}
}

// parseSRT parses SRT format into segments
func (td *TaskDispatcher) parseSRT(content string) ([]queue.Segment, error) {
	var segments []queue.Segment
	lines := strings.Split(content, "\n")

	var currentSeg queue.Segment
	var inText bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and sequence numbers
		if line == "" {
			inText = false
			if currentSeg.Text != "" {
				segments = append(segments, currentSeg)
				currentSeg = queue.Segment{}
			}
			continue
		}

		// Check if this is a timestamp line
		if strings.Contains(line, "-->") {
			parts := strings.Split(line, "-->")
			if len(parts) == 2 {
				currentSeg.Start = td.parseTimestamp(strings.TrimSpace(parts[0]))
				currentSeg.End = td.parseTimestamp(strings.TrimSpace(parts[1]))
				inText = true
			}
			continue
		}

		// If we're after a timestamp, this is text
		if inText {
			if currentSeg.Text != "" {
				currentSeg.Text += " "
			}
			currentSeg.Text += line
		}
	}

	// Add final segment if exists
	if currentSeg.Text != "" {
		segments = append(segments, currentSeg)
	}

	return segments, nil
}

// parseLRC parses LRC format into segments
func (td *TaskDispatcher) parseLRC(content string) ([]queue.Segment, error) {
	var segments []queue.Segment
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "[") {
			continue
		}

		// Parse [mm:ss.xx]Text format
		endBracket := strings.Index(line, "]")
		if endBracket == -1 {
			continue
		}

		timestamp := line[1:endBracket]
		text := strings.TrimSpace(line[endBracket+1:])

		start := td.parseLRCTimestamp(timestamp)

		// Calculate end time (next segment's start, or add 3 seconds for last)
		end := start + 3.0
		if i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(nextLine, "[") {
				nextEnd := strings.Index(nextLine, "]")
				if nextEnd != -1 {
					nextTimestamp := nextLine[1:nextEnd]
					end = td.parseLRCTimestamp(nextTimestamp)
				}
			}
		}

		segments = append(segments, queue.Segment{
			Start: start,
			End:   end,
			Text:  text,
		})
	}

	return segments, nil
}

// parseVTT parses WebVTT format into segments
func (td *TaskDispatcher) parseVTT(content string) ([]queue.Segment, error) {
	// VTT is similar to SRT but with WEBVTT header
	content = strings.TrimPrefix(content, "WEBVTT")
	content = strings.TrimSpace(content)
	return td.parseSRT(content) // Reuse SRT parser
}

// parseTimestamp converts SRT timestamp (00:00:10,500) to seconds
func (td *TaskDispatcher) parseTimestamp(ts string) float64 {
	// Format: HH:MM:SS,mmm or HH:MM:SS.mmm
	ts = strings.Replace(ts, ",", ".", 1)

	parts := strings.Split(ts, ":")
	if len(parts) != 3 {
		return 0
	}

	hours := parseFloat(parts[0])
	minutes := parseFloat(parts[1])
	seconds := parseFloat(parts[2])

	return hours*3600 + minutes*60 + seconds
}

// parseLRCTimestamp converts LRC timestamp (mm:ss.xx) to seconds
func (td *TaskDispatcher) parseLRCTimestamp(ts string) float64 {
	parts := strings.Split(ts, ":")
	if len(parts) != 2 {
		return 0
	}

	minutes := parseFloat(parts[0])
	seconds := parseFloat(parts[1])

	return minutes*60 + seconds
}

// parseFloat is a helper to parse float from string
func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}

// CheckHealth performs a health check on the orchestrator
// Currently checks if the HTTP port (9000) is available/listening
func CheckHealth() error {
	// FUTURE: Add more comprehensive health checks (database, queue, etc.)

	// Check if we can connect to the HTTP port
	// For now, just verify the service can bind to the port or is already listening
	conn, err := net.DialTimeout("tcp", "localhost:9000", 2*time.Second)
	if err != nil {
		// If we can't connect, the service might not be up yet
		return fmt.Errorf("HTTP server not responding on port 9000: %w", err)
	}
	conn.Close()
	return nil
}

// QueueAdapter adapts queue.Queue to monitor.QueueInterface
type QueueAdapter struct {
	queue *queue.Queue
}

// Enqueue implements monitor.QueueInterface
func (qa *QueueAdapter) Enqueue(task interface{}) error {
	// Expect task to be a map[string]interface{} with file_path
	taskMap, ok := task.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid task type: expected map[string]interface{}")
	}

	filePath, ok := taskMap["file_path"].(string)
	if !ok {
		return fmt.Errorf("invalid task: missing file_path")
	}

	// Create proper Task object
	queueTask := queue.NewTask(filePath, queue.TaskTypeTranscribe)

	return qa.queue.Enqueue(queueTask)
}

// WebhookWorkerPoolAdapter adapts discovery.Pool to webhooks.WorkerPoolInterface
type WebhookWorkerPoolAdapter struct {
	pool *discovery.Pool
}

// SelectWorker implements webhooks.WorkerPoolInterface
func (a *WebhookWorkerPoolAdapter) SelectWorker() (*webhooks.Worker, error) {
	worker, err := a.pool.SelectWorker()
	if err != nil {
		return nil, err
	}

	// Convert discovery.Worker to webhooks.Worker
	return &webhooks.Worker{
		Address: worker.Address,
		Healthy: worker.Healthy,
	}, nil
}
