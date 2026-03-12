package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/sirupsen/logrus"
)

// ScanResult contains statistics from directory scan
type ScanResult struct {
	Scanned     int            // Total files scanned
	Queued      int            // Files queued for transcription
	Skipped     int            // Files skipped
	SkipReasons map[string]int // Skip reason counts
}

// QueueInterface defines the interface for task queueing
type QueueInterface interface {
	Enqueue(task interface{}) error
}

// Scanner scans directories for media files
type Scanner interface {
	ScanDirectory(directory string, recursive bool, language string) (*ScanResult, error)
}

// BasicScanner is a scanner implementation that finds media files and queues them
type BasicScanner struct {
	queue  QueueInterface
	log    *logrus.Logger
	config *config.Config
}

// NewScanner creates a new scanner instance
func NewScanner(queue QueueInterface, cfg *config.Config) Scanner {
	return &BasicScanner{
		queue:  queue,
		log:    nil, // Optional logger
		config: cfg,
	}
}

// NewScannerWithLogger creates a new scanner instance with logger for progress logging
func NewScannerWithLogger(queue QueueInterface, log *logrus.Logger, cfg *config.Config) Scanner {
	return &BasicScanner{
		queue:  queue,
		log:    log,
		config: cfg,
	}
}

// mediaExtensions contains all supported media file extensions
var mediaExtensions = map[string]bool{
	// Video formats
	".mkv":  true,
	".mp4":  true,
	".avi":  true,
	".mov":  true,
	".m4v":  true,
	".webm": true,
	".flv":  true,
	".wmv":  true,
	".mpg":  true,
	".mpeg": true,
	".m2ts": true,
	".ts":   true,
	// Audio formats
	".mp3":  true,
	".flac": true,
	".m4a":  true,
	".wav":  true,
	".ogg":  true,
	".opus": true,
	".wma":  true,
	".aac":  true,
}

// IsMediaFile checks if a file has a supported media extension
func IsMediaFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return mediaExtensions[ext]
}

// ScanDirectory scans a directory for media files and queues them for transcription
func (s *BasicScanner) ScanDirectory(directory string, recursive bool, language string) (*ScanResult, error) {
	// Validate directory exists
	info, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not found: %s", directory)
		}
		if os.IsPermission(err) {
			return nil, fmt.Errorf("permission denied: %s", directory)
		}
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}

	// Verify it's a directory
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", directory)
	}

	result := &ScanResult{
		SkipReasons: make(map[string]int),
	}

	// Get batch scan limit from config (0 means unlimited)
	batchScanLimit := 0
	if s.config != nil {
		batchScanLimit = s.config.Monitor.BatchScanLimit
	}

	// Walk directory tree
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files/directories we can't access
			return nil
		}

		// Skip directories (but continue walking if recursive)
		if info.IsDir() {
			// If not recursive and not the root directory, skip subdirectories
			if !recursive && path != directory {
				return filepath.SkipDir
			}
			return nil
		}

		// Filter by media extension
		if !IsMediaFile(path) {
			return nil
		}

		// Count as scanned
		result.Scanned++

		// Check batch scan limit (0 means unlimited)
		if batchScanLimit > 0 && result.Scanned > batchScanLimit {
			if s.log != nil {
				s.log.WithFields(logrus.Fields{
					"scanned": result.Scanned,
					"limit":   batchScanLimit,
				}).Warn("Batch scan limit reached, stopping scan")
			}
			return filepath.SkipAll // Stop scanning
		}

		// Progress logging every 100 files
		if s.log != nil && result.Scanned%100 == 0 {
			s.log.WithFields(logrus.Fields{
				"scanned": result.Scanned,
				"queued":  result.Queued,
				"limit":   batchScanLimit,
			}).Infof("Scan progress: %d files scanned", result.Scanned)
		}

		// Queue file for transcription
		if s.queue != nil {
			task := map[string]interface{}{
				"file_path":  path,
				"language":   language,
				"priority":   2, // Standard priority
				"from_batch": true,
			}

			if err := s.queue.Enqueue(task); err != nil {
				// Log error but continue processing other files
				return nil
			}

			result.Queued++
		}

		return nil
	}

	// Walk the directory tree
	if err := filepath.Walk(directory, walkFunc); err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return result, nil
}
