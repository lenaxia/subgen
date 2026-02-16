# STORY_07: gRPC Client for Worker Communication

**Status:** Not Started  
**Effort:** 6-8 hours  
**Epic:** EPIC_01 (Go Orchestrator Core)  
**Created:** 2026-02-15

---

## User Story

**As a** developer  
**I want** a gRPC client to communicate with Python workers  
**So that** the orchestrator can send transcription tasks and receive results

---

## Acceptance Criteria

- [ ] gRPC client implements all 3 RPC methods from proto
- [ ] Transcribe RPC with streaming support for large files
- [ ] DetectLanguage RPC for language detection
- [ ] HealthCheck RPC for worker monitoring
- [ ] Connection pooling with configurable pool size
- [ ] Retry logic with exponential backoff (3 retries max)
- [ ] Timeout handling (configurable, default 5hr for transcribe)
- [ ] Error wrapping with context
- [ ] 10+ test cases with mock gRPC server
- [ ] Integration with worker pool from STORY_06
- [ ] Prometheus metrics for RPC calls (duration, errors)
- [ ] Work log created

---

## Integration Points

### Proto Definition (transcription.proto:1-181)

**Location:** `/home/mikekao/personal/subgen/api/transcription.proto:1-181`

**Service Definition:**
```protobuf
service TranscriptionService {
  // Transcribe audio to subtitles (SRT or LRC)
  rpc Transcribe(TranscribeRequest) returns (TranscribeResponse);
  
  // Detect language from audio sample
  rpc DetectLanguage(DetectLanguageRequest) returns (DetectLanguageResponse);
  
  // Health check for orchestrator monitoring
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}
```

**Key Message Structures:**

**TranscribeRequest (lines 26-41):**
```protobuf
message TranscribeRequest {
  string file_path = 1;                       // Shared NFS path
  string task_type = 2;                       // "transcribe" or "translate"
  string force_language = 3;                  // ISO 639-1 code or empty
  TranscribeOptions options = 4;              // Model, threads, etc.
  map<string, string> metadata = 5;           // plex_item_id, etc.
}
```

**TranscribeOptions (lines 43-73):**
```protobuf
message TranscribeOptions {
  string whisper_model = 1;              // tiny, base, small, medium, large, large-v3
  int32 whisper_threads = 2;             // CPU threads
  bool word_level_highlight = 3;         // Karaoke style
  string custom_regroup = 4;             // stable-ts algorithm
  bool lrc_for_audio = 5;                // LRC vs SRT for audio files
  string custom_prompt = 6;              // Model prompt
  bool append_footer = 7;                // "Transcribed by..."
  string subtitle_language_name = 8;     // aa, en, etc.
  bool show_model_in_filename = 9;       // Include model in filename
  bool show_subgen_in_filename = 10;     // Include "subgen" in filename
}
```

**TranscribeResponse (lines 75-90):**
```protobuf
message TranscribeResponse {
  bool success = 1;
  string subtitle_path = 2;              // Where subtitle was written
  string detected_language = 3;          // ISO 639-1 code
  string error_message = 4;
  TranscriptionStats stats = 5;          // Performance metrics
}
```

**HealthCheckResponse (lines 152-180):**
```protobuf
message HealthCheckResponse {
  enum Status {
    UNKNOWN = 0;
    HEALTHY = 1;
    UNHEALTHY = 2;
    STARTING = 3;
  }
  
  Status status = 1;
  int64 memory_mb = 2;
  bool model_loaded = 3;
  int32 jobs_processed = 4;
  int32 jobs_active = 5;                 // Currently processing
  string version = 6;
  int64 uptime_seconds = 7;
}
```

---

## Technical Design

### File Structure

```
internal/grpcclient/
├── client.go           # Main gRPC client
├── client_test.go      # Unit tests
├── pool.go             # Connection pool
├── retry.go            # Retry logic with backoff
└── metrics.go          # Prometheus metrics
```

---

### Main Client (client.go)

**File:** `internal/grpcclient/client.go`

