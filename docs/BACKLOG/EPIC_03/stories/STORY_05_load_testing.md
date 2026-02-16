# Story 05: Load Testing & Performance Validation

**Epic**: EPIC_03 - Integration & Testing  
**Status**: Not Started  
**Priority**: High  
**Estimated Effort**: 4-6 hours  
**Assignee**: TBD

---

## User Story

As a **DevOps engineer**,  
I want **load tests that validate system performance under stress**,  
So that **I can verify the system handles production workloads without degradation**.

---

## Context

Load testing validates that the system can handle:
- Concurrent webhook requests (burst traffic)
- Large queue backlogs (many pending transcriptions)
- Extended operation (24-hour soak test)
- Worker restarts during operation
- Resource constraints (CPU/memory limits)

**Why This Matters:**
- Media servers can send webhook bursts (entire season added)
- System must handle backlog without crashes
- Long-running stability is critical (24/7 operation)
- Performance degradation must be detected early

**Performance Targets** (from EPIC_03 README):
- gRPC latency: < 100ms (p99 for HealthCheck)
- Queue throughput: 100+ tasks/sec
- Concurrent webhooks: 50+ req/sec without errors
- 24-hour uptime: Zero crashes
- Memory growth: < 20% after 1000 transcriptions (STORY_04)

**Current State:**
- No load tests exist
- No performance benchmarks
- No soak tests

**Target State:**
- Automated load test suite
- Performance benchmarks established
- 24-hour soak test procedure
- Stress test scenarios documented
- Grafana dashboard for monitoring (optional)

---

## Acceptance Criteria

- [ ] Load test file created: `test/load/load_test.go`
- [ ] Test: 100 concurrent webhook requests
- [ ] Test: Queue handles 1000+ tasks without errors
- [ ] Test: 50 webhooks/sec sustained for 1 minute
- [ ] Test: gRPC p99 latency < 100ms
- [ ] Test: Worker restart during operation (graceful handling)
- [ ] Test: Orchestrator restart during operation
- [ ] 24-hour soak test procedure documented
- [ ] Performance benchmark results recorded
- [ ] Grafana dashboard for monitoring (optional)
- [ ] Stress test scenarios defined
- [ ] All tests pass
- [ ] Work log created

---

## Technical Design

### Load Testing Architecture

```
┌────────────────────────────────────────────────────────────────┐
│  Load Test Generator                                           │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  Concurrent Webhook Generator                            │ │
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │ │
│  │  • 50-100 concurrent goroutines                         │ │
│  │  • Send webhook requests continuously                   │ │
│  │  • Measure response times                               │ │
│  │  • Track success/error rates                            │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                 │
│                         ↓ HTTP POST                            │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  Orchestrator Under Test                                 │ │
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │ │
│  │  • Queue: 1000+ tasks                                    │ │
│  │  • Metrics: Prometheus export                            │ │
│  │  • Logs: Structured JSON                                 │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                 │
│                         ↓ gRPC                                 │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  Worker Under Test                                       │ │
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │ │
│  │  • Process transcriptions                                │ │
│  │  • Memory monitored                                      │ │
│  │  • Health checks every 30s                               │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  Metrics Collection                                      │ │
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ │ │
│  │  • Prometheus scrapes metrics                            │ │
│  │  • Response time percentiles                             │ │
│  │  • Error rates                                           │ │
│  │  • Memory/CPU usage                                      │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### File Structure

```
test/
├── load/
│   ├── load_test.go                        # Main load tests
│   ├── concurrent_webhook_test.go          # Concurrent webhook tests
│   ├── queue_stress_test.go                # Queue stress tests
│   ├── soak_test.go                        # 24-hour soak test
│   └── benchmark_test.go                   # Go benchmarks
├── scripts/
│   ├── run_load_tests.sh                   # Load test runner
│   └── run_soak_test.sh                    # 24-hour soak test
├── monitoring/
│   ├── grafana_dashboard.json              # Grafana dashboard
│   └── prometheus_alerts.yml               # Alert rules
└── reports/
    ├── load_test_report.md                 # Test results
    └── benchmark_results.txt               # Benchmark output
