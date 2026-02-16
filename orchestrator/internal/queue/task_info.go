package queue

import (
	"sync"
	"time"
)

// TaskStatus represents the current status of a task
type TaskStatus string

const (
	TaskStatusQueued     TaskStatus = "queued"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
)

// TaskInfo contains detailed information about a task for status reporting
type TaskInfo struct {
	ID          string
	FilePath    string
	Status      TaskStatus
	Priority    int
	QueuedAt    time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Duration    time.Duration
	OutputFile  string
	Error       string
	Progress    int // 0-100 (future: worker reports progress)
	ETASeconds  int // Estimated time remaining (future: based on file size)
	WorkerID    int
}

// TaskHistory stores recent task completions in a circular buffer
type TaskHistory struct {
	mu      sync.RWMutex
	tasks   []TaskInfo
	maxSize int
	index   int
}

// NewTaskHistory creates a new task history with specified max size
func NewTaskHistory(maxSize int) *TaskHistory {
	return &TaskHistory{
		tasks:   make([]TaskInfo, 0, maxSize),
		maxSize: maxSize,
	}
}

// Add adds a task to history (circular buffer, oldest tasks are overwritten)
func (h *TaskHistory) Add(task TaskInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.tasks) < h.maxSize {
		// Still growing, just append
		h.tasks = append(h.tasks, task)
	} else {
		// At capacity, overwrite oldest
		h.tasks[h.index] = task
		h.index = (h.index + 1) % h.maxSize
	}
}

// List returns tasks in reverse chronological order (newest first)
// with pagination support via limit and offset
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

// Get retrieves a specific task by ID
func (h *TaskHistory) Get(taskID string) *TaskInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Search from newest to oldest
	for i := len(h.tasks) - 1; i >= 0; i-- {
		if h.tasks[i].ID == taskID {
			return &h.tasks[i]
		}
	}
	return nil
}

// Total returns total number of tasks in history
func (h *TaskHistory) Total() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.tasks)
}