```go
package grpcclient

import (
	"context"
	"fmt"
	"time"
	
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	
	pb "github.com/your-org/subgen/orchestrator/pkg/api/v1"
	"github.com/your-org/subgen/orchestrator/internal/config"
	"github.com/your-org/subgen/orchestrator/internal/queue"
)

// TranscriptionClient wraps gRPC client with retry and metrics
type TranscriptionClient struct {
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

// NewTranscriptionClient creates a new client
func NewTranscriptionClient(cfg *config.TranscriptionConfig, metrics *ClientMetrics, log *logrus.Logger) *TranscriptionClient {
	return &TranscriptionClient{
		pool:              NewConnectionPool(10), // Max 10 connections
		metrics:           metrics,
		log:               log,
		transcribeTimeout: 5 * time.Hour,         // Long timeout for transcription
		healthTimeout:     5 * time.Second,
		maxRetries:        3,
		retryDelay:        1 * time.Second,
	}
}

// Transcribe sends a transcription task to a worker
func (c *TranscriptionClient) Transcribe(ctx context.Context, workerAddr string, task *queue.Task) (*pb.TranscribeResponse, error) {
	start := time.Now()
	
	// Get connection from pool
	conn, err := c.pool.Get(ctx, workerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(workerAddr, conn)
	
	client := pb.NewTranscriptionServiceClient(conn)
	
	// Build request from task
	req := &pb.TranscribeRequest{
		FilePath:      task.FilePath,
		TaskType:      task.TaskType,
		ForceLanguage: task.ForceLanguage,
		Options: &pb.TranscribeOptions{
			WhisperModel:           c.getWhisperModel(task),
			WhisperThreads:         c.getWhisperThreads(task),
			WordLevelHighlight:     task.WordLevelHighlight,
			CustomRegroup:          task.CustomRegroup,
			LrcForAudio:            task.LRCForAudioFiles,
			SubtitleLanguageName:   task.SubtitleLanguageName,
			AppendFooter:           task.AppendFooter,
		},
		Metadata: c.buildMetadata(task),
	}
	
	// Add timeout to context
	ctx, cancel := context.WithTimeout(ctx, c.transcribeTimeout)
	defer cancel()
	
	c.log.WithFields(logrus.Fields{
		"worker_addr": workerAddr,
		"file_path":   task.FilePath,
		"task_type":   task.TaskType,
	}).Info("Sending transcription request")
	
	// Call with retry
	var resp *pb.TranscribeResponse
	err = c.retryWithBackoff(ctx, func() error {
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
		"worker_addr":     workerAddr,
		"subtitle_path":   resp.SubtitlePath,
		"detected_lang":   resp.DetectedLanguage,
		"duration_sec":    duration,
		"segments":        resp.Stats.SegmentCount,
	}).Info("Transcription completed")
	
	return resp, nil
}

// DetectLanguage detects language from audio file
func (c *TranscriptionClient) DetectLanguage(ctx context.Context, workerAddr string, filePath string) (*pb.DetectLanguageResponse, error) {
	conn, err := c.pool.Get(ctx, workerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(workerAddr, conn)
	
	client := pb.NewTranscriptionServiceClient(conn)
	
	req := &pb.DetectLanguageRequest{
		AudioSource: &pb.DetectLanguageRequest_FilePath{
			FilePath: filePath,
		},
		SampleLength: 30, // 30 seconds sample
		SampleOffset: 0,
	}
	
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	c.log.WithFields(logrus.Fields{
		"worker_addr": workerAddr,
		"file_path":   filePath,
	}).Debug("Detecting language")
	
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

// HealthCheck checks if a worker is healthy
func (c *TranscriptionClient) HealthCheck(ctx context.Context, workerAddr string) (*pb.HealthCheckResponse, error) {
	conn, err := c.pool.Get(ctx, workerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.Put(workerAddr, conn)
	
	client := pb.NewTranscriptionServiceClient(conn)
	
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
func (c *TranscriptionClient) retryWithBackoff(ctx context.Context, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
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
		
		lastErr = err
		c.log.WithError(err).WithField("attempt", attempt).Error("RPC call failed")
	}
	
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// Helper functions
func (c *TranscriptionClient) getWhisperModel(task *queue.Task) string {
	if task.WhisperModel != "" {
		return task.WhisperModel
	}
	return "medium" // Default
}

func (c *TranscriptionClient) getWhisperThreads(task *queue.Task) int32 {
	if task.WhisperThreads > 0 {
		return int32(task.WhisperThreads)
	}
	return 4 // Default
}

func (c *TranscriptionClient) buildMetadata(task *queue.Task) map[string]string {
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
```

---

### Connection Pool (pool.go)

**File:** `internal/grpcclient/pool.go`

```go
package grpcclient

import (
	"context"
	"fmt"
	"sync"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ConnectionPool manages gRPC connections to workers
type ConnectionPool struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn
	
	maxConns int
}

// NewConnectionPool creates a connection pool
func NewConnectionPool(maxConns int) *ConnectionPool {
	return &ConnectionPool{
		conns:    make(map[string]*grpc.ClientConn),
		maxConns: maxConns,
	}
}

// Get retrieves or creates a connection to worker
func (p *ConnectionPool) Get(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	// Try to get existing connection
	p.mu.RLock()
	conn, exists := p.conns[addr]
	p.mu.RUnlock()
	
	if exists && conn.GetState() != grpc.Shutdown {
		return conn, nil
	}
	
	// Create new connection
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// Double-check after acquiring write lock
	conn, exists = p.conns[addr]
	if exists && conn.GetState() != grpc.Shutdown {
		return conn, nil
	}
	
	// Dial new connection
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial worker: %w", err)
	}
	
	p.conns[addr] = conn
	
	return conn, nil
}

// Put returns a connection to the pool (no-op, keeps conn alive)
func (p *ConnectionPool) Put(addr string, conn *grpc.ClientConn) {
	// Connection remains in pool for reuse
}

// Close closes all connections
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	for addr, conn := range p.conns {
		if err := conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection to %s: %w", addr, err)
		}
		delete(p.conns, addr)
	}
	
	return nil
}
```

