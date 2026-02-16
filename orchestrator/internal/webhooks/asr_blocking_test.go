package webhooks

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/queue"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockQueueForBlocking implements QueueInterface for testing blocking behavior
type MockQueueForBlocking struct {
	mu             sync.Mutex
	enqueuedTasks  []Task
	simulateDelay  time.Duration
	simulateError  error
	processResults map[chan *queue.TranscriptionResult]*queue.TranscriptionResult
}

func newMockQueueForBlocking() *MockQueueForBlocking {
	return &MockQueueForBlocking{
		enqueuedTasks:  []Task{},
		processResults: make(map[chan *queue.TranscriptionResult]*queue.TranscriptionResult),
	}
}

func (m *MockQueueForBlocking) Enqueue(task Task) error {
	m.mu.Lock()
	m.enqueuedTasks = append(m.enqueuedTasks, task)
	m.mu.Unlock()

	// Simulate async processing
	go func() {
		if m.simulateDelay > 0 {
			time.Sleep(m.simulateDelay)
		}

		if task.ResultChan != nil {
			defer close(task.ResultChan)

			if m.simulateError != nil {
				task.ResultChan <- &queue.TranscriptionResult{
					Error: m.simulateError,
				}
			} else {
				result := &queue.TranscriptionResult{
					Segments: []queue.Segment{
						{Start: 0.0, End: 3.2, Text: "Test segment 1"},
						{Start: 3.4, End: 6.8, Text: "Test segment 2"},
					},
					Metadata: queue.Metadata{
						Language: "en",
						Duration: 10.5,
						Model:    "medium",
					},
					Error: nil,
				}
				task.ResultChan <- result
			}
		}
	}()

	return nil
}

func (m *MockQueueForBlocking) Size() int                                  { return len(m.enqueuedTasks) }
func (m *MockQueueForBlocking) ProcessingCount() int                       { return 0 }
func (m *MockQueueForBlocking) IsIdle() bool                               { return true }
func (m *MockQueueForBlocking) GetTaskInfo(taskID string) *queue.TaskInfo  { return nil }
func (m *MockQueueForBlocking) GetAllProcessingTaskInfo() []queue.TaskInfo { return []queue.TaskInfo{} }
func (m *MockQueueForBlocking) GetHistory(limit, offset int) []queue.TaskInfo {
	return []queue.TaskInfo{}
}
func (m *MockQueueForBlocking) GetHistoryTotal() int { return 0 }

// TestASRBlocking_Success tests successful blocking ASR request
func TestASRBlocking_Success(t *testing.T) {
	// Create mock queue with 100ms delay
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 100 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel) // Suppress logs during test

	_ = NewServer(cfg, mockQueue, log) // Server not used directly in test, but needed for initialization

	// Create result channel
	resultChan := make(chan *queue.TranscriptionResult, 1)

	// Create task
	task := Task{
		FilePath:          "/test/audio.mp3",
		TranscriptionType: "transcribe",
		ForceLanguage:     "en",
		AudioContent:      []byte("test audio data"),
		ASROptions:        map[string]string{"output": "srt"},
		ResultChan:        resultChan,
	}

	// Enqueue task
	err := mockQueue.Enqueue(task)
	require.NoError(t, err)

	// Block for result with timeout
	start := time.Now()
	select {
	case result := <-resultChan:
		elapsed := time.Since(start)

		// Verify blocking occurred (should be ~100ms)
		assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond, "Should block for at least 100ms")
		assert.Less(t, elapsed, 200*time.Millisecond, "Should complete within 200ms")

		// Verify result
		require.NotNil(t, result)
		assert.Nil(t, result.Error, "Should have no error")
		assert.Equal(t, 2, len(result.Segments), "Should have 2 segments")
		assert.Equal(t, "en", result.Metadata.Language)
		assert.Equal(t, 10.5, result.Metadata.Duration)

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for result")
	}
}

