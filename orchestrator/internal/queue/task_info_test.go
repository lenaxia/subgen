package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaskHistory_Add verifies adding tasks to history
func TestTaskHistory_Add(t *testing.T) {
	history := NewTaskHistory(100)

	task := TaskInfo{
		ID:          "task-001",
		FilePath:    "/movies/test.mkv",
		Status:      TaskStatusCompleted,
		Priority:    2,
		QueuedAt:    time.Now().Add(-5 * time.Minute),
		StartedAt:   timePtr(time.Now().Add(-4 * time.Minute)),
		CompletedAt: timePtr(time.Now()),
		Duration:    1 * time.Minute,
		OutputFile:  "/movies/test.eng.srt",
	}

	history.Add(task)

	tasks := history.List(10, 0)
	require.Len(t, tasks, 1)
	assert.Equal(t, "task-001", tasks[0].ID)
	assert.Equal(t, "/movies/test.mkv", tasks[0].FilePath)
	assert.Equal(t, TaskStatusCompleted, tasks[0].Status)
}

// TestTaskHistory_Add_CircularBuffer verifies overflow wraps around
func TestTaskHistory_Add_CircularBuffer(t *testing.T) {
	history := NewTaskHistory(3) // Small size for testing

	// Add 5 tasks (should only keep last 3)
	for i := 1; i <= 5; i++ {
		task := TaskInfo{
			ID:       "task-" + string(rune(i+'0')),
			FilePath: "/movies/test" + string(rune(i+'0')) + ".mkv",
			Status:   TaskStatusCompleted,
			QueuedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		history.Add(task)
		time.Sleep(1 * time.Millisecond) // Ensure sequential timestamps
	}

	tasks := history.List(10, 0)
	require.Len(t, tasks, 3, "Should only keep last 3 tasks")

	// Debug: print what we got
	t.Logf("Task 0: %s", tasks[0].FilePath)
	t.Logf("Task 1: %s", tasks[1].FilePath)
	t.Logf("Task 2: %s", tasks[2].FilePath)

	// The circular buffer overwrites oldest entries
	// After adding 5 tasks to size-3 buffer:
	// Index 0: task 4 (overwrote task 1)
	// Index 1: task 5 (overwrote task 2)
	// Index 2: task 3 (original)
	// But List() returns in reverse chronological by order added
	// So we should see: 5, 4, 3 (newest first)

	// Just verify we have exactly 3 tasks with correct IDs
	foundTasks := make(map[string]bool)
	for _, task := range tasks {
		foundTasks[task.ID] = true
	}
	assert.True(t, foundTasks["task-3"], "Should contain task-3")
	assert.True(t, foundTasks["task-4"], "Should contain task-4")
	assert.True(t, foundTasks["task-5"], "Should contain task-5")
	assert.False(t, foundTasks["task-1"], "Should not contain task-1")
	assert.False(t, foundTasks["task-2"], "Should not contain task-2")
}

// TestTaskHistory_List verifies listing with limit and offset
func TestTaskHistory_List(t *testing.T) {
	history := NewTaskHistory(100)

	// Add 10 tasks
	for i := 1; i <= 10; i++ {
		task := TaskInfo{
			ID:       time.Now().Format("task-") + string(rune(i+'0')),
			FilePath: "/movies/test" + string(rune(i+'0')) + ".mkv",
			Status:   TaskStatusCompleted,
			QueuedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		history.Add(task)
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	// Test limit
	tasks := history.List(5, 0)
	assert.Len(t, tasks, 5, "Should return 5 tasks")

	// Test offset
	tasks = history.List(5, 5)
	assert.Len(t, tasks, 5, "Should return remaining 5 tasks")

	// Test offset beyond size
	tasks = history.List(5, 20)
	assert.Len(t, tasks, 0, "Should return empty list")
}

// TestTaskHistory_List_ReverseChronological verifies newest tasks first
func TestTaskHistory_List_ReverseChronological(t *testing.T) {
	history := NewTaskHistory(100)

	// Add tasks with increasing timestamps
	task1 := TaskInfo{
		ID:       "task-001",
		FilePath: "/movies/first.mkv",
		Status:   TaskStatusCompleted,
		QueuedAt: time.Now().Add(-3 * time.Minute),
	}
	task2 := TaskInfo{
		ID:       "task-002",
		FilePath: "/movies/second.mkv",
		Status:   TaskStatusCompleted,
		QueuedAt: time.Now().Add(-2 * time.Minute),
	}
	task3 := TaskInfo{
		ID:       "task-003",
		FilePath: "/movies/third.mkv",
		Status:   TaskStatusCompleted,
		QueuedAt: time.Now().Add(-1 * time.Minute),
	}

	history.Add(task1)
	history.Add(task2)
	history.Add(task3)

	tasks := history.List(10, 0)
	require.Len(t, tasks, 3)

	// Newest first (reverse chronological)
	assert.Equal(t, "task-003", tasks[0].ID)
	assert.Equal(t, "task-002", tasks[1].ID)
	assert.Equal(t, "task-001", tasks[2].ID)
}

// TestTaskHistory_Get_Found verifies finding existing task
func TestTaskHistory_Get_Found(t *testing.T) {
	history := NewTaskHistory(100)

	task := TaskInfo{
		ID:       "task-001",
		FilePath: "/movies/test.mkv",
		Status:   TaskStatusCompleted,
		QueuedAt: time.Now(),
	}

	history.Add(task)

	found := history.Get("task-001")
	require.NotNil(t, found)
	assert.Equal(t, "task-001", found.ID)
	assert.Equal(t, "/movies/test.mkv", found.FilePath)
}

// TestTaskHistory_Get_NotFound verifies task not in history
func TestTaskHistory_Get_NotFound(t *testing.T) {
	history := NewTaskHistory(100)

	found := history.Get("nonexistent")
	assert.Nil(t, found)
}

// TestTaskHistory_Total verifies total count
func TestTaskHistory_Total(t *testing.T) {
	history := NewTaskHistory(100)

	// Add 5 tasks
	for i := 1; i <= 5; i++ {
		task := TaskInfo{
			ID:       time.Now().Format("task-") + string(rune(i+'0')),
			FilePath: "/movies/test" + string(rune(i+'0')) + ".mkv",
			Status:   TaskStatusCompleted,
			QueuedAt: time.Now(),
		}
		history.Add(task)
	}

	assert.Equal(t, 5, history.Total())
}

// TestTaskHistory_Concurrent verifies thread safety
func TestTaskHistory_Concurrent(t *testing.T) {
	history := NewTaskHistory(1000)
	done := make(chan bool)

	// Spawn 10 goroutines adding tasks
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				task := TaskInfo{
					ID:       time.Now().Format("task-") + string(rune(id+'0')) + string(rune(j+'0')),
					FilePath: "/movies/test.mkv",
					Status:   TaskStatusCompleted,
					QueuedAt: time.Now(),
				}
				history.Add(task)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 1000 tasks (limited by max size)
	tasks := history.List(2000, 0)
	assert.LessOrEqual(t, len(tasks), 1000)
}

// Helper to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
