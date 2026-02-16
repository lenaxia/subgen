package monitor

import (
	"os"
	"path/filepath"
)

// addRecursive recursively adds all subdirectories to the watcher
func (fw *FileWatcher) addRecursive(rootPath string) error {
	dirCount := 0

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fw.log.WithError(err).Warnf("Error accessing path: %s", path)
			// Continue walking despite errors
			return nil
		}

		// Only add directories
		if !info.IsDir() {
			return nil
		}

		// Skip symlinks to prevent infinite loops
		if info.Mode()&os.ModeSymlink != 0 {
			fw.log.Debugf("Skipping symlink directory: %s", path)
			return filepath.SkipDir
		}

		// Add directory to watcher
		if fw.watcher != nil {
			if err := fw.watcher.Add(path); err != nil {
				fw.log.WithError(err).Warnf("Failed to watch subdirectory: %s", path)
			} else {
				dirCount++
				fw.log.Debugf("Watching subdirectory: %s", path)
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	if dirCount > 0 {
		fw.log.WithField("directories", dirCount).Infof("Added %d directories to watcher for %s", dirCount, rootPath)
	}
	return nil
}

// isDirectory checks if a path is a directory
// Returns false if path doesn't exist or is not a directory
func (fw *FileWatcher) isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// handleDirectoryCreated processes a directory creation event
func (fw *FileWatcher) handleDirectoryCreated(dirPath string) {
	fw.log.WithField("directory", dirPath).Info("Directory created")

	// Add the new directory and all its subdirectories to watcher
	if err := fw.addRecursive(dirPath); err != nil {
		fw.log.WithError(err).Warnf("Failed to add recursive watch for: %s", dirPath)
	} else {
		fw.log.WithField("directory", dirPath).Info("Added new directory to watcher")
	}
}