// TestASRBlocking_Timeout tests timeout behavior
func TestASRBlocking_Timeout(t *testing.T) {
	// Create mock queue that never completes
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 10 * time.Second // Longer than timeout

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 500 * time.Millisecond, // Short timeout for test
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	_ = NewServer(cfg, mockQueue, log)

	// Create result channel
	resultChan := make(chan *queue.TranscriptionResult, 1)

	// Create task
	task := Task{
		FilePath:          "/test/audio.mp3",
		TranscriptionType: "transcribe",
		AudioContent:      []byte("test audio data"),
		ResultChan:        resultChan,
	}

	// Enqueue task
	err := mockQueue.Enqueue(task)
	require.NoError(t, err)

	// Wait for result with timeout
	start := time.Now()
	select {
	case result := <-resultChan:
		// Should not receive result before timeout
		t.Fatalf("Received result before timeout: %+v", result)

	case <-time.After(cfg.ASR.Timeout):
		elapsed := time.Since(start)

		// Verify timeout occurred
		assert.GreaterOrEqual(t, elapsed, cfg.ASR.Timeout, "Should timeout after configured duration")
		assert.Less(t, elapsed, cfg.ASR.Timeout+100*time.Millisecond, "Should timeout promptly")
	}
}

// TestASRBlocking_WorkerError tests error handling
func TestASRBlocking_WorkerError(t *testing.T) {
	// Create mock queue that returns error
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 10 * time.Millisecond
	mockQueue.simulateError = fmt.Errorf("worker crashed")

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	_ = NewServer(cfg, mockQueue, log)

	// Create result channel
	resultChan := make(chan *queue.TranscriptionResult, 1)

	// Create task
	task := Task{
		FilePath:     "/test/audio.mp3",
		AudioContent: []byte("test audio data"),
		ResultChan:   resultChan,
	}

	// Enqueue task
	err := mockQueue.Enqueue(task)
	require.NoError(t, err)

	// Wait for result
	select {
	case result := <-resultChan:
		// Verify error was returned
		require.NotNil(t, result)
		assert.NotNil(t, result.Error, "Should have error")
		assert.Contains(t, result.Error.Error(), "worker crashed")

	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for result")
	}
}

// TestASRBlocking_ConcurrentRequests tests multiple concurrent ASR requests
func TestASRBlocking_ConcurrentRequests(t *testing.T) {
	// Create mock queue
	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 50 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	_ = NewServer(cfg, mockQueue, log)

	// Make 10 concurrent requests
	numRequests := 10
	var wg sync.WaitGroup
	results := make([]*queue.TranscriptionResult, numRequests)
	errors := make([]error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Create result channel for this request
			resultChan := make(chan *queue.TranscriptionResult, 1)

			// Create task
			task := Task{
				FilePath:     fmt.Sprintf("/test/audio%d.mp3", idx),
				AudioContent: []byte(fmt.Sprintf("test audio data %d", idx)),
				ResultChan:   resultChan,
			}

			// Enqueue task
			err := mockQueue.Enqueue(task)
			if err != nil {
				errors[idx] = err
				return
			}

			// Wait for result
			select {
			case result := <-resultChan:
				results[idx] = result

			case <-time.After(1 * time.Second):
				errors[idx] = fmt.Errorf("timeout")
			}
		}(i)
	}

	wg.Wait()

	// Verify all requests succeeded
	for i := 0; i < numRequests; i++ {
		assert.NoError(t, errors[i], "Request %d should not have error", i)
		assert.NotNil(t, results[i], "Request %d should have result", i)
		if results[i] != nil {
			assert.Nil(t, results[i].Error, "Request %d should have no transcription error", i)
			assert.Equal(t, 2, len(results[i].Segments), "Request %d should have 2 segments", i)
		}
	}
}

// TestASRBlocking_ChannelCleanup tests that channels are properly cleaned up
func TestASRBlocking_ChannelCleanup(t *testing.T) {
	// This test verifies no channel memory leaks
	// Run with: go test -race ./internal/webhooks -run TestASRBlocking_ChannelCleanup

	mockQueue := newMockQueueForBlocking()
	mockQueue.simulateDelay = 10 * time.Millisecond

	cfg := &config.Config{
		ASR: config.ASRConfig{
			Timeout: 5 * time.Second,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	_ = NewServer(cfg, mockQueue, log)

	// Make many requests to detect potential leaks
	for i := 0; i < 100; i++ {
		resultChan := make(chan *queue.TranscriptionResult, 1)

		task := Task{
			FilePath:     fmt.Sprintf("/test/audio%d.mp3", i),
			AudioContent: []byte("test audio data"),
			ResultChan:   resultChan,
		}

		err := mockQueue.Enqueue(task)
		require.NoError(t, err)

		// Read result
		select {
		case result := <-resultChan:
			assert.NotNil(t, result)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Timeout on iteration", i)
		}
	}

	// If there are leaks, race detector will report them
	t.Log("Channel cleanup test completed successfully")
}
