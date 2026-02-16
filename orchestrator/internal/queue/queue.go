package queue

import (
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	ErrQueueFull     = errors.New("queue is full")
	ErrDuplicateTask = errors.New("task already queued or processing")
	ErrQueueEmpty    = errors.New("queue is empty")
	ErrTaskNotFound  = errors.New("task not found")
)

// Queue is a thread-safe bounded priority queue with deduplication
type Queue struct {
	mu sync.RWMutex

	// Priority heap for tasks
	heap *taskHeap

	// Deduplication tracking (task ID → task pointer)
	queued     map[string]*Task // Tasks waiting in queue
	processing map[string]*Task // Tasks currently being processed

	// Task history for completed/failed tasks
	history *TaskHistory

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
		history:    NewTaskHistory(100), // Keep last 100 completed/failed tasks
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

	// Add to history
	taskInfo := q.taskToTaskInfo(task, TaskStatusCompleted, "")
	q.history.Add(taskInfo)

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

	task.CompletedAt = time.Now()
	delete(q.processing, taskID)

	// Add to history with error
	taskInfo := q.taskToTaskInfo(task, TaskStatusFailed, err.Error())
	q.history.Add(taskInfo)

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

// CleanupStaleTasks removes tasks that have been processing for longer than timeout
// Returns the number of tasks cleaned up
func (q *Queue) CleanupStaleTasks(timeout time.Duration) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for taskID, task := range q.processing {
		if !task.StartedAt.IsZero() && now.Sub(task.StartedAt) > timeout {
			q.log.WithFields(logrus.Fields{
				"task_id":         taskID,
				"file_path":       task.FilePath,
				"processing_time": now.Sub(task.StartedAt),
				"timeout":         timeout,
			}).Warn("Cleaning up stale task")

			delete(q.processing, taskID)
			q.metrics.ProcessingSize.Set(float64(len(q.processing)))
			q.metrics.TasksFailed.WithLabelValues(string(task.Type)).Inc()
			cleaned++
		}
	}

	return cleaned
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

// GetTaskInfo retrieves detailed information about a task (queued, processing, or from history)
func (q *Queue) GetTaskInfo(taskID string) *TaskInfo {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Check if task is queued
	if task, exists := q.queued[taskID]; exists {
		info := q.taskToTaskInfo(task, TaskStatusQueued, "")
		return &info
	}

	// Check if task is processing
	if task, exists := q.processing[taskID]; exists {
		info := q.taskToTaskInfo(task, TaskStatusProcessing, "")
		return &info
	}

	// Check history
	return q.history.Get(taskID)
}

// GetAllProcessingTaskInfo returns detailed information about all currently processing tasks
func (q *Queue) GetAllProcessingTaskInfo() []TaskInfo {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]TaskInfo, 0, len(q.processing))
	for _, task := range q.processing {
		info := q.taskToTaskInfo(task, TaskStatusProcessing, "")
		result = append(result, info)
	}
	return result
}

// GetHistory returns task history with pagination
func (q *Queue) GetHistory(limit, offset int) []TaskInfo {
	// TaskHistory is already thread-safe
	return q.history.List(limit, offset)
}

// GetHistoryTotal returns total number of tasks in history
func (q *Queue) GetHistoryTotal() int {
	return q.history.Total()
}

// taskToTaskInfo converts a Task to TaskInfo with given status
// This is a helper method that must be called with lock held
func (q *Queue) taskToTaskInfo(task *Task, status TaskStatus, errorMsg string) TaskInfo {
	info := TaskInfo{
		ID:       task.ID,
		FilePath: task.FilePath,
		Status:   status,
		Priority: int(task.Priority),
		QueuedAt: task.QueuedAt,
		Error:    errorMsg,
	}

	if !task.StartedAt.IsZero() {
		info.StartedAt = &task.StartedAt
	}

	if !task.CompletedAt.IsZero() {
		info.CompletedAt = &task.CompletedAt
		info.Duration = task.CompletedAt.Sub(task.StartedAt)
	}

	// TODO: Add OutputFile, Progress, ETASeconds, WorkerID when available
	
	return info
}
