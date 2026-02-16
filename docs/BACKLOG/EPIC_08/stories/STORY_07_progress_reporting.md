# Story 07: Queue Status & Progress Reporting

**Epic**: EPIC_08  
**Status**: Not Started  
**Effort**: 4-6 hours  
**Priority**: LOW  
**Assignee**: Delegation Agent

---

## User Story

As a Subgen administrator or monitoring tool,
I want to query queue status and transcription progress,
So that I can monitor system health, track progress, and troubleshoot issues.

---

## Background

Currently, there's no way to inspect queue state or monitor transcription progress via API. Users must check logs to understand what's happening. This story adds REST endpoints for:
- Queue status (queued/processing/completed counts)
- Active transcriptions with progress
- Recent completion history
- Individual task status lookup

---

## Acceptance Criteria

- [ ] `GET /queue/status` - Current queue state and statistics
- [ ] `GET /queue/processing` - List of active transcriptions
- [ ] `GET /queue/history` - Recent completions (last 100)
- [ ] `GET /tasks/{task_id}` - Individual task status
- [ ] JSON responses with clear structure
- [ ] Metrics: queue depth, processing count, completion rate
- [ ] Performance: Endpoints respond in <100ms
- [ ] Thread-safe access to queue state
- [ ] Unit tests for all endpoints
- [ ] Integration tests
- [ ] Type checking passes
- [ ] Work log created

---

## Technical Design

### API Endpoints

#### 1. GET /queue/status

**Response:**
```json
{
  "status": "active",
  "queued": 15,
  "processing": 2,
  "completed_last_hour": 47,
  "failed_last_hour": 1,
  "idle": false,
  "uptime_seconds": 86400,
  "workers": {
    "total": 2,
    "active": 2,
    "idle": 0
  }
}
```

#### 2. GET /queue/processing

**Response:**
```json
{
  "tasks": [
    {
      "id": "task-12345",
      "file_path": "/movies/action/movie.mkv",
      "priority": 2,
      "started_at": "2026-02-16T12:34:56Z",
      "progress": 65,
      "eta_seconds": 120,
      "worker_id": 1
    },
    {
      "id": "task-12346",
      "file_path": "/tv/show/s01e02.mkv",
      "priority": 1,
      "started_at": "2026-02-16T12:35:10Z",
      "progress": 23,
      "eta_seconds": 300,
      "worker_id": 2
    }
  ]
}
```

#### 3. GET /queue/history

**Query Parameters:**
- `limit` (optional, default: 100) - Max results to return
- `offset` (optional, default: 0) - Pagination offset

**Response:**
```json
{
  "tasks": [
    {
      "id": "task-12344",
      "file_path": "/movies/comedy/movie.mkv",
      "status": "completed",
      "started_at": "2026-02-16T12:30:00Z",
      "completed_at": "2026-02-16T12:33:45Z",
      "duration_seconds": 225,
      "output_file": "/movies/comedy/movie.eng.srt"
    },
    {
      "id": "task-12343",
      "file_path": "/tv/show/s01e01.mkv",
      "status": "failed",
      "started_at": "2026-02-16T12:25:00Z",
      "completed_at": "2026-02-16T12:26:30Z",
      "duration_seconds": 90,
      "error": "audio extraction failed: no audio tracks"
    }
  ],
  "total": 1247,
  "limit": 100,
  "offset": 0
}
```

#### 4. GET /tasks/{task_id}

**Response (In Progress):**
```json
{
  "id": "task-12345",
  "file_path": "/movies/action/movie.mkv",
  "status": "processing",
  "priority": 2,
  "queued_at": "2026-02-16T12:30:00Z",
  "started_at": "2026-02-16T12:34:56Z",
  "progress": 65,
  "eta_seconds": 120,
  "worker_id": 1
}
```

**Response (Completed):**
```json
{
  "id": "task-12344",
  "file_path": "/movies/comedy/movie.mkv",
  "status": "completed",
  "priority": 2,
  "queued_at": "2026-02-16T12:29:00Z",
  "started_at": "2026-02-16T12:30:00Z",
  "completed_at": "2026-02-16T12:33:45Z",
  "duration_seconds": 225,
  "output_file": "/movies/comedy/movie.eng.srt"
}
```

**Response (Not Found):**
```json
{
  "status": "error",
  "error": "task not found: task-99999"
}
```

### Approach

1. **Task Tracking** - Extend queue to track task lifecycle
2. **History Storage** - In-memory circular buffer for recent tasks
3. **Progress Reporting** - Workers report progress via gRPC callbacks (future)
4. **Status Endpoints** - Handlers query queue state

### Data Structures

```go
// Task status tracking
type TaskStatus string

const (
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// Extended task info
type TaskInfo struct {
	ID           string
	FilePath     string
	Status       TaskStatus
	Priority     int
	QueuedAt     time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Duration     time.Duration
	OutputFile   string
	Error        string
	Progress     int       // 0-100
	ETASeconds   int       // Estimated time remaining
	WorkerID     int
}

// History storage (circular buffer)
type TaskHistory struct {
	mu      sync.RWMutex
	tasks   []TaskInfo
	maxSize int
	index   int
}

func NewTaskHistory(maxSize int) *TaskHistory {
	return &TaskHistory{
		tasks:   make([]TaskInfo, 0, maxSize),
		maxSize: maxSize,
	}
}

func (h *TaskHistory) Add(task TaskInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if len(h.tasks) < h.maxSize {
		h.tasks = append(h.tasks, task)
	} else {
		h.tasks[h.index] = task
		h.index = (h.index + 1) % h.maxSize
	}
}

func (h *TaskHistory) List(limit, offset int) []TaskInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if offset >= len(h.tasks) {
		return []TaskInfo{}
	}
	
	end := offset + limit
	if end > len(h.tasks) {
		end = len(h.tasks)
	}
	
	// Return reverse chronological order (newest first)
	result := make([]TaskInfo, end-offset)
	for i := range result {
		result[i] = h.tasks[len(h.tasks)-1-offset-i]
	}
	return result
}

func (h *TaskHistory) Get(taskID string) *TaskInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	for i := len(h.tasks) - 1; i >= 0; i-- {
		if h.tasks[i].ID == taskID {
			return &h.tasks[i]
		}
	}
	return nil
}
```

