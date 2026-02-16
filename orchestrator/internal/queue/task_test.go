package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTask(t *testing.T) {
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)

	assert.NotNil(t, task)
	assert.Equal(t, "/media/movie.mkv", task.FilePath)
	assert.Equal(t, TaskTypeTranscribe, task.Type)
	assert.Equal(t, PriorityTranscribe, task.Priority)
	assert.Equal(t, "transcribe", task.TaskType)
	assert.NotEmpty(t, task.ID)
	assert.False(t, task.QueuedAt.IsZero())
}

func TestComputeID_SamePathSameID(t *testing.T) {
	task1 := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie.mkv", TaskTypeASR)

	// Same file path should produce same ID regardless of task type
	assert.Equal(t, task1.ID, task2.ID)
}

func TestComputeID_DifferentPathDifferentID(t *testing.T) {
	task1 := NewTask("/media/movie1.mkv", TaskTypeTranscribe)
	task2 := NewTask("/media/movie2.mkv", TaskTypeTranscribe)

	assert.NotEqual(t, task1.ID, task2.ID)
}

func TestTaskTypeToPriority(t *testing.T) {
	tests := []struct {
		name     string
		taskType TaskType
		expected Priority
	}{
		{"detect language", TaskTypeDetectLanguage, PriorityDetectLanguage},
		{"ASR", TaskTypeASR, PriorityASR},
		{"transcribe", TaskTypeTranscribe, PriorityTranscribe},
		{"unknown defaults to transcribe", TaskType("unknown"), PriorityTranscribe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority := TaskTypeToPriority(tt.taskType)
			assert.Equal(t, tt.expected, priority)
		})
	}
}

func TestWaitTime_NotStarted(t *testing.T) {
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	time.Sleep(10 * time.Millisecond)

	waitTime := task.WaitTime()

	assert.True(t, waitTime >= 10*time.Millisecond)
	assert.True(t, waitTime < 100*time.Millisecond)
}

func TestWaitTime_Started(t *testing.T) {
	task := NewTask("/media/movie.mkv", TaskTypeTranscribe)
	time.Sleep(10 * time.Millisecond)
	task.StartedAt = time.Now()

	waitTime := task.WaitTime()

	// Wait time should be frozen when task started
	assert.True(t, waitTime >= 10*time.Millisecond)
	assert.True(t, waitTime < 100*time.Millisecond)
}
