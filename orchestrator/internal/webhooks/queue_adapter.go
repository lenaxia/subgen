package webhooks

import (
	"github.com/mccloud/subgen/orchestrator/internal/queue"
)

// QueueAdapter adapts queue.Queue to the QueueInterface expected by webhooks
type QueueAdapter struct {
	queue *queue.Queue
}

// NewQueueAdapter creates a new queue adapter
func NewQueueAdapter(q *queue.Queue) *QueueAdapter {
	return &QueueAdapter{queue: q}
}

// Enqueue converts webhook Task to queue.Task and enqueues it
func (a *QueueAdapter) Enqueue(task Task) error {
	// Determine task type based on context
	taskType := queue.TaskTypeTranscribe
	if task.AudioContent != nil && len(task.AudioContent) > 0 {
		// ASR tasks have AudioContent
		taskType = queue.TaskTypeASR
	} else if task.ForceLanguage == "" && task.FilePath != "" {
		// No language specified, might need detection
		// For now, default to transcribe
		taskType = queue.TaskTypeTranscribe
	}

	// Convert webhook Task to queue.Task
	queueTask := queue.NewTask(task.FilePath, taskType)

	// Set transcription options
	queueTask.TaskType = task.TranscriptionType
	queueTask.ForceLanguage = task.ForceLanguage

	// Set Plex metadata
	queueTask.PlexItemID = task.PlexItemID
	queueTask.PlexServer = task.PlexServer
	queueTask.PlexToken = task.PlexToken

	// Set Jellyfin/Emby metadata
	queueTask.JellyfinItemID = task.JellyfinItemID
	queueTask.JellyfinServer = task.JellyfinServer
	queueTask.JellyfinToken = task.JellyfinToken

	// Set ASR-specific fields
	queueTask.AudioContent = task.AudioContent
	queueTask.ASROptions = task.ASROptions

	// Enqueue the task
	return a.queue.Enqueue(queueTask)
}
