package queue

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewQueue verifies queue initialization
func TestNewQueue(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewQueueMetricsWithRegistry(registry)
	log := logrus.New()
	q := NewQueue(100, metrics, log)

	assert.NotNil(t, q)
	assert.Equal(t, 0, q.Size())
	assert.True(t, q.IsIdle())
}

// TestEnqueue_Success verifies basic enqueue operation
func TestEnqueue_Success(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	err := q.Enqueue(task)

	require.NoError(t, err)
	assert.Equal(t, 1, q.Size())
	assert.False(t, q.IsIdle())
}

// TestEnqueue_Duplicate verifies deduplication for queued tasks
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

// TestEnqueue_DuplicateWhileProcessing verifies deduplication for processing tasks
func TestEnqueue_DuplicateWhileProcessing(t *testing.T) {
	q := newTestQueue(100)
	task1 := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	// Enqueue and dequeue task1 (now processing)
	_ = q.Enqueue(task1)
	_, _ = q.Dequeue()

	// Try to enqueue duplicate
	err := q.Enqueue(task2)

	assert.ErrorIs(t, err, ErrDuplicateTask)
	assert.Equal(t, 0, q.Size())
	assert.Equal(t, 1, q.ProcessingCount())
}

// TestEnqueue_QueueFull verifies bounded queue behavior
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

// TestDequeue_Success verifies basic dequeue operation
func TestDequeue_Success(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	_ = q.Enqueue(task)
	dequeuedTask, err := q.Dequeue()

	require.NoError(t, err)
	assert.Equal(t, task.ID, dequeuedTask.ID)
	assert.Equal(t, 0, q.Size())
	assert.Equal(t, 1, q.ProcessingCount())
	assert.False(t, dequeuedTask.StartedAt.IsZero())
}

// TestDequeue_EmptyQueue verifies empty queue error handling
func TestDequeue_EmptyQueue(t *testing.T) {
	q := newTestQueue(100)

	task, err := q.Dequeue()

	assert.ErrorIs(t, err, ErrQueueEmpty)
	assert.Nil(t, task)
}

// TestPriorityOrdering verifies tasks are dequeued by priority
func TestPriorityOrdering(t *testing.T) {
	q := newTestQueue(100)

	// Enqueue tasks in reverse priority order
	task1 := NewTask("/media/transcribe.mkv", TaskTypeTranscribe) // Priority 2
	task2 := NewTask("/media/asr.mp3", TaskTypeASR)               // Priority 1
	task3 := NewTask("/media/detect.mkv", TaskTypeDetectLanguage) // Priority 0

	_ = q.Enqueue(task1)
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	_ = q.Enqueue(task2)
	time.Sleep(1 * time.Millisecond)
	_ = q.Enqueue(task3)

	// Dequeue should return in priority order (0, 1, 2)
	t1, _ := q.Dequeue()
	t2, _ := q.Dequeue()
	t3, _ := q.Dequeue()

	assert.Equal(t, TaskTypeDetectLanguage, t1.Type)
	assert.Equal(t, TaskTypeASR, t2.Type)
	assert.Equal(t, TaskTypeTranscribe, t3.Type)
}

// TestFIFO_WithinSamePriority verifies FIFO ordering for same priority
func TestFIFO_WithinSamePriority(t *testing.T) {
	q := newTestQueue(100)

	// Enqueue 3 tasks with same priority
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)
	task3 := NewTask("/media/movie3.mkv", TaskTypeTranscribe)

	_ = q.Enqueue(task1)
	time.Sleep(1 * time.Millisecond)
	_ = q.Enqueue(task2)
	time.Sleep(1 * time.Millisecond)
	_ = q.Enqueue(task3)

	// Should dequeue in FIFO order
	t1, _ := q.Dequeue()
	t2, _ := q.Dequeue()
	t3, _ := q.Dequeue()

	assert.Equal(t, task1.ID, t1.ID)
	assert.Equal(t, task2.ID, t2.ID)
	assert.Equal(t, task3.ID, t3.ID)
}

// TestMarkDone verifies task completion tracking
func TestMarkDone(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	_ = q.Enqueue(task)
	dequeuedTask, _ := q.Dequeue()
	err := q.MarkDone(dequeuedTask.ID)

	require.NoError(t, err)
	assert.Equal(t, 0, q.ProcessingCount())
	assert.True(t, q.IsIdle())
	assert.False(t, dequeuedTask.CompletedAt.IsZero())
}

// TestMarkDone_TaskNotFound verifies error handling for unknown task
func TestMarkDone_TaskNotFound(t *testing.T) {
	q := newTestQueue(100)

	err := q.MarkDone("nonexistent-task-id")

	assert.ErrorIs(t, err, ErrTaskNotFound)
}