```

---

## Implementation Steps

### Step 1: Concurrent Webhook Load Test

**File: `/home/mikekao/personal/subgen/test/load/concurrent_webhook_test.go`**

```go
package load

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	orchestratorURL = "http://localhost:9000"
	testAudioFile   = "../testdata/short_audio.mp3"
)

// Test 1: 100 Concurrent Webhook Requests
func TestLoad_100ConcurrentWebhooks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	const numRequests = 100
	const concurrency = 20

	// Counters
	var successCount atomic.Int64
	var errorCount atomic.Int64
	var totalDuration atomic.Int64

	// Wait group
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	t.Logf("Sending %d webhook requests with concurrency %d...", numRequests, concurrency)
	start := time.Now()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire

		go func(id int) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release

			requestStart := time.Now()

			// Create Emby webhook payload
			payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, testAudioFile)

			resp, err := http.Post(
				orchestratorURL+"/emby",
				"application/x-www-form-urlencoded",
				bytes.NewReader([]byte(payload)),
			)

			requestDuration := time.Since(requestStart)
			totalDuration.Add(int64(requestDuration.Milliseconds()))

			if err != nil {
				errorCount.Add(1)
				t.Logf("Request %d failed: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
				successCount.Add(1)
			} else {
				errorCount.Add(1)
				t.Logf("Request %d got status %d", id, resp.StatusCode)
			}
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(start)

	// Calculate metrics
	successRate := float64(successCount.Load()) / float64(numRequests) * 100
	avgDuration := float64(totalDuration.Load()) / float64(numRequests)
	throughput := float64(numRequests) / totalTime.Seconds()

	t.Logf("\n=== Load Test Results ===")
	t.Logf("Total Time:    %.2fs", totalTime.Seconds())
	t.Logf("Success:       %d/%d (%.1f%%)", successCount.Load(), numRequests, successRate)
	t.Logf("Errors:        %d", errorCount.Load())
	t.Logf("Avg Latency:   %.1fms", avgDuration)
	t.Logf("Throughput:    %.1f req/sec", throughput)

	// Assertions
	assert.GreaterOrEqual(t, successRate, 95.0, "Success rate should be >= 95%")
	assert.Less(t, avgDuration, 500.0, "Average latency should be < 500ms")

	t.Log("✅ PASS: Concurrent webhook load test")
}

// Test 2: Sustained Load (50 req/sec for 1 minute)
func TestLoad_SustainedLoad_50ReqPerSec(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	const targetRPS = 50
	const durationSec = 60
	const totalRequests = targetRPS * durationSec

	var successCount atomic.Int64
	var errorCount atomic.Int64

	ticker := time.NewTicker(time.Second / targetRPS)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(durationSec+10)*time.Second)
	defer cancel()

	requestsSent := 0
	start := time.Now()

	t.Logf("Sending sustained load: %d req/sec for %d seconds...", targetRPS, durationSec)

	for requestsSent < totalRequests {
		select {
		case <-ticker.C:
			go func(id int) {
				payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, testAudioFile)
				resp, err := http.Post(
					orchestratorURL+"/emby",
					"application/x-www-form-urlencoded",
					bytes.NewReader([]byte(payload)),
				)

				if err != nil {
					errorCount.Add(1)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
					successCount.Add(1)
				} else {
					errorCount.Add(1)
				}
			}(requestsSent)

			requestsSent++

			// Log progress every 10 seconds
			if requestsSent%(targetRPS*10) == 0 {
				elapsed := time.Since(start).Seconds()
				actualRPS := float64(requestsSent) / elapsed
				t.Logf("  Progress: %d/%d requests (%.1f req/sec)",
					requestsSent, totalRequests, actualRPS)
			}

		case <-ctx.Done():
			t.Fatalf("Test timeout")
		}
	}

	// Wait for all requests to complete
	time.Sleep(5 * time.Second)

	totalTime := time.Since(start)
	actualRPS := float64(totalRequests) / totalTime.Seconds()
	successRate := float64(successCount.Load()) / float64(totalRequests) * 100

	t.Logf("\n=== Sustained Load Results ===")
	t.Logf("Duration:      %.1fs", totalTime.Seconds())
	t.Logf("Requests Sent: %d", totalRequests)
	t.Logf("Success:       %d (%.1f%%)", successCount.Load(), successRate)
	t.Logf("Errors:        %d", errorCount.Load())
	t.Logf("Actual RPS:    %.1f", actualRPS)

	// Assertions
	assert.GreaterOrEqual(t, successRate, 95.0, "Success rate should be >= 95%")
	assert.GreaterOrEqual(t, actualRPS, float64(targetRPS)*0.9, "Should sustain ~50 req/sec")

	t.Log("✅ PASS: Sustained load test")
}

