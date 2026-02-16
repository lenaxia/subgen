package util

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorWithHint_Error(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		hints        []string
		wantContains []string
		wantNotEmpty bool
	}{
		{
			name:  "error with single hint",
			err:   fmt.Errorf("transcription failed"),
			hints: []string{"Check file permissions"},
			wantContains: []string{
				"transcription failed",
				"Troubleshooting:",
				"Check file permissions",
			},
			wantNotEmpty: true,
		},
		{
			name: "error with multiple hints",
			err:  fmt.Errorf("audio extraction failed"),
			hints: []string{
				"Verify file has audio tracks",
				"Check file is not corrupted",
				"Try re-encoding the file",
			},
			wantContains: []string{
				"audio extraction failed",
				"Troubleshooting:",
				"Verify file has audio tracks",
				"Check file is not corrupted",
				"Try re-encoding the file",
			},
			wantNotEmpty: true,
		},
		{
			name:         "error with no hints",
			err:          fmt.Errorf("simple error"),
			hints:        []string{},
			wantContains: []string{"simple error"},
			wantNotEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errWithHint := &ErrorWithHint{
				Err:   tt.err,
				Hints: tt.hints,
			}

			result := errWithHint.Error()

			if tt.wantNotEmpty {
				assert.NotEmpty(t, result)
			}

			for _, want := range tt.wantContains {
				assert.Contains(t, result, want)
			}
		})
	}
}

func TestErrorWithHint_Unwrap(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	errWithHint := &ErrorWithHint{
		Err:   originalErr,
		Hints: []string{"hint 1"},
	}

	unwrapped := errWithHint.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestNewErrorWithHints(t *testing.T) {
	originalErr := fmt.Errorf("test error")
	hints := []string{"hint 1", "hint 2"}

	result := NewErrorWithHints(originalErr, hints...)

	errWithHint, ok := result.(*ErrorWithHint)
	assert.True(t, ok, "should return *ErrorWithHint")
	assert.Equal(t, originalErr, errWithHint.Err)
	assert.Equal(t, hints, errWithHint.Hints)
}

func TestAudioExtractionError(t *testing.T) {
	filePath := "/path/to/movie.mkv"
	originalErr := fmt.Errorf("no audio tracks")

	result := AudioExtractionError(filePath, originalErr)

	errStr := result.Error()
	assert.Contains(t, errStr, "audio extraction failed")
	assert.Contains(t, errStr, "no audio tracks")
	assert.Contains(t, errStr, "ffprobe")
	assert.Contains(t, errStr, filePath)
	assert.Contains(t, errStr, "corrupted")
	assert.Contains(t, errStr, "ffmpeg")
}

func TestWorkerConnectionError(t *testing.T) {
	address := "worker-1:50051"
	originalErr := fmt.Errorf("connection refused")

	result := WorkerConnectionError(address, originalErr)

	errStr := result.Error()
	assert.Contains(t, errStr, "failed to connect to worker")
	assert.Contains(t, errStr, address)
	assert.Contains(t, errStr, "connection refused")
	assert.Contains(t, errStr, "worker is running")
	assert.Contains(t, errStr, "network connectivity")
	assert.Contains(t, errStr, "firewall")
}

func TestPathMappingError(t *testing.T) {
	originalPath := "/data/movies"
	mappedPath := "/mnt/media/movies"

	result := PathMappingError(originalPath, mappedPath)

	errStr := result.Error()
	assert.Contains(t, errStr, "mapped path does not exist")
	assert.Contains(t, errStr, originalPath)
	assert.Contains(t, errStr, mappedPath)
	assert.Contains(t, errStr, "PATH_MAPPING_FROM")
	assert.Contains(t, errStr, "PATH_MAPPING_TO")
	assert.Contains(t, errStr, "volume mounts")
}

func TestFileNotFoundError(t *testing.T) {
	filePath := "/missing/file.mkv"

	result := FileNotFoundError(filePath)

	errStr := result.Error()
	assert.Contains(t, errStr, "file not found")
	assert.Contains(t, errStr, filePath)
	assert.Contains(t, errStr, "file path is correct")
	assert.Contains(t, errStr, "read access")
	assert.Contains(t, errStr, "path mapping")
}

func TestWorkerTimeoutError(t *testing.T) {
	filePath := "/path/to/long_video.mkv"
	duration := "30m"

	result := WorkerTimeoutError(filePath, duration)

	errStr := result.Error()
	assert.Contains(t, errStr, "worker timeout")
	assert.Contains(t, errStr, filePath)
	assert.Contains(t, errStr, duration)
	assert.Contains(t, errStr, "worker is running")
	assert.Contains(t, errStr, "Increase timeout")
	assert.Contains(t, errStr, "GPU/CPU usage")
}

func TestQueueFullError(t *testing.T) {
	maxSize := 100

	result := QueueFullError(maxSize)

	errStr := result.Error()
	assert.Contains(t, errStr, "queue is full")
	assert.Contains(t, errStr, "100")
	assert.Contains(t, errStr, "QUEUE_MAX_SIZE")
	assert.Contains(t, errStr, "workers")
}

func TestInvalidLanguageCodeError(t *testing.T) {
	code := "xyz"

	result := InvalidLanguageCodeError(code)

	errStr := result.Error()
	assert.Contains(t, errStr, "invalid language code")
	assert.Contains(t, errStr, code)
	assert.Contains(t, errStr, "ISO 639")
	assert.Contains(t, errStr, "eng")
	assert.Contains(t, errStr, "spa")
}

func TestErrorWithHint_FormattingPreservesStructure(t *testing.T) {
	err := NewErrorWithHints(
		fmt.Errorf("test error"),
		"Hint 1",
		"Hint 2",
		"Hint 3",
	)

	errStr := err.Error()

	// Check structure
	assert.Contains(t, errStr, "test error")
	assert.Contains(t, errStr, "\nTroubleshooting:")

	// Check all hints are on separate lines with proper formatting
	lines := strings.Split(errStr, "\n")
	var hintLines []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "-") {
			hintLines = append(hintLines, line)
		}
	}

	assert.Len(t, hintLines, 3, "should have 3 hint lines")
}