### Files to Create

1. **orchestrator/internal/queue/task_info.go**
   - TaskInfo struct
   - TaskStatus enum
   - TaskHistory circular buffer

2. **orchestrator/internal/queue/task_info_test.go**
   - Unit tests for TaskHistory

3. **orchestrator/internal/webhooks/queue_status.go**
   - Handler implementations for all 4 endpoints

4. **orchestrator/internal/webhooks/queue_status_test.go**
   - Unit tests for handlers

### Files to Modify

1. **orchestrator/internal/queue/queue.go**
   - Add task tracking to Queue interface
   - Implement GetQueuedTasks(), GetProcessingTasks() methods

2. **orchestrator/internal/webhooks/server.go**
   - Add routes: GET /queue/status, /queue/processing, /queue/history, /tasks/:id
   - Pass TaskHistory to handlers

---

## Testing Strategy

### Unit Tests

**task_info_test.go:**
```go
func TestTaskHistory_Add(t *testing.T) {
	// Test adding tasks to history
}

func TestTaskHistory_Add_CircularBuffer(t *testing.T) {
	// Test overflow wraps around (circular buffer)
}

func TestTaskHistory_List(t *testing.T) {
	// Test listing with limit and offset
}

func TestTaskHistory_List_ReverseChronological(t *testing.T) {
	// Verify newest tasks first
}

func TestTaskHistory_Get_Found(t *testing.T) {
	// Test finding existing task
}

func TestTaskHistory_Get_NotFound(t *testing.T) {
	// Test task not in history
}
```

**queue_status_test.go:**
```go
func TestHandleQueueStatus(t *testing.T) {
	// Test queue status endpoint returns correct counts
}

func TestHandleQueueProcessing(t *testing.T) {
	// Test processing tasks endpoint
}

func TestHandleQueueHistory(t *testing.T) {
	// Test history endpoint with pagination
}

func TestHandleTaskStatus_Found(t *testing.T) {
	// Test individual task lookup (found)
}

func TestHandleTaskStatus_NotFound(t *testing.T) {
	// Test individual task lookup (not found)
}
```

### Integration Tests

```go
func TestQueueStatus_Integration(t *testing.T) {
	// Start server with mock queue
	// Queue several tasks
	// Start processing some tasks
	// Complete some tasks
	// Query all endpoints
	// Verify response data matches queue state
}
```

### Manual Testing

```bash
# Test 1: Queue status
curl http://localhost:9000/queue/status
# Expected: Current queue statistics

# Test 2: Processing tasks
curl http://localhost:9000/queue/processing
# Expected: List of active transcriptions

# Test 3: History
curl http://localhost:9000/queue/history
# Expected: Recent completions (up to 100)

# Test 4: History pagination
curl "http://localhost:9000/queue/history?limit=10&offset=20"
# Expected: 10 tasks starting at offset 20

# Test 5: Individual task status
curl http://localhost:9000/tasks/task-12345
# Expected: Task details

# Test 6: Task not found
curl http://localhost:9000/tasks/nonexistent
# Expected: 404 error
```

---

## Definition of Done

- [ ] Story file created (this document)
- [ ] Tests written FIRST (TDD)
- [ ] TaskInfo struct implemented
- [ ] TaskHistory circular buffer implemented
- [ ] Queue tracking methods implemented
- [ ] All 4 endpoints implemented
- [ ] Thread-safe access to shared state
- [ ] All unit tests passing
- [ ] Integration tests passing
- [ ] Manual testing completed
- [ ] Type checking passes
- [ ] Performance: <100ms response time
- [ ] Work log created (0025_2026-02-16_epic08_story07_progress_reporting.md)
- [ ] Code committed and pushed

---

## Performance Considerations

- **In-memory storage** - History limited to last 100 tasks (configurable)
- **Lock contention** - Use RWMutex for read-heavy workload
- **Response time** - Target <100ms for all endpoints
- **Memory usage** - ~10KB per task × 100 = ~1MB memory footprint

---

## Future Enhancements

- **WebSocket endpoint** - Real-time updates via websockets
- **Prometheus metrics** - Expose queue metrics for monitoring
- **Progress callbacks** - Worker reports progress during transcription
- **ETA calculation** - Better estimates based on file size and model
- **Persistent history** - Store in SQLite for long-term tracking

---

## Success Criteria

1. **Accuracy**: Status reflects actual queue state
2. **Performance**: All endpoints respond in <100ms
3. **Reliability**: No race conditions or data corruption
4. **Usability**: Clear JSON responses for dashboards
5. **Scalability**: Handle 1000+ tasks in history

---

## Use Cases

1. **Dashboard UI** - Display queue status and progress
2. **Monitoring** - Alert on high queue depth or failures
3. **Troubleshooting** - Investigate stuck or failed tasks
4. **User feedback** - "Your file is #15 in queue"
5. **API integration** - External tools can poll task status

---

## References

- **Original Implementation**: None (new feature)
- **Similar Systems**: Celery (Python task queue), Sidekiq (Ruby)
- **Go Patterns**: Circular buffer, RWMutex for concurrent access

---

**Story Created**: 2026-02-16  
**Last Updated**: 2026-02-16