---

### Metrics (metrics.go)

**File:** `internal/grpcclient/metrics.go`

```go
package grpcclient

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ClientMetrics holds Prometheus metrics for gRPC client
type ClientMetrics struct {
	RPCCalls    *prometheus.CounterVec
	RPCErrors   *prometheus.CounterVec
	RPCDuration *prometheus.HistogramVec
}

// NewClientMetrics creates Prometheus metrics
func NewClientMetrics() *ClientMetrics {
	return &ClientMetrics{
		RPCCalls: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_grpc_calls_total",
				Help: "Total number of gRPC calls",
			},
			[]string{"method"},
		),
		
		RPCErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_grpc_errors_total",
				Help: "Total number of gRPC errors",
			},
			[]string{"method"},
		),
		
		RPCDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "subgen_grpc_duration_seconds",
				Help:    "gRPC call duration",
				Buckets: []float64{1, 10, 30, 60, 300, 600, 1800, 3600}, // 1s to 1hr
			},
			[]string{"method", "status"},
		),
	}
}
```

---

## Test Cases (10+)

**File:** `internal/grpcclient/client_test.go`

1. Transcribe RPC success
2. Transcribe RPC with retry on transient error
3. Transcribe RPC timeout
4. DetectLanguage RPC success
5. HealthCheck RPC success
6. Connection pool reuses connections
7. Connection pool creates new connection on dial failure
8. Retry logic with exponential backoff
9. Metadata building (Plex/Jellyfin)
10. Metrics updated correctly
11. Context cancellation
12. Max retries exceeded error

---

## Implementation Steps

### Step 1: Generate Proto Code (30 min)
```bash
cd /home/mikekao/personal/subgen
cd api
./generate.sh  # Generates Go code from proto
```

### Step 2: Implement Client (2 hours)
Create client.go with Transcribe, DetectLanguage, HealthCheck methods

### Step 3: Implement Connection Pool (1 hour)
Create pool.go with connection caching and reuse

### Step 4: Implement Retry Logic (1 hour)
Add exponential backoff retry for transient failures

### Step 5: Add Metrics (30 min)
Instrument all RPC calls with Prometheus metrics

### Step 6: Write Tests (2 hours)
Mock gRPC server, test all scenarios

### Step 7: Integration with Worker Pool (1 hour)
Create orchestrator loop:
```go
for {
    task, _ := queue.Dequeue()
    worker, _ := workerPool.SelectWorker()
    resp, _ := grpcClient.Transcribe(ctx, worker.Address, task)
    queue.MarkDone(task.ID)
    
    // Refresh media server metadata
    if task.PlexItemID != "" {
        plexClient.RefreshMetadata(ctx, task.PlexItemID)
    }
}
```

---

## Dependencies

**Requires:**
- STORY_01 (Project Setup) ✅
- STORY_04 (Queue) ✅
- STORY_06 (Worker Pool) ✅

**Blocks:**
- None (last component for core orchestrator)

---

## Definition of Done

- [ ] All 10+ tests passing
- [ ] Transcribe RPC works with real worker
- [ ] DetectLanguage RPC works
- [ ] HealthCheck RPC works
- [ ] Connection pooling verified
- [ ] Retry logic with backoff
- [ ] Timeout handling
- [ ] Prometheus metrics
- [ ] Integration with worker pool
- [ ] Manual end-to-end test (webhook → worker → subtitle)
- [ ] Code passes golangci-lint
- [ ] Work log created
- [ ] Coverage > 80%

---

## Notes

### Key Design Decisions

1. **Why connection pooling?**
   - Reusing connections reduces latency (no handshake overhead)
   - Important for repeated health checks

2. **Why 5 hour timeout for transcribe?**
   - Large video files can take hours to transcribe
   - Legacy Python has similar timeout
   - Prevent orphaned goroutines

3. **Why exponential backoff?**
   - Transient network errors common
   - Avoid overwhelming worker with retries
   - Standard practice for distributed systems

### References

- gRPC Go docs: https://grpc.io/docs/languages/go/
- Connection pooling: https://github.com/grpc/grpc-go/blob/master/examples/features/connection_pool/client/main.go
- Proto definition: `/home/mikekao/personal/subgen/api/transcription.proto`

---

**Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