// TestMarkFailed verifies task failure tracking
func TestMarkFailed(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	_ = q.Enqueue(task)
	dequeuedTask, _ := q.Dequeue()
	err := q.MarkFailed(dequeuedTask.ID, errors.New("test error"))

	require.NoError(t, err)
	assert.Equal(t, 0, q.ProcessingCount())
	assert.True(t, q.IsIdle())
}

// TestMarkFailed_TaskNotFound verifies error handling for unknown task
func TestMarkFailed_TaskNotFound(t *testing.T) {
	q := newTestQueue(100)

	err := q.MarkFailed("nonexistent-task-id", errors.New("test error"))

	assert.ErrorIs(t, err, ErrTaskNotFound)
}

// TestIsActive verifies task status tracking
func TestIsActive(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	// Not active initially
	assert.False(t, q.IsActive(task.ID))

	// Active when queued
	_ = q.Enqueue(task)
	assert.True(t, q.IsActive(task.ID))

	// Active when processing
	_, _ = q.Dequeue()
	assert.True(t, q.IsActive(task.ID))

	// Not active when done
	_ = q.MarkDone(task.ID)
	assert.False(t, q.IsActive(task.ID))
}

// TestIsIdle verifies idle status tracking
func TestIsIdle(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	// Idle initially
	assert.True(t, q.IsIdle())

	// Not idle when queued
	_ = q.Enqueue(task)
	assert.False(t, q.IsIdle())

	// Not idle when processing
	_, _ = q.Dequeue()
	assert.False(t, q.IsIdle())

	// Idle when done
	_ = q.MarkDone(task.ID)
	assert.True(t, q.IsIdle())
}

// TestGetQueuedTasks verifies queued task listing
func TestGetQueuedTasks(t *testing.T) {
	q := newTestQueue(100)
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)

	_ = q.Enqueue(task1)
	_ = q.Enqueue(task2)

	queued := q.GetQueuedTasks()

	assert.Equal(t, 2, len(queued))
	assert.Contains(t, queued, task1.ID)
	assert.Contains(t, queued, task2.ID)
}

// TestGetProcessingTasks verifies processing task listing
func TestGetProcessingTasks(t *testing.T) {
	q := newTestQueue(100)
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)

	_ = q.Enqueue(task1)
	_ = q.Enqueue(task2)
	q.Dequeue() // Move task1 to processing

	processing := q.GetProcessingTasks()

	assert.Equal(t, 1, len(processing))
	assert.Contains(t, processing, task1.ID)
}

// TestConcurrentEnqueue verifies thread safety for concurrent enqueues
func TestConcurrentEnqueue(t *testing.T) {
	q := newTestQueue(1000)
	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			task := NewTask("/media/movie"+string(rune(idx))+".mkv", TaskTypeTranscribe)
			q.Enqueue(task)
		}(i)
	}

	wg.Wait()

	// Should have enqueued all tasks
	assert.Equal(t, numGoroutines, q.Size())
}

// TestConcurrentDequeue verifies thread safety for concurrent dequeues
func TestConcurrentDequeue(t *testing.T) {
	q := newTestQueue(1000)
	numTasks := 100

	// Enqueue tasks
	for i := 0; i < numTasks; i++ {
		task := NewTask("/media/movie"+string(rune(i))+".mkv", TaskTypeTranscribe)
		q.Enqueue(task)
	}

	// Dequeue concurrently
	var wg sync.WaitGroup
	wg.Add(numTasks)
	for i := 0; i < numTasks; i++ {
		go func() {
			defer wg.Done()
			q.Dequeue()
		}()
	}

	wg.Wait()

	// All tasks should be processing
	assert.Equal(t, 0, q.Size())
	assert.Equal(t, numTasks, q.ProcessingCount())
}

// TestReenqueueAfterDone verifies task can be re-enqueued after completion
func TestReenqueueAfterDone(t *testing.T) {
	q := newTestQueue(100)
	task1 := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	// First attempt
	_ = q.Enqueue(task1)
	t1, _ := q.Dequeue()
	_ = q.MarkDone(t1.ID)

	// Re-enqueue same file (new task instance)
	task2 := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	err := q.Enqueue(task2)

	require.NoError(t, err)
	assert.Equal(t, 1, q.Size())
}

// Helper function to create test queue
func newTestQueue(maxSize int) *Queue {
	// Use a new registry for each test to avoid conflicts
	registry := prometheus.NewRegistry()
	metrics := NewQueueMetricsWithRegistry(registry)
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Suppress logs in tests
	return NewQueue(maxSize, metrics, log)
}

