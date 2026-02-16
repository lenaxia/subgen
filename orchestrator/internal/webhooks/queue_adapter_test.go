package webhooks

import (
	"testing"

	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueueAdapter_Enqueue(t *testing.T) {
	// Create test queue
	registry := prometheus.NewRegistry()
	metrics := queue.NewQueueMetricsWithRegistry(registry)
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	q := queue.NewQueue(100, metrics, log)
	adapter := NewQueueAdapter(q)

	// Create webhook task
	task := Task{
		FilePath:          "/media/movie.mkv",
		TranscriptionType: "transcribe",
		ForceLanguage:     "en",
		PlexItemID:        "12345",
		PlexServer:        "http://plex:32400",
		PlexToken:         "abc123",
	}

	// Enqueue task
	err := adapter.Enqueue(task)

	require.NoError(t, err)
	assert.Equal(t, 1, q.Size())

	// Dequeue and verify fields were mapped correctly
	dequeuedTask, _ := q.Dequeue()
	assert.Equal(t, task.FilePath, dequeuedTask.FilePath)
	assert.Equal(t, task.TranscriptionType, dequeuedTask.TaskType)
	assert.Equal(t, task.ForceLanguage, dequeuedTask.ForceLanguage)
	assert.Equal(t, task.PlexItemID, dequeuedTask.PlexItemID)
	assert.Equal(t, task.PlexServer, dequeuedTask.PlexServer)
	assert.Equal(t, task.PlexToken, dequeuedTask.PlexToken)
}

func TestQueueAdapter_Enqueue_Duplicate(t *testing.T) {
	// Create test queue
	registry := prometheus.NewRegistry()
	metrics := queue.NewQueueMetricsWithRegistry(registry)
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	q := queue.NewQueue(100, metrics, log)
	adapter := NewQueueAdapter(q)

	// Create webhook task
	task := Task{
		FilePath:          "/media/movie.mkv",
		TranscriptionType: "transcribe",
	}

	// Enqueue twice
	err1 := adapter.Enqueue(task)
	err2 := adapter.Enqueue(task)

	require.NoError(t, err1)
	assert.ErrorIs(t, err2, queue.ErrDuplicateTask)
	assert.Equal(t, 1, q.Size())
}

func TestQueueAdapter_Enqueue_QueueFull(t *testing.T) {
	// Create test queue with size 1
	registry := prometheus.NewRegistry()
	metrics := queue.NewQueueMetricsWithRegistry(registry)
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	q := queue.NewQueue(1, metrics, log)
	adapter := NewQueueAdapter(q)

	// Create webhook tasks
	task1 := Task{FilePath: "/media/movie1.mkv"}
	task2 := Task{FilePath: "/media/movie2.mkv"}

	// Fill queue
	err1 := adapter.Enqueue(task1)
	err2 := adapter.Enqueue(task2)

	require.NoError(t, err1)
	assert.ErrorIs(t, err2, queue.ErrQueueFull)
	assert.Equal(t, 1, q.Size())
}

func TestQueueAdapter_Enqueue_JellyfinMetadata(t *testing.T) {
	// Create test queue
	registry := prometheus.NewRegistry()
	metrics := queue.NewQueueMetricsWithRegistry(registry)
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	q := queue.NewQueue(100, metrics, log)
	adapter := NewQueueAdapter(q)

	// Create webhook task with Jellyfin metadata
	task := Task{
		FilePath:       "/media/movie.mkv",
		JellyfinItemID: "jellyfin-123",
		JellyfinServer: "http://jellyfin:8096",
		JellyfinToken:  "jelly-token",
	}

	// Enqueue task
	err := adapter.Enqueue(task)

	require.NoError(t, err)

	// Dequeue and verify Jellyfin fields
	dequeuedTask, _ := q.Dequeue()
	assert.Equal(t, task.JellyfinItemID, dequeuedTask.JellyfinItemID)
	assert.Equal(t, task.JellyfinServer, dequeuedTask.JellyfinServer)
	assert.Equal(t, task.JellyfinToken, dequeuedTask.JellyfinToken)
}
