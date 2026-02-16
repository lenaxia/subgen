# STORY_04: Priority Queue Management with Deduplication

**Status:** Not Started  
**Effort:** 8-10 hours  
**Epic:** EPIC_01 (Go Orchestrator Core)  
**Created:** 2026-02-15

---

## User Story

**As a** developer  
**I want** a bounded priority queue with deduplication and status tracking  
**So that** transcription tasks are processed in order without duplicates and memory stays bounded

---

## Acceptance Criteria

- [ ] PriorityQueue implementation with 3 priority levels
- [ ] Deduplication prevents same file from being queued twice
- [ ] Status tracking for queued vs processing tasks
- [ ] Thread-safe operations with mutex/RWMutex
- [ ] Bounded queue with configurable max size
- [ ] Queue full returns error (no silent drops)
- [ ] Priority levels: 0 (Detect), 1 (ASR), 2 (Transcribe)
- [ ] FIFO within same priority (tie-breaker by timestamp)
- [ ] 12+ test cases covering all edge cases
- [ ] Prometheus metrics for queue size and wait time
- [ ] Work log created

---

## Integration Points

### Legacy Queue Implementation (subgen.py:272-324)

**Location:** `/home/mikekao/personal/subgen/subgen.py:272-324`

**Current Python Implementation:**

```python
class DeduplicatedQueue(queue.PriorityQueue):
    """Queue that prevents duplicates, handles priority, and tracks status."""
    def __init__(self):
        super().__init__()
        self._queued = set()     # Tracks task IDs waiting in queue
        self._processing = set() # Tracks task IDs currently being handled
        self._lock = Lock()

    def put(self, item, block=True, timeout=None):
        with self._lock:
            task_id = item["path"]
            if task_id not in self._queued and task_id not in self._processing:
                # Priority: 0 (Detect), 1 (ASR), 2 (Transcribe)
                task_type = item.get("type", "transcribe")
                priority = 0 if task_type == "detect_language" else (1 if task_type == "asr" else 2)
                
                # PriorityQueue requires a tuple: (priority, tie_breaker, item)
                super().put((priority, time.time(), item), block, timeout)
                self._queued.add(task_id)
                return True
            return False

    def get(self, block=True, timeout=None):
        # PriorityQueue returns the tuple, we want just the item
        priority, timestamp, item = super().get(block, timeout)
        with self._lock:
            task_id = item["path"]
            self._queued.discard(task_id)
            self._processing.add(task_id)
        return item

    def mark_done(self, item):
        with self._lock:
            task_id = item["path"]
            self._processing.discard(task_id)

    def is_idle(self):
        with self._lock:
            return self.empty() and len(self._processing) == 0

    def is_active(self, task_id):
        """Checks if a task_id is currently queued or processing."""
        with self._lock:
            return task_id in self._queued or task_id in self._processing

    def get_queued_tasks(self):
        with self._lock:
            return list(self._queued)

    def get_processing_tasks(self):
        with self._lock:
            return list(self._processing)
```

**Key Behaviors to Replicate:**

1. **Deduplication:** Uses `task_id` (file path) to prevent duplicates
2. **Priority Levels:**
   - `0` = Detect language (highest priority)
   - `1` = ASR (Bazarr direct transcription)
   - `2` = Transcribe (webhook-triggered, lowest priority)
3. **Status Tracking:** Separate sets for queued vs processing
4. **Tie-Breaking:** Uses `time.time()` for FIFO within same priority
5. **Atomic Operations:** All mutations protected by `_lock`

---

## Technical Design

### File Structure

```
internal/queue/
├── queue.go            # Main queue implementation
├── queue_test.go       # Unit tests
├── task.go             # Task struct definition
├── priority.go         # Priority constants and helpers
└── metrics.go          # Prometheus metrics
```

---

### Task Definition (task.go)

**File:** `internal/queue/task.go`

