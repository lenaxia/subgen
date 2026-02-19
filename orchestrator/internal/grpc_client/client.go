package grpc_client

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mccloud/subgen/orchestrator/internal/queue"
	pb "github.com/mccloud/subgen/orchestrator/pkg/pb"
)

// Client wraps gRPC client with retry and metrics
type Client struct {
	pool    *ConnectionPool
	metrics *ClientMetrics
	log     *logrus.Logger

	// Timeouts
	transcribeTimeout time.Duration
	healthTimeout     time.Duration

	// Retry config
	maxRetries int
	retryDelay time.Duration
}

// NewClient creates a new gRPC client
func NewClient(
	transcribeTimeout time.Duration,
	healthTimeout time.Duration,
	maxRetries int,
	retryDelay time.Duration,
	metrics *ClientMetrics,
	log *logrus.Logger,
) *Client {
	return &Client{
		pool:              NewConnectionPool(10), // Max 10 connections per worker
		metrics:           metrics,
		log:               log,
		transcribeTimeout: transcribeTimeout,
		healthTimeout:     healthTimeout,
		maxRetries:        maxRetries,
		retryDelay:        retryDelay,
	}
}

// Transcribe sends a transcription task to a worker
func (c *Client) Transcribe(ctx context.Context, workerAddr string, task *queue.Task) (*pb.TranscribeResponse, error) {
	conn, err := c.pool.Get(ctx, workerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(workerAddr, conn)

	client := pb.NewTranscriptionServiceClient(conn)
	return c.transcribeWithClient(ctx, client, task)
}

// DetectLanguage detects language from audio file with customizable sample parameters
func (c *Client) DetectLanguage(ctx context.Context, workerAddr string, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error) {
	conn, err := c.pool.Get(ctx, workerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(workerAddr, conn)

	client := pb.NewTranscriptionServiceClient(conn)
	return c.detectLanguageWithClient(ctx, client, filePath, offset, length)
}

// HealthCheck checks if a worker is healthy
func (c *Client) HealthCheck(ctx context.Context, workerAddr string) (*pb.HealthCheckResponse, error) {
	conn, err := c.pool.Get(ctx, workerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(workerAddr, conn)

	client := pb.NewTranscriptionServiceClient(conn)
	return c.healthCheckWithClient(ctx, client)
}

// Close closes all connections in the pool
func (c *Client) Close() error {
	return c.pool.Close()
}

// transcribeWithClient sends a transcription task to a worker (internal method for testing)
func (c *Client) transcribeWithClient(ctx context.Context, client pb.TranscriptionServiceClient, task *queue.Task) (*pb.TranscribeResponse, error) {
	start := time.Now()

	// Build request from task
	req := &pb.TranscribeRequest{
		TaskType:      task.TaskType,
		ForceLanguage: task.ForceLanguage,
		Options: &pb.TranscribeOptions{
			WhisperModel:   c.getWhisperModel(task),
			WhisperThreads: c.getWhisperThreads(task),
		},
		Metadata: c.buildMetadata(task),
	}

	// Set audio source: either file_path or audio_content
	if len(task.AudioContent) > 0 {
		// ASR task with audio content
		req.AudioSource = &pb.TranscribeRequest_AudioContent{
			AudioContent: task.AudioContent,
		}
		c.log.WithField("audio_bytes", len(task.AudioContent)).Debug("Sending audio content in gRPC request")
	} else if task.FilePath != "" {
		// Media server task with file path
		req.AudioSource = &pb.TranscribeRequest_FilePath{
			FilePath: task.FilePath,
		}
		c.log.WithField("file_path", task.FilePath).Debug("Sending file path in gRPC request")
	} else {
		return nil, fmt.Errorf("task has neither file path nor audio content")
	}

	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, c.transcribeTimeout)
	defer cancel()

	c.log.WithFields(logrus.Fields{
		"file_path": task.FilePath,
		"task_type": task.TaskType,
	}).Info("Sending transcription request")

	// Call with retry
	var resp *pb.TranscribeResponse
	err := c.retryWithBackoff(ctx, func() error {
		var callErr error
		resp, callErr = client.Transcribe(ctx, req)
		return callErr
	})

	// Update metrics
	duration := time.Since(start).Seconds()
	if err != nil {
		c.metrics.RPCDuration.WithLabelValues("Transcribe", "error").Observe(duration)
		c.metrics.RPCErrors.WithLabelValues("Transcribe").Inc()
		return nil, fmt.Errorf("transcribe RPC failed after retries: %w", err)
	}

	c.metrics.RPCDuration.WithLabelValues("Transcribe", "success").Observe(duration)
	c.metrics.RPCCalls.WithLabelValues("Transcribe").Inc()

	if !resp.Success {
		return resp, fmt.Errorf("transcription failed: %s", resp.ErrorMessage)
	}

	c.log.WithFields(logrus.Fields{
		"subtitle_path": resp.SubtitlePath,
		"detected_lang": resp.DetectedLanguage,
		"duration_sec":  duration,
	}).Info("Transcription completed")

	return resp, nil
}

// detectLanguageWithClient detects language from audio file (internal method for testing)
func (c *Client) detectLanguageWithClient(ctx context.Context, client pb.TranscriptionServiceClient, filePath string, offset float64, length float64) (*pb.DetectLanguageResponse, error) {
	req := &pb.DetectLanguageRequest{
		AudioSource: &pb.DetectLanguageRequest_FilePath{
			FilePath: filePath,
		},
		SampleLength: int32(length), // Convert float64 to int32
		SampleOffset: int32(offset), // Convert float64 to int32
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	c.log.WithField("file_path", filePath).Debug("Detecting language")

	resp, err := client.DetectLanguage(ctx, req)
	if err != nil {
		c.metrics.RPCErrors.WithLabelValues("DetectLanguage").Inc()
		return nil, fmt.Errorf("detect language RPC failed: %w", err)
	}

	c.metrics.RPCCalls.WithLabelValues("DetectLanguage").Inc()

	c.log.WithFields(logrus.Fields{
		"language":   resp.LanguageName,
		"confidence": resp.Confidence,
	}).Info("Language detected")

	return resp, nil
}

// healthCheckWithClient checks if a worker is healthy (internal method for testing)
func (c *Client) healthCheckWithClient(ctx context.Context, client pb.TranscriptionServiceClient) (*pb.HealthCheckResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.healthTimeout)
	defer cancel()

	resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		c.metrics.RPCErrors.WithLabelValues("HealthCheck").Inc()
		return nil, err
	}

	c.metrics.RPCCalls.WithLabelValues("HealthCheck").Inc()

	return resp, nil
}

// retryWithBackoff executes fn with exponential backoff
func (c *Client) retryWithBackoff(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, 8s, ...
			delay := c.retryDelay * time.Duration(1<<uint(attempt-1))

			c.log.WithFields(logrus.Fields{
				"attempt": attempt,
				"delay":   delay,
			}).Warn("Retrying after error")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryable(err) {
			return err
		}

		lastErr = err
		c.log.WithError(err).WithField("attempt", attempt).Error("RPC call failed")
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryable checks if an error should trigger a retry
func isRetryable(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		// Non-gRPC errors are not retryable
		return false
	}

	// Retry on transient errors
	code := st.Code()
	return code == codes.Unavailable ||
		code == codes.DeadlineExceeded ||
		code == codes.ResourceExhausted ||
		code == codes.Aborted
}

// Helper functions

func (c *Client) getWhisperModel(task *queue.Task) string {
	// TODO: Get from task or config
	return "medium" // Default
}

func (c *Client) getWhisperThreads(task *queue.Task) int32 {
	// TODO: Get from task or config
	return 4 // Default
}

func (c *Client) buildMetadata(task *queue.Task) map[string]string {
	metadata := make(map[string]string)

	if task.PlexItemID != "" {
		metadata["plex_item_id"] = task.PlexItemID
		metadata["plex_server"] = task.PlexServer
		metadata["plex_token"] = task.PlexToken
	}

	if task.JellyfinItemID != "" {
		metadata["jellyfin_item_id"] = task.JellyfinItemID
		metadata["jellyfin_server"] = task.JellyfinServer
		metadata["jellyfin_token"] = task.JellyfinToken
	}

	return metadata
}