// TestCleanupStaleTasks tests GAP #3: stale task detection and cleanup
func TestCleanupStaleTasks(t *testing.T) {
	q := newTestQueue(100)

	// Enqueue and dequeue 3 tasks
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)
	task3 := NewTask("/media/movie3.mkv", TaskTypeTranscribe)

	_ = q.Enqueue(task1)
	_ = q.Enqueue(task2)
	_ = q.Enqueue(task3)

	// Dequeue all tasks (move to processing)
	t1, _ := q.Dequeue()
	t2, _ := q.Dequeue()
	t3, _ := q.Dequeue()

	// Verify all in processing
	assert.Equal(t, 3, q.ProcessingCount())

	// Manually set StartedAt to simulate old tasks
	q.mu.Lock()
	q.processing[t1.ID].StartedAt = time.Now().Add(-2 * time.Hour)    // Stale (> 1 hour)
	q.processing[t2.ID].StartedAt = time.Now().Add(-90 * time.Minute) // Stale (> 1 hour)
	q.processing[t3.ID].StartedAt = time.Now().Add(-5 * time.Minute)  // Fresh (< 1 hour)
	q.mu.Unlock()

	// Cleanup with 1 hour timeout (should remove task1 and task2)
	cleaned := q.CleanupStaleTasks(1 * time.Hour)

	assert.Equal(t, 2, cleaned)
	assert.Equal(t, 1, q.ProcessingCount()) // Only task3 remains
}

// TestCleanupStaleTasks_NoStale tests cleanup when no tasks are stale
func TestCleanupStaleTasks_NoStale(t *testing.T) {
	q := newTestQueue(100)

	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	_ = q.Enqueue(task)
	_, _ = q.Dequeue()

	// Task just started, should not be cleaned
	cleaned := q.CleanupStaleTasks(1 * time.Hour)

	assert.Equal(t, 0, cleaned)
	assert.Equal(t, 1, q.ProcessingCount())
}

// TestCleanupStaleTasks_EmptyQueue tests cleanup on empty queue
func TestCleanupStaleTasks_EmptyQueue(t *testing.T) {
	q := newTestQueue(100)

	cleaned := q.CleanupStaleTasks(1 * time.Hour)

	assert.Equal(t, 0, cleaned)
	assert.Equal(t, 0, q.ProcessingCount())
}

// TestQueue_GetTaskInfo verifies retrieving task info
func TestQueue_GetTaskInfo(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	task.ID = "task-123"

	err := q.Enqueue(task)
	require.NoError(t, err)

	// Get task info while queued
	info := q.GetTaskInfo("task-123")
	require.NotNil(t, info)
	assert.Equal(t, "task-123", info.ID)
	assert.Equal(t, "/media/movie.mkv", info.FilePath)
	assert.Equal(t, TaskStatusQueued, info.Status)
	assert.Equal(t, 2, info.Priority) // TaskTypeTranscribe = priority 2

	// Dequeue and check processing status
	_, err = q.Dequeue()
	require.NoError(t, err)

	info = q.GetTaskInfo("task-123")
	require.NotNil(t, info)
	assert.Equal(t, TaskStatusProcessing, info.Status)
}

// TestQueue_GetAllProcessingTaskInfo verifies getting all processing tasks
func TestQueue_GetAllProcessingTaskInfo(t *testing.T) {
	q := newTestQueue(100)
	
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeASR)
	task3 := NewTask("/media/movie3.mkv", TaskTypeDetectLanguage)

	// Enqueue and dequeue all tasks (move to processing)
	_ = q.Enqueue(task1)
	_ = q.Enqueue(task2)
	_ = q.Enqueue(task3)

	_, _ = q.Dequeue()
	_, _ = q.Dequeue()
	_, _ = q.Dequeue()

	// Get all processing tasks
	processingTasks := q.GetAllProcessingTaskInfo()
	assert.Len(t, processingTasks, 3)

	// Verify all are in processing status
	for _, info := range processingTasks {
		assert.Equal(t, TaskStatusProcessing, info.Status)
	}
}

// TestQueue_MarkDone_AddsToHistory verifies completed tasks are added to history
func TestQueue_MarkDone_AddsToHistory(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	// Queue, dequeue, then mark done
	_ = q.Enqueue(task)
	dequeued, _ := q.Dequeue()
	
	err := q.MarkDone(dequeued.ID)
	require.NoError(t, err)

	// Task should now be in history
	info := q.GetTaskInfo(dequeued.ID)
	require.NotNil(t, info)
	assert.Equal(t, TaskStatusCompleted, info.Status)
	assert.NotNil(t, info.CompletedAt)
}

// TestQueue_MarkFailed_AddsToHistory verifies failed tasks are added to history
func TestQueue_MarkFailed_AddsToHistory(t *testing.T) {
	q := newTestQueue(100)
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	// Queue, dequeue, then mark failed
	_ = q.Enqueue(task)
	dequeued, _ := q.Dequeue()
	
	err := q.MarkFailed(dequeued.ID, errors.New("test error"))
	require.NoError(t, err)

	// Task should now be in history with error
	info := q.GetTaskInfo(dequeued.ID)
	require.NotNil(t, info)
	assert.Equal(t, TaskStatusFailed, info.Status)
	assert.Contains(t, info.Error, "test error")
}