```go
package queue

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// TaskType represents the type of transcription task
type TaskType string

const (
	TaskTypeDetectLanguage TaskType = "detect_language"
	TaskTypeASR            TaskType = "asr"
	TaskTypeTranscribe     TaskType = "transcribe"
)

// Priority levels (lower number = higher priority)
type Priority int

const (
	PriorityDetectLanguage Priority = 0 // Highest priority
	PriorityASR            Priority = 1 // Medium priority
	PriorityTranscribe     Priority = 2 // Lowest priority
)

// Task represents a transcription task
type Task struct {
	// Identification
	ID       string    // SHA256 hash of FilePath
	FilePath string    // Absolute path to media file
	Type     TaskType  // Type of task
	Priority Priority  // Priority level
	
	// Transcription Options
	TaskType       string // "transcribe" or "translate"
	ForceLanguage  string // ISO 639-1 code or empty
	
	// Media Server Metadata (for refresh after completion)
	PlexItemID     string // Plex rating key
	PlexServer     string // Plex server URL
	PlexToken      string // Plex auth token
	JellyfinItemID string // Jellyfin item ID
	JellyfinServer string // Jellyfin server URL
	JellyfinToken  string // Jellyfin auth token
	
	// ASR-specific fields
	AudioContent   []byte            // For ASR tasks (Bazarr upload)
	ASROptions     map[string]string // ASR query parameters
	
	// Timing
	QueuedAt    time.Time // When task was enqueued
	StartedAt   time.Time // When task started processing (zero if not started)
	CompletedAt time.Time // When task completed (zero if not completed)
}

// NewTask creates a new task with computed ID
func NewTask(filePath string, taskType TaskType) *Task {
	t := &Task{
		FilePath:  filePath,
		Type:      taskType,
		Priority:  TaskTypeToPriority(taskType),
		QueuedAt:  time.Now(),
		TaskType:  "transcribe", // Default
	}
	t.ID = t.ComputeID()
	return t
}

// ComputeID generates a unique ID from file path
func (t *Task) ComputeID() string {
	hash := sha256.Sum256([]byte(t.FilePath))
	return hex.EncodeToString(hash[:])
}

// TaskTypeToPriority converts task type to priority
func TaskTypeToPriority(taskType TaskType) Priority {
	switch taskType {
	case TaskTypeDetectLanguage:
		return PriorityDetectLanguage
	case TaskTypeASR:
		return PriorityASR
	case TaskTypeTranscribe:
		return PriorityTranscribe
	default:
		return PriorityTranscribe
	}
}

// WaitTime returns how long task has been waiting
func (t *Task) WaitTime() time.Duration {
	if !t.StartedAt.IsZero() {
		return t.StartedAt.Sub(t.QueuedAt)
	}
	return time.Since(t.QueuedAt)
}
```

---

### Priority Queue Implementation (queue.go)

**File:** `internal/queue/queue.go`

