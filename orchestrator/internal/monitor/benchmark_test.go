package monitor_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/mccloud/subgen/orchestrator/internal/monitor"
	"github.com/sirupsen/logrus"
)

// BenchmarkScanner_10000Files measures scan performance on 10k files
func BenchmarkScanner_10000Files(b *testing.B) {
	// Create test directory with 10,000 files
	tmpDir, err := os.MkdirTemp("", "benchmark_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 10,000 media files
	b.Logf("Creating 10,000 test files...")
	for i := 0; i < 10000; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("file_%05d.mkv", i))
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	// Create scanner (no queue/skip checker for benchmark)
	cfg := &config.Config{
		Monitor: config.MonitorConfig{
			BatchScanLimit: 0,
		},
	}
	scanner := monitor.NewScanner(nil, nil, cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := scanner.ScanDirectory(tmpDir, true, "en")
		if err != nil {
			b.Fatal(err)
		}
		if result.Scanned != 10000 {
			b.Fatalf("Expected 10000 scanned, got %d", result.Scanned)
		}
	}
}

// BenchmarkScanner_1000Files measures scan performance on 1k files for quick testing
func BenchmarkScanner_1000Files(b *testing.B) {
	// Create test directory with 1,000 files
	tmpDir, err := os.MkdirTemp("", "benchmark_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 1,000 media files
	for i := 0; i < 1000; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("file_%04d.mkv", i))
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	// Create scanner
	cfg := &config.Config{
		Monitor: config.MonitorConfig{
			BatchScanLimit: 0,
		},
	}
	scanner := monitor.NewScanner(nil, nil, cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scanner.ScanDirectory(tmpDir, true, "en")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWatcher_100Directories measures watcher initialization time
func BenchmarkWatcher_100Directories(b *testing.B) {
	// Create 100 nested directories
	tmpDir, err := os.MkdirTemp("", "benchmark_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	for i := 0; i < 100; i++ {
		subDir := filepath.Join(tmpDir, fmt.Sprintf("dir_%03d", i))
		if err := os.MkdirAll(subDir, 0755); err != nil {
			b.Fatal(err)
		}
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	config := monitor.DefaultConfig()

	callback := func(path string) {}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fw, err := monitor.NewFileWatcher([]string{tmpDir}, callback, config, log)
		if err != nil {
			b.Fatal(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		go fw.Watch(ctx)
		time.Sleep(10 * time.Millisecond) // Brief wait for initialization
		cancel()
		time.Sleep(10 * time.Millisecond) // Brief wait for cleanup
	}
}

// BenchmarkStability_Check measures stability checking overhead
func BenchmarkStability_Check(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "benchmark_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.mkv")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		b.Fatal(err)
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	config := monitor.DefaultConfig()
	config.StabilityChecks = 3
	config.StabilityWait = 10 * time.Millisecond // Faster for benchmark

	fw, _ := monitor.NewFileWatcher([]string{tmpDir}, nil, config, log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fw.WaitForStability(testFile)
	}
}

// BenchmarkWatcher_FileEvents measures how fast the watcher can handle file events
func BenchmarkWatcher_FileEvents(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "benchmark_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Count callbacks
	callbackCount := 0
	callback := func(path string) {
		callbackCount++
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	config := monitor.DefaultConfig()
	config.StabilityChecks = 0 // Disable stability checking for benchmark

	fw, err := monitor.NewFileWatcher([]string{tmpDir}, callback, config, log)
	if err != nil {
		b.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go fw.Watch(ctx)
	time.Sleep(100 * time.Millisecond) // Wait for initialization

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create file
		filePath := filepath.Join(tmpDir, fmt.Sprintf("movie_%d.mkv", i))
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	// Wait for all callbacks to complete
	time.Sleep(500 * time.Millisecond)
	b.StopTimer()

	// Verify we got all callbacks
	if callbackCount != b.N {
		b.Logf("Warning: Expected %d callbacks, got %d", b.N, callbackCount)
	}
}

// BenchmarkMediaFileFilter measures the overhead of media file filtering
func BenchmarkMediaFileFilter(b *testing.B) {
	testCases := []string{
		"/path/to/movie.mkv",
		"/path/to/song.mp3",
		"/path/to/video.mp4",
		"/path/to/readme.txt",
		"/path/to/image.jpg",
		"/path/to/doc.pdf",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range testCases {
			_ = monitor.IsMediaFile(path)
		}
	}
}
