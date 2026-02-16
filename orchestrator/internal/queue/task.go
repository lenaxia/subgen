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

// Segment represents a single subtitle segment
type Segment struct {
	Start float64 // Start time in seconds
	End   float64 // End time in seconds
	Text  string  // Subtitle text
}

// Metadata holds transcription metadata
type Metadata struct {
	Language string  // Detected language code (e.g., "en")
	Duration float64 // Audio duration in seconds
	Model    string  // Whisper model used (e.g., "medium")
}

// TranscriptionResult holds the result of a completed transcription
type TranscriptionResult struct {
	Segments []Segment // Transcription segments
	Metadata Metadata  // Language, duration, model
	Error    error     // Error if transcription failed
}

// Task represents a transcription task
type Task struct {
	// Identification
	ID       string   // SHA256 hash of FilePath
	FilePath string   // Absolute path to media file
	Type     TaskType // Type of task
	Priority Priority // Priority level

	// Transcription Options
	TaskType      string // "transcribe" or "translate"
	ForceLanguage string // ISO 639-1 code or empty

	// Media Server Metadata (for refresh after completion)
	PlexItemID     string // Plex rating key
	PlexServer     string // Plex server URL
	PlexToken      string // Plex auth token
	JellyfinItemID string // Jellyfin item ID
	JellyfinServer string // Jellyfin server URL
	JellyfinToken  string // Jellyfin auth token

	// ASR-specific fields
	AudioContent []byte            // For ASR tasks (Bazarr upload)
	ASROptions   map[string]string // ASR query parameters

	// Blocking operations (ASR, detect language)
	// When set, worker sends result to this channel instead of writing file
	// Channel is buffered (size 1) to prevent worker blocking
	ResultChan chan *TranscriptionResult

	// Timing
	QueuedAt    time.Time // When task was enqueued
	StartedAt   time.Time // When task started processing (zero if not started)
	CompletedAt time.Time // When task completed (zero if not completed)
}

// NewTask creates a new task with computed ID
func NewTask(filePath string, taskType TaskType) *Task {
	t := &Task{
		FilePath: filePath,
		Type:     taskType,
		Priority: TaskTypeToPriority(taskType),
		QueuedAt: time.Now(),
		TaskType: "transcribe", // Default
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