```go
package queue

import (
	"container/heap"
	"errors"
	"fmt"
	"sync"
	"time"
	
	"github.com/sirupsen/logrus"
)

var (
	ErrQueueFull      = errors.New("queue is full")
	ErrDuplicateTask  = errors.New("task already queued or processing")
	ErrQueueEmpty     = errors.New("queue is empty")
	ErrTaskNotFound   = errors.New("task not found")
)

// Queue is a thread-safe bounded priority queue with deduplication
type Queue struct {
	mu sync.RWMutex
	
	// Priority heap for tasks
	heap *taskHeap
	
	// Deduplication tracking (task ID → task pointer)
	queued     map[string]*Task // Tasks waiting in queue
	processing map[string]*Task // Tasks currently being processed
	
	// Configuration
	maxSize int
	
	// Metrics
	metrics *QueueMetrics
	
	// Logger
	log *logrus.Logger
}

// NewQueue creates a new priority queue
func NewQueue(maxSize int, metrics *QueueMetrics, log *logrus.Logger) *Queue {
	h := &taskHeap{}
	heap.Init(h)
	
	return &Queue{
		heap:       h,
		queued:     make(map[string]*Task),
		processing: make(map[string]*Task),
		maxSize:    maxSize,
		metrics:    metrics,
		log:        log,
	}
}

// Enqueue adds a task to the queue
// Returns ErrQueueFull if queue is at capacity
// Returns ErrDuplicateTask if task is already queued or processing
func (q *Queue) Enqueue(task *Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	// Check for duplicates
	if _, exists := q.queued[task.ID]; exists {
		q.log.WithFields(logrus.Fields{
			"task_id":   task.ID,
			"file_path": task.FilePath,
		}).Debug("Task already queued, skipping")
		return ErrDuplicateTask
	}
	
	if _, exists := q.processing[task.ID]; exists {
		q.log.WithFields(logrus.Fields{
			"task_id":   task.ID,
			"file_path": task.FilePath,
		}).Debug("Task already processing, skipping")
		return ErrDuplicateTask
	}
	
	// Check queue size
	if q.heap.Len() >= q.maxSize {
		q.log.WithFields(logrus.Fields{
			"queue_size": q.heap.Len(),
			"max_size":   q.maxSize,
		}).Warn("Queue is full")
		return ErrQueueFull
	}
	
	// Add to heap and tracking map
	task.QueuedAt = time.Now()
	heap.Push(q.heap, task)
	q.queued[task.ID] = task
	
	// Update metrics
	q.metrics.QueueSize.Set(float64(q.heap.Len()))
	q.metrics.TasksQueued.WithLabelValues(string(task.Type)).Inc()
	
	q.log.WithFields(logrus.Fields{
		"task_id":   task.ID,
		"file_path": task.FilePath,
		"priority":  task.Priority,
		"type":      task.Type,
	}).Info("Task enqueued")
	
	return nil
}

// Dequeue removes and returns the highest priority task
// Returns ErrQueueEmpty if queue is empty
func (q *Queue) Dequeue() (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if q.heap.Len() == 0 {
		return nil, ErrQueueEmpty
	}
	
	// Pop from heap
	task := heap.Pop(q.heap).(*Task)
	
	// Move from queued to processing
	delete(q.queued, task.ID)
	task.StartedAt = time.Now()
	q.processing[task.ID] = task
	
	// Update metrics
	q.metrics.QueueSize.Set(float64(q.heap.Len()))
	q.metrics.ProcessingSize.Set(float64(len(q.processing)))
	q.metrics.TaskWaitTime.WithLabelValues(string(task.Type)).Observe(task.WaitTime().Seconds())
	
	q.log.WithFields(logrus.Fields{
		"task_id":   task.ID,
		"file_path": task.FilePath,
		"wait_time": task.WaitTime(),
	}).Info("Task dequeued")
	
	return task, nil
}

// MarkDone marks a task as completed and removes from processing set
func (q *Queue) MarkDone(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	task, exists := q.processing[taskID]
	if !exists {
		return ErrTaskNotFound
	}
	
	task.CompletedAt = time.Now()
	delete(q.processing, taskID)
	
	// Update metrics
	q.metrics.ProcessingSize.Set(float64(len(q.processing)))
	q.metrics.TasksCompleted.WithLabelValues(string(task.Type)).Inc()
	
	processingTime := task.CompletedAt.Sub(task.StartedAt).Seconds()
	q.metrics.TaskProcessingTime.WithLabelValues(string(task.Type)).Observe(processingTime)
	
	q.log.WithFields(logrus.Fields{
		"task_id":         task.ID,
		"file_path":       task.FilePath,
		"processing_time": processingTime,
	}).Info("Task completed")
	
	return nil
}

// MarkFailed removes task from processing without marking as complete
func (q *Queue) MarkFailed(taskID string, err error) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	task, exists := q.processing[taskID]
	if !exists {
		return ErrTaskNotFound
	}
	
	delete(q.processing, taskID)
	
	// Update metrics
	q.metrics.ProcessingSize.Set(float64(len(q.processing)))
	q.metrics.TasksFailed.WithLabelValues(string(task.Type)).Inc()
	
	q.log.WithFields(logrus.Fields{
		"task_id":   task.ID,
		"file_path": task.FilePath,
		"error":     err,
	}).Error("Task failed")
	
	return nil
}

// IsActive checks if a task is queued or processing
func (q *Queue) IsActive(taskID string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	_, inQueue := q.queued[taskID]
	_, inProcessing := q.processing[taskID]
	
	return inQueue || inProcessing
}

// IsIdle returns true if queue is empty and nothing is processing
func (q *Queue) IsIdle() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	return q.heap.Len() == 0 && len(q.processing) == 0
}

// Size returns current queue size (not including processing)
func (q *Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	return q.heap.Len()
}

// ProcessingCount returns number of tasks currently processing
func (q *Queue) ProcessingCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	return len(q.processing)
}

// GetQueuedTasks returns list of queued task IDs
func (q *Queue) GetQueuedTasks() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	tasks := make([]string, 0, len(q.queued))
	for id := range q.queued {
		tasks = append(tasks, id)
	}
	return tasks
}

// GetProcessingTasks returns list of processing task IDs
func (q *Queue) GetProcessingTasks() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	tasks := make([]string, 0, len(q.processing))
	for id := range q.processing {
		tasks = append(tasks, id)
	}
	return tasks
}

// taskHeap implements heap.Interface for priority queue
type taskHeap []*Task

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	// Lower priority value = higher priority
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	// Tie-breaker: earlier queued time = higher priority (FIFO)
	return h[i].QueuedAt.Before(h[j].QueuedAt)
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *taskHeap) Push(x interface{}) {
	*h = append(*h, x.(*Task))
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	task := old[n-1]
	*h = old[0 : n-1]
	return task
}
```

