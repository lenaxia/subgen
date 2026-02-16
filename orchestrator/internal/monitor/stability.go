package monitor

import (
	"os"
	"time"
)

// WaitForStability checks if a file has stopped growing/changing.
// Returns true if file is stable, false if timeout or error occurs.
//
// The algorithm performs multiple consecutive checks of the file size at
// configured intervals. The file is considered stable only when the size
// remains unchanged for all required checks. If the size changes during
// checking, the counter resets and checking starts over.
//
// This prevents processing of files that are still being uploaded or copied.
func (fw *FileWatcher) WaitForStability(filePath string) bool {
	// Stability checking disabled
	if fw.config.StabilityChecks <= 0 {
		return true
	}

	fw.log.WithField("file", filePath).Debug("Waiting for file stability")

	checks := fw.config.StabilityChecks
	interval := fw.config.StabilityWait
	timeout := time.After(fw.config.StabilityTimeout)

	var lastSize int64 = -1
	stableCount := 0

	for stableCount < checks {
		select {
		case <-timeout:
			fw.log.WithField("file", filePath).Warn("Stability check timeout")
			return false

		default:
			// Get current file size
			stat, err := os.Stat(filePath)
			if err != nil {
				fw.log.WithError(err).WithField("file", filePath).Error("Failed to stat file during stability check")
				return false
			}

			currentSize := stat.Size()

			// Check if size is stable
			if currentSize == lastSize {
				stableCount++
				fw.log.WithFields(map[string]interface{}{
					"file":        filePath,
					"size":        currentSize,
					"stableCount": stableCount,
					"required":    checks,
				}).Debug("File size stable")
			} else {
				// Size changed, reset counter
				if lastSize != -1 {
					fw.log.WithFields(map[string]interface{}{
						"file":    filePath,
						"oldSize": lastSize,
						"newSize": currentSize,
					}).Debug("File size changed, resetting stability counter")
				}
				stableCount = 0
				lastSize = currentSize
			}

			// Wait before next check (unless this was the last check)
			if stableCount < checks {
				time.Sleep(interval)
			}
		}
	}

	fw.log.WithFields(map[string]interface{}{
		"file": filePath,
		"size": lastSize,
	}).Info("File is stable")

	return true
}