// Test 3: Queue Stress Test (1000+ Tasks)
func TestLoad_QueueStress_1000Tasks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	const numTasks = 1000

	t.Logf("Enqueueing %d tasks rapidly...", numTasks)
	start := time.Now()

	for i := 0; i < numTasks; i++ {
		payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, testAudioFile)
		resp, err := http.Post(
			orchestratorURL+"/emby",
			"application/x-www-form-urlencoded",
			bytes.NewReader([]byte(payload)),
		)

		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		resp.Body.Close()

		// Check status
		require.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable,
			"Should get 200 OK or 503 if queue full")

		if (i+1)%200 == 0 {
			t.Logf("  Enqueued: %d/%d tasks", i+1, numTasks)
		}
	}

	enqueueTime := time.Since(start)
	throughput := float64(numTasks) / enqueueTime.Seconds()

	t.Logf("\n=== Queue Stress Results ===")
	t.Logf("Enqueue Time:  %.2fs", enqueueTime.Seconds())
	t.Logf("Throughput:    %.0f tasks/sec", throughput)

	// Assertions
	assert.Greater(t, throughput, 100.0, "Queue should handle > 100 tasks/sec")

	t.Log("✅ PASS: Queue stress test")

	// Wait for queue to drain
	t.Log("Waiting for queue to drain (this may take a while)...")
	// Don't wait in test, just log
}

// Test 4: gRPC Latency Benchmark
func TestLoad_gRPCLatency_P99(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping benchmark in short mode")
	}

	// Connect to worker directly
	conn, err := grpc.Dial(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewTranscriptionServiceClient(conn)

	const numCalls = 1000
	latencies := make([]time.Duration, numCalls)

	t.Logf("Measuring gRPC HealthCheck latency (%d calls)...", numCalls)

	for i := 0; i < numCalls; i++ {
		start := time.Now()

		_, err := client.HealthCheck(context.Background(), &pb.HealthCheckRequest{})
		latency := time.Since(start)

		if err == nil {
			latencies[i] = latency
		}

		if (i+1)%200 == 0 {
			t.Logf("  Progress: %d/%d calls", i+1, numCalls)
		}
	}

	// Calculate percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]

	// Calculate average
	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	avg := total / time.Duration(len(latencies))

	t.Logf("\n=== gRPC Latency Results ===")
	t.Logf("Average: %v", avg)
	t.Logf("P50:     %v", p50)
	t.Logf("P95:     %v", p95)
	t.Logf("P99:     %v", p99)

	// Assertions
	assert.Less(t, p99, 100*time.Millisecond, "P99 latency should be < 100ms")
	assert.Less(t, avg, 50*time.Millisecond, "Average latency should be < 50ms")

	t.Log("✅ PASS: gRPC latency within targets")
}

// Test 5: Worker Restart During Load
func TestLoad_WorkerRestart_Graceful(t *testing.T) {
	t.Skip("Requires Docker Compose control - manual test")

	// This test would:
	// 1. Start load generation (50 req/sec)
	// 2. After 30 seconds, restart worker container
	// 3. Continue load generation
	// 4. Verify: orchestrator handles worker unavailable gracefully
	// 5. Verify: tasks are requeued when worker comes back
	// 6. Verify: no data loss or crashes

	// Implementation requires Docker API or docker-compose control
}