---

### Prometheus Metrics (metrics.go)

**File:** `internal/queue/metrics.go`

```go
package queue

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// QueueMetrics holds Prometheus metrics for the queue
type QueueMetrics struct {
	// Gauge metrics
	QueueSize       prometheus.Gauge
	ProcessingSize  prometheus.Gauge
	
	// Counter metrics
	TasksQueued    *prometheus.CounterVec
	TasksCompleted *prometheus.CounterVec
	TasksFailed    *prometheus.CounterVec
	
	// Histogram metrics
	TaskWaitTime       *prometheus.HistogramVec
	TaskProcessingTime *prometheus.HistogramVec
}

// NewQueueMetrics creates Prometheus metrics for the queue
func NewQueueMetrics() *QueueMetrics {
	return &QueueMetrics{
		QueueSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "subgen_queue_size",
			Help: "Current number of tasks in the queue",
		}),
		
		ProcessingSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "subgen_queue_processing_size",
			Help: "Current number of tasks being processed",
		}),
		
		TasksQueued: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_tasks_queued_total",
				Help: "Total number of tasks queued",
			},
			[]string{"type"}, // Task type label
		),
		
		TasksCompleted: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_tasks_completed_total",
				Help: "Total number of tasks completed",
			},
			[]string{"type"},
		),
		
		TasksFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "subgen_tasks_failed_total",
				Help: "Total number of tasks that failed",
			},
			[]string{"type"},
		),
		
		TaskWaitTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "subgen_task_wait_time_seconds",
				Help:    "Time tasks spent waiting in queue",
				Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1800}, // 1s to 30min
			},
			[]string{"type"},
		),
		
		TaskProcessingTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "subgen_task_processing_time_seconds",
				Help:    "Time tasks spent processing",
				Buckets: []float64{10, 30, 60, 120, 300, 600, 1800, 3600}, // 10s to 1hr
			},
			[]string{"type"},
		),
	}
}
```

---

## Test Cases

### Unit Tests (12+)

**File:** `internal/queue/queue_test.go`

