package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/discovery"
	"github.com/mccloud/subgen/orchestrator/internal/grpc_client"
	"github.com/mccloud/subgen/orchestrator/internal/mediaserver"
	"github.com/mccloud/subgen/orchestrator/internal/observability"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
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

	// Log startup
	buildInfo := GetBuildInfo()
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

	// Register observability middleware
	app := webhookServer.App()
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
	w.pool.Refresh(context.Background())

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
	defer td.queue.MarkDone(task.ID)

	td.log.WithFields(logrus.Fields{
		"task_id":   task.ID,
		"file_path": task.FilePath,
		"task_type": task.TaskType,
	}).Info("Dispatching task")

	// Select a worker
	worker, err := td.workerPool.SelectWorker()
	if err != nil {
		td.log.WithError(err).Error("Failed to select worker")
		return
	}

	// Call transcribe RPC
	resp, err := td.grpcClient.Transcribe(ctx, worker.Address, task)
	if err != nil {
		td.log.WithError(err).Error("Transcription failed")
		return
	}

	if !resp.Success {
		td.log.WithField("error", resp.ErrorMessage).Error("Transcription unsuccessful")
		return
	}

	td.log.WithFields(logrus.Fields{
		"subtitle_path":     resp.SubtitlePath,
		"detected_language": resp.DetectedLanguage,
	}).Info("Transcription completed successfully")

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