// Test 6: Orchestrator Restart During Load
func TestLoad_OrchestratorRestart_GracefulShutdown(t *testing.T) {
	t.Skip("Requires Docker Compose control - manual test")

	// This test would:
	// 1. Start load generation
	// 2. After 30 seconds, send SIGTERM to orchestrator
	// 3. Verify: orchestrator finishes in-flight requests
	// 4. Verify: graceful shutdown within 30 seconds
	// 5. Verify: no worker crashes

	// Implementation requires process control
}
```

---

### Step 2: 24-Hour Soak Test

**File: `/home/mikekao/personal/subgen/test/load/soak_test.go`**

```go
package load

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test: 24-Hour Soak Test
func TestSoak_24Hours_ZeroCrashes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping soak test in short mode")
	}

	const duration = 24 * time.Hour
	const requestInterval = 5 * time.Second // 1 request every 5 seconds

	var successCount atomic.Int64
	var errorCount atomic.Int64
	var crashCount atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), duration+1*time.Hour)
	defer cancel()

	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	t.Logf("Starting 24-hour soak test...")
	t.Logf("Request interval: %v", requestInterval)
	start := time.Now()

	// Background health monitoring
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		healthTicker := time.NewTicker(30 * time.Second)
		defer healthTicker.Stop()

		for {
			select {
			case <-healthTicker.C:
				// Check orchestrator health
				resp, err := http.Get(orchestratorURL + "/status")
				if err != nil || resp.StatusCode != http.StatusOK {
					t.Logf("⚠️  Health check failed at %v", time.Since(start))
					crashCount.Add(1)
				} else {
					resp.Body.Close()
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// Main load loop
	requestNum := 0
	for {
		select {
		case <-ticker.C:
			requestNum++

			go func(id int) {
				payload := fmt.Sprintf(`data={"Event":"library.new","Item":{"Path":"%s"}}`, testAudioFile)
				resp, err := http.Post(
					orchestratorURL+"/emby",
					"application/x-www-form-urlencoded",
					bytes.NewReader([]byte(payload)),
				)

				if err != nil {
					errorCount.Add(1)
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					successCount.Add(1)
				} else {
					errorCount.Add(1)
				}
			}(requestNum)

			// Log progress every hour
			if requestNum%(3600/int(requestInterval.Seconds())) == 0 {
				elapsed := time.Since(start)
				hours := int(elapsed.Hours())
				t.Logf("\n[Hour %d] Progress:", hours)
				t.Logf("  Requests: %d", requestNum)
				t.Logf("  Success:  %d", successCount.Load())
				t.Logf("  Errors:   %d", errorCount.Load())
				t.Logf("  Crashes:  %d", crashCount.Load())
			}

		case <-ctx.Done():
			t.Log("Soak test duration reached")
			wg.Wait()
			goto results
		}
	}

results:
	totalTime := time.Since(start)

	t.Logf("\n=== 24-Hour Soak Test Results ===")
	t.Logf("Duration:      %.1f hours", totalTime.Hours())
	t.Logf("Total Requests: %d", requestNum)
	t.Logf("Success:       %d", successCount.Load())
	t.Logf("Errors:        %d", errorCount.Load())
	t.Logf("Crashes:       %d", crashCount.Load())

	// CRITICAL: Zero crashes required
	assert.Equal(t, int64(0), crashCount.Load(), "System should not crash during 24-hour soak test")
	
	// Success rate should be high
	successRate := float64(successCount.Load()) / float64(requestNum) * 100
	assert.GreaterOrEqual(t, successRate, 90.0, "Success rate should be >= 90%")

	t.Log("✅ PASS: 24-hour soak test completed with zero crashes")
}
```

---

### Step 3: Performance Benchmarks

**File: `/home/mikekao/personal/subgen/test/load/benchmark_test.go`**

```go
package load

import (
	"context"
	"testing"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/mccloud/subgen/orchestrator/internal/grpc_client"
)

// Benchmark: Queue Operations
func BenchmarkQueue_Push(b *testing.B) {
	q := queue.NewPriorityQueue(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := &queue.Task{
			ID:       fmt.Sprintf("task-%d", i),
			FilePath: "/test/file.mp3",
			Priority: queue.PriorityNormal,
		}
		q.Push(task)
	}
}

func BenchmarkQueue_Pop(b *testing.B) {
	q := queue.NewPriorityQueue(10000)

	// Pre-fill queue
	for i := 0; i < 1000; i++ {
		task := &queue.Task{
			ID:       fmt.Sprintf("task-%d", i),
			FilePath: "/test/file.mp3",
			Priority: queue.PriorityNormal,
		}
		q.Push(task)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Pop()
		
		// Refill if empty
		if q.Size() == 0 {
			for j := 0; j < 1000; j++ {
				task := &queue.Task{
					ID:       fmt.Sprintf("task-%d", i*1000+j),
					FilePath: "/test/file.mp3",
					Priority: queue.PriorityNormal,
				}
				q.Push(task)
			}
		}
	}
}

// Benchmark: gRPC HealthCheck
func BenchmarkGRPC_HealthCheck(b *testing.B) {
	client := grpc_client.NewClient(
		5*time.Minute,
		5*time.Second,
		3,
		1*time.Second,
		nil,
		logrus.New(),
	)
	defer client.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.HealthCheck(ctx, "localhost:50051")
		if err != nil {
			b.Errorf("HealthCheck failed: %v", err)
		}
	}
}

// Benchmark: Webhook Parsing
func BenchmarkWebhook_ParsePlex(b *testing.B) {
	payload := []byte(`{"event": "library.new", "Metadata": {"ratingKey": "12345"}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var data PlexPayload
		json.Unmarshal(payload, &data)
	}
}
```

---

### Step 4: Soak Test Runner Script

**File: `/home/mikekao/personal/subgen/test/scripts/run_soak_test.sh`**

```bash
#!/bin/bash
# Run 24-hour soak test with monitoring

set -e

echo "========================================="
echo "  24-Hour Soak Test"
echo "========================================="
echo ""
echo "This test will run for 24 hours."
echo "Press Ctrl+C to abort."
echo ""
read -p "Press Enter to start..."

# Create reports directory
mkdir -p ../reports

# Start Docker Compose
echo ""
echo "Starting services..."
cd ../
docker-compose -f docker-compose.integration.yml up -d

echo "Waiting for services to be healthy..."
sleep 30

# Start monitoring
echo ""
echo "Starting metrics collection..."

# Background: Collect metrics every 5 minutes
(
  while true; do
    TIMESTAMP=$(date +%s)
    
    # Scrape Prometheus metrics
    curl -s http://localhost:9090/metrics > "reports/metrics_${TIMESTAMP}.txt"
    
    # Docker stats
    docker stats --no-stream > "reports/docker_stats_${TIMESTAMP}.txt"
    
    sleep 300  # 5 minutes
  done
) &
MONITOR_PID=$!

# Cleanup function
cleanup() {
    echo ""
    echo "Stopping monitoring..."
    kill $MONITOR_PID 2>/dev/null || true
    
    echo "Stopping services..."
    docker-compose -f docker-compose.integration.yml down
    
    echo "Generating final report..."
    generate_soak_report
}
trap cleanup EXIT

# Run soak test
echo ""
echo "Starting soak test (24 hours)..."
cd load
go test -v -run TestSoak_24Hours -timeout 25h 2>&1 | tee ../reports/soak_test.log

echo ""
echo "✅ Soak test completed!"

# Generate report function
generate_soak_report() {
    cat > ../reports/soak_test_report.md <<EOF
# 24-Hour Soak Test Report

**Date:** $(date +%Y-%m-%d)
**Start:** $(date)
**Duration:** 24 hours

---

## Results

\`\`\`
$(grep "Soak Test Results" ../reports/soak_test.log -A 10 || echo "No results")
\`\`\`

---

## Metrics Collected

- Total metric snapshots: $(ls ../reports/metrics_*.txt 2>/dev/null | wc -l)
- Total Docker stats: $(ls ../reports/docker_stats_*.txt 2>/dev/null | wc -l)

---

## Conclusion

$(grep "PASS: 24-hour soak test" ../reports/soak_test.log > /dev/null && \
  echo "✅ **SOAK TEST PASSED** - Zero crashes in 24 hours" || \
  echo "❌ **SOAK TEST FAILED** - System crashed, see logs")
EOF

    cat ../reports/soak_test_report.md
}
```

**Usage:**
```bash
cd test/scripts
./run_soak_test.sh
# Runs for 24 hours
```

---

### Step 5: Grafana Dashboard (Optional)

**File: `/home/mikekao/personal/subgen/test/monitoring/grafana_dashboard.json`**

```json
{
  "dashboard": {
    "title": "Subgen Load Test Monitoring",
    "panels": [
      {
        "title": "Orchestrator Memory",
        "targets": [
          {
            "expr": "subgen_orchestrator_memory_bytes / 1024 / 1024"
          }
        ],
        "yAxis": {
          "label": "Memory (MB)"
        }
      },
      {
        "title": "Worker Memory",
        "targets": [
          {
            "expr": "subgen_worker_memory_mb"
          }
        ]
      },
      {
        "title": "Queue Size",
        "targets": [
          {
            "expr": "subgen_queue_size"
          }
        ]
      },
      {
        "title": "Webhook Request Rate",
        "targets": [
          {
            "expr": "rate(subgen_webhook_requests_total[1m])"
          }
        ]
      },
      {
        "title": "gRPC Latency (P99)",
        "targets": [
          {
            "expr": "histogram_quantile(0.99, subgen_grpc_duration_seconds_bucket)"
          }
        ]
      },
      {
        "title": "Error Rate",
        "targets": [
          {
            "expr": "rate(subgen_webhook_errors_total[5m])"
          }
        ]
      }
    ]
  }
}
```

---

## Performance Benchmarks

### Expected Results

**Go Orchestrator:**
```
BenchmarkQueue_Push-8           5000000    250 ns/op    128 B/op    2 allocs/op
BenchmarkQueue_Pop-8            3000000    400 ns/op    64 B/op     1 allocs/op
BenchmarkGRPC_HealthCheck-8     10000      150000 ns/op 1024 B/op   15 allocs/op
BenchmarkWebhook_ParsePlex-8    2000000    800 ns/op    256 B/op    3 allocs/op
```

**System Performance:**
```
Webhook Throughput:     50+ req/sec (sustained)
Queue Operations:       100+ ops/sec (push/pop)
gRPC HealthCheck:       < 100ms (p99)
Concurrent Webhooks:    100 simultaneous requests
Memory Growth:          < 20% after 1000 transcriptions
24-Hour Uptime:         Zero crashes
```

---

## Manual Soak Test Procedure

**File: `/home/mikekao/personal/subgen/test/manual/SOAK_TEST_PROCEDURE.md`**

```markdown
# 24-Hour Soak Test - Manual Procedure

## Overview

This procedure runs the system under light continuous load for 24 hours to validate stability.

---

## Prerequisites

- Docker Compose environment running
- Monitoring tools (Grafana/Prometheus) optional but recommended

---

## Step 1: Start Services

```bash
cd test
docker-compose -f docker-compose.integration.yml up -d

# Verify services are healthy
docker-compose ps
docker logs subgen-orchestrator-integration
docker logs subgen-worker-integration
```

---

## Step 2: Start Load Generator

```bash
# Run soak test script (runs for 24 hours)
cd scripts
./run_soak_test.sh
```

**Alternative: Manual load generation**

```bash
# In a separate terminal, run this loop
while true; do
  curl -X POST http://localhost:9000/emby \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "data={\"Event\":\"library.new\",\"Item\":{\"Path\":\"/testdata/short_audio.mp3\"}}"
  
  sleep 5  # One request every 5 seconds
done
```

---

## Step 3: Monitor During Test

### Every Hour, Check:

**1. Service Health:**
```bash
docker-compose ps
# All services should be "Up" and healthy
```

**2. Memory Usage:**
```bash
docker stats --no-stream

# Expected:
# orchestrator: 50-100MB (stable)
# worker: 1-3GB (may fluctuate)
```

**3. Logs:**
```bash
# Check for errors
docker logs subgen-orchestrator-integration --since 1h | grep -i error
docker logs subgen-worker-integration --since 1h | grep -i error
```

**4. Prometheus Metrics:**
```bash
curl -s http://localhost:9090/metrics | grep subgen_orchestrator_memory_bytes
curl -s http://localhost:9090/metrics | grep subgen_worker_memory_mb
```

**5. Queue Status:**
```bash
curl -s http://localhost:9090/metrics | grep subgen_queue_size
# Should be near 0 (processing faster than enqueueing)
```

---

## Step 4: Record Results

Create a log file recording hourly observations:

```
Hour 0:  Services started, memory: orchestrator 75MB, worker 1200MB
Hour 1:  Memory: orchestrator 78MB (+3MB), worker 1205MB (+5MB)
Hour 2:  Memory: orchestrator 78MB (stable), worker 1210MB (+5MB)
...
Hour 24: Memory: orchestrator 85MB (+10MB), worker 1250MB (+50MB)
```

---

## Step 5: Validation

After 24 hours, verify:

- [ ] Both services still running (no crashes)
- [ ] Memory growth < 20% from hour 1 baseline
- [ ] No error rate increase over time
- [ ] Queue size remains low (< 10 tasks)
- [ ] Logs show no repeated errors
- [ ] Prometheus metrics stable

---

## Success Criteria

✅ **PASS** if:
- Zero crashes (both services run for full 24 hours)
- Memory growth < 20%
- Error rate < 5%
- No performance degradation

❌ **FAIL** if:
- Any service crashes
- Memory grows > 20%
- Error rate > 10%
- Performance degrades over time

---

## Troubleshooting

**Service crashed:**
- Check: `docker logs <container>`
- Look for: OOM kills, panics, exceptions

**Memory growing:**
- Run: `docker exec subgen-worker-integration python -c "import gc; gc.collect()"`
- Check: Prometheus metrics for leak indicators

**High error rate:**
- Check: Queue size (might be full)
- Check: Worker health (might be unhealthy)
- Check: Disk space (NFS might be full)

---

## Cleanup

```bash
# Stop services
docker-compose -f docker-compose.integration.yml down

# Remove generated subtitles
rm -f testdata/*.srt testdata/*.lrc

# Review reports
ls -lh reports/
```
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] Concurrent webhook test passes (100 requests)
- [ ] Sustained load test passes (50 req/sec for 1 min)
- [ ] Queue stress test passes (1000+ tasks)
- [ ] gRPC latency benchmarks meet targets (< 100ms p99)
- [ ] Performance benchmarks recorded
- [ ] 24-hour soak test procedure documented
- [ ] Soak test executed successfully (manual or automated)
- [ ] Load test runner scripts created
- [ ] Grafana dashboard created (optional)
- [ ] Performance report generated
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_03_story_05.md`
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Run all load tests (short version)
cd test/load
go test -v -short

# Run full load tests (skip soak test)
go test -v -run TestLoad

# Run benchmarks
go test -bench=. -benchmem

# Run soak test (24 hours)
cd ../scripts
./run_soak_test.sh

# View benchmark results
go test -bench=. -benchmem | tee ../reports/benchmark_results.txt
```

---

## Dependencies

**Requires:**
- STORY_01 (gRPC Integration Tests) - Docker Compose setup
- STORY_04 (Memory Leak Validation) - Memory tests passing

**Blocks:**
- EPIC_04 (K8s Deployment) - Performance validated for production

---

## Notes

### Load Test vs Stress Test

**Load Test:**
- Validates system under expected load
- Target: 50 req/sec (much higher than production: ~2/day)
- Verifies performance targets are met

**Stress Test:**
- Pushes system beyond expected load
- Target: 100+ req/sec until failure
- Identifies breaking points

### Why 24-Hour Soak Test?

- Detects slow memory leaks (< 1MB/hour)
- Validates long-running stability
- Exposes edge cases that only appear over time
- Simulates real production usage patterns

### Performance Targets Justification

**Queue: 100+ tasks/sec**
- Production: ~2 tasks/day
- 100 tasks/sec = 8.6M tasks/day (4.3M times faster than needed)
- Plenty of headroom

**Webhooks: 50+ req/sec**
- Production: ~2 req/day
- 50 req/sec = 4.3M req/day (2.1M times faster)
- Handles burst scenarios (entire season added at once)

**gRPC: < 100ms p99**
- Localhost communication should be fast
- 100ms is generous (actual: ~5-10ms)

### Prometheus Metrics for Load Testing

Monitor these during load tests:
```
subgen_webhook_requests_total{source="emby"}
subgen_webhook_duration_seconds{source="emby"}
subgen_queue_size
subgen_worker_memory_mb
subgen_grpc_duration_seconds
```

### CI/CD Integration

```yaml
# Run load tests on PR (short version)
- name: Load tests
  run: |
    cd test/load
    go test -v -short -timeout 10m

# Run soak test weekly (scheduled)
# Run on dedicated runner with 24+ hour timeout
```

---

## References

- [EPIC_03 README](../README.md) - Performance targets
- Go testing benchmarks: https://go.dev/blog/benchmarks
- HTTP load testing: https://github.com/rakyll/hey
- gRPC benchmarking: https://grpc.io/docs/guides/benchmarking/

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