```go
package queue

import (
	"testing"
	"time"
	
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueue(t *testing.T) {
	metrics := NewQueueMetrics()
	log := logrus.New()
	q := NewQueue(100, metrics, log)
	
	assert.NotNil(t, q)
	assert.Equal(t, 0, q.Size())
	assert.True(t, q.IsIdle())
}

func TestEnqueue_Success(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	
	err := q.Enqueue(task)
	
	require.NoError(t, err)
	assert.Equal(t, 1, q.Size())
	assert.False(t, q.IsIdle())
}

func TestEnqueue_Duplicate(t *testing.T) {
	q := newTestQueue(100)
	task1 := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	
	err1 := q.Enqueue(task1)
	err2 := q.Enqueue(task2)
	
	require.NoError(t, err1)
	assert.ErrorIs(t, err2, ErrDuplicateTask)
	assert.Equal(t, 1, q.Size())
}

func TestEnqueue_QueueFull(t *testing.T) {
	q := newTestQueue(2) // Max size = 2
	
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)
	task3 := NewTask("/media/movie3.mkv", TaskTypeTranscribe)
	
	err1 := q.Enqueue(task1)
	err2 := q.Enqueue(task2)
	err3 := q.Enqueue(task3) // Should fail
	
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.ErrorIs(t, err3, ErrQueueFull)
	assert.Equal(t, 2, q.Size())
}

func TestDequeue_Success(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	
	q.Enqueue(task)
	dequeuedTask, err := q.Dequeue()
	
	require.NoError(t, err)
	assert.Equal(t, task.ID, dequeuedTask.ID)
	assert.Equal(t, 0, q.Size())
	assert.Equal(t, 1, q.ProcessingCount())
}

func TestDequeue_EmptyQueue(t *testing.T) {
	q := newTestQueue(100)
	
	task, err := q.Dequeue()
	
	assert.ErrorIs(t, err, ErrQueueEmpty)
	assert.Nil(t, task)
}

func TestPriorityOrdering(t *testing.T) {
	q := newTestQueue(100)
	
	// Enqueue tasks in reverse priority order
	task1 := NewTask("/media/transcribe.mkv", TaskTypeTranscribe) // Priority 2
	task2 := NewTask("/media/asr.mp3", TaskTypeASR)               // Priority 1
	task3 := NewTask("/media/detect.mkv", TaskTypeDetectLanguage) // Priority 0
	
	q.Enqueue(task1)
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	q.Enqueue(task2)
	time.Sleep(1 * time.Millisecond)
	q.Enqueue(task3)
	
	// Dequeue should return in priority order (0, 1, 2)
	t1, _ := q.Dequeue()
	t2, _ := q.Dequeue()
	t3, _ := q.Dequeue()
	
	assert.Equal(t, TaskTypeDetectLanguage, t1.Type)
	assert.Equal(t, TaskTypeASR, t2.Type)
	assert.Equal(t, TaskTypeTranscribe, t3.Type)
}

func TestFIFO_WithinSamePriority(t *testing.T) {
	q := newTestQueue(100)
	
	// Enqueue 3 tasks with same priority
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)
	task3 := NewTask("/media/movie3.mkv", TaskTypeTranscribe)
	
	q.Enqueue(task1)
	time.Sleep(1 * time.Millisecond)
	q.Enqueue(task2)
	time.Sleep(1 * time.Millisecond)
	q.Enqueue(task3)
	
	// Should dequeue in FIFO order
	t1, _ := q.Dequeue()
	t2, _ := q.Dequeue()
	t3, _ := q.Dequeue()
	
	assert.Equal(t, task1.ID, t1.ID)
	assert.Equal(t, task2.ID, t2.ID)
	assert.Equal(t, task3.ID, t3.ID)
}

func TestMarkDone(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	
	q.Enqueue(task)
	dequeuedTask, _ := q.Dequeue()
	err := q.MarkDone(dequeuedTask.ID)
	
	require.NoError(t, err)
	assert.Equal(t, 0, q.ProcessingCount())
	assert.True(t, q.IsIdle())
}

func TestMarkFailed(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	
	q.Enqueue(task)
	dequeuedTask, _ := q.Dequeue()
	err := q.MarkFailed(dequeuedTask.ID, errors.New("test error"))
	
	require.NoError(t, err)
	assert.Equal(t, 0, q.ProcessingCount())
	assert.True(t, q.IsIdle())
}

func TestIsActive(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	
	// Not active initially
	assert.False(t, q.IsActive(task.ID))
	
	// Active when queued
	q.Enqueue(task)
	assert.True(t, q.IsActive(task.ID))
	
	// Active when processing
	q.Dequeue()
	assert.True(t, q.IsActive(task.ID))
	
	// Not active when done
	q.MarkDone(task.ID)
	assert.False(t, q.IsActive(task.ID))
}

func TestGetQueuedTasks(t *testing.T) {
	q := newTestQueue(100)
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)
	
	q.Enqueue(task1)
	q.Enqueue(task2)
	
	queued := q.GetQueuedTasks()
	
	assert.Equal(t, 2, len(queued))
	assert.Contains(t, queued, task1.ID)
	assert.Contains(t, queued, task2.ID)
}

func TestGetProcessingTasks(t *testing.T) {
	q := newTestQueue(100)
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)
	
	q.Enqueue(task1)
	q.Enqueue(task2)
	q.Dequeue() // Move task1 to processing
	
	processing := q.GetProcessingTasks()
	
	assert.Equal(t, 1, len(processing))
	assert.Contains(t, processing, task1.ID)
}

// Helper function to create test queue
func newTestQueue(maxSize int) *Queue {
	metrics := NewQueueMetrics()
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Suppress logs in tests
	return NewQueue(maxSize, metrics, log)
}
```

---

## Implementation Steps

### Step 1: Create Task Struct (1 hour)
```bash
cd /home/mikekao/personal/subgen/orchestrator
mkdir -p internal/queue
touch internal/queue/task.go
```

1. Copy task.go implementation from above
2. Define TaskType, Priority constants
3. Implement NewTask constructor
4. Implement ComputeID using SHA256
5. Add WaitTime helper method

### Step 2: Create Priority Queue (2 hours)
```bash
touch internal/queue/queue.go
```

1. Implement taskHeap (heap.Interface)
2. Implement Queue struct with mutex
3. Implement Enqueue with deduplication
4. Implement Dequeue with priority ordering
5. Implement MarkDone and MarkFailed
6. Implement IsActive, IsIdle helpers

### Step 3: Add Metrics (1 hour)
```bash
touch internal/queue/metrics.go
```

1. Define QueueMetrics struct
2. Create Prometheus gauges (queue_size, processing_size)
3. Create counters (tasks_queued, completed, failed)
4. Create histograms (wait_time, processing_time)
5. Integrate metrics into queue operations

### Step 4: Write Tests (3 hours)
```bash
touch internal/queue/queue_test.go
```

1. Test basic enqueue/dequeue
2. Test deduplication
3. Test queue full error
4. Test priority ordering
5. Test FIFO within priority
6. Test status tracking
7. Test thread safety (concurrent enqueues)
8. Run tests: `go test ./internal/queue -v`

### Step 5: Integration with Webhooks (1 hour)

Update webhook handlers to use queue:

```go
// internal/webhooks/server.go

type Server struct {
	queue *queue.Queue // Add queue field
	// ... other fields
}

func (s *Server) handlePlex(c *fiber.Ctx) error {
	// ... parse payload
	
	task := queue.NewTask(filePath, queue.TaskTypeTranscribe)
	task.PlexItemID = ratingKey
	task.PlexServer = s.config.Plex.Server
	task.PlexToken = s.config.Plex.Token
	
	if err := s.queue.Enqueue(task); err != nil {
		if errors.Is(err, queue.ErrDuplicateTask) {
			return c.Status(200).JSON(fiber.Map{"message": "Task already queued"})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	
	return c.Status(202).JSON(fiber.Map{"message": "Task queued"})
}
```

### Step 6: Add Queue Status Endpoint (30 min)

```go
// internal/webhooks/server.go

func (s *Server) handleQueueStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"queue_size":      s.queue.Size(),
		"processing":      s.queue.ProcessingCount(),
		"queued_tasks":    s.queue.GetQueuedTasks(),
		"processing_tasks": s.queue.GetProcessingTasks(),
	})
}
```

### Step 7: Documentation (30 min)

Create work log documenting:
- Priority logic research from Python
- Deduplication design decisions
- Thread safety considerations
- Metric definitions

---

## Dependencies

**Requires:**
- STORY_01 (Project Setup) ✅
- STORY_02 (Configuration) ✅

**Blocks:**
- STORY_05 (Media Server Clients) - needs queue
- STORY_06 (Worker Discovery) - needs queue
- STORY_07 (gRPC Client) - needs queue

---

## Definition of Done

- [ ] All 12+ tests passing
- [ ] Priority ordering works (0, 1, 2)
- [ ] FIFO within same priority
- [ ] Deduplication prevents duplicates
- [ ] Queue full returns error
- [ ] Thread-safe (mutex protection)
- [ ] Prometheus metrics exported
- [ ] Status tracking (queued vs processing)
- [ ] Integration with webhook handlers
- [ ] Queue status endpoint working
- [ ] Code passes golangci-lint
- [ ] Work log created
- [ ] Coverage > 80% for queue package

---

## Notes

### Key Design Decisions

1. **Why SHA256 for task ID?**
   - Consistent hashing for same file path
   - No collisions
   - Same approach as legacy Python code

2. **Why container/heap?**
   - Efficient priority queue (O(log n) insert/remove)
   - Standard library, well-tested
   - Same semantics as Python's heapq

3. **Why separate queued/processing maps?**
   - Matches legacy behavior
   - Allows checking if task is active
   - Prevents re-queueing processing tasks

4. **Why bounded queue?**
   - Prevents OOM on webhook flood
   - Forces backpressure
   - Matches legacy MAX_QUEUE_SIZE

### References

- Legacy implementation: `subgen.py:272-324`
- Go heap interface: https://pkg.go.dev/container/heap
- Prometheus metrics: https://prometheus.io/docs/guides/go-application/

---

**Owner:** TBD  
**Created:** 2026-02-15  
**Last Updated:** 2026-02-15
