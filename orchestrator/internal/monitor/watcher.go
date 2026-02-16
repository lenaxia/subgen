package monitor

import (
	"context"
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// FileCallback is called when a new file is detected
type FileCallback func(filePath string)

// FileWatcher monitors directories for new media files
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	folders  []string
	callback FileCallback
	config   *Config
	log      *logrus.Logger
}

// NewFileWatcher creates a new FileWatcher instance
func NewFileWatcher(folders []string, callback FileCallback, config *Config, log *logrus.Logger) (*FileWatcher, error) {
	if log == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	if config == nil {
		config = DefaultConfig()
	}

	return &FileWatcher{
		folders:  folders,
		callback: callback,
		config:   config,
		log:      log,
	}, nil
}

// Watch starts monitoring configured directories for file creation events.
// It blocks until the context is canceled or an unrecoverable error occurs.
func (fw *FileWatcher) Watch(ctx context.Context) error {
	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	fw.watcher = watcher

	// Add all configured folders
	for _, folder := range fw.folders {
		if err := watcher.Add(folder); err != nil {
			fw.log.WithError(err).Warnf("Failed to watch folder: %s", folder)
			// Continue watching other folders even if one fails
		} else {
			fw.log.Infof("Watching folder: %s", folder)
		}
	}

	// Event loop
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				fw.log.Info("Watcher events channel closed")
				return nil
			}

			// Only handle CREATE events
			if event.Op&fsnotify.Create == fsnotify.Create {
				fw.handleFileCreated(event.Name)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				fw.log.Info("Watcher errors channel closed")
				return nil
			}
			fw.log.WithError(err).Error("Watcher error")
			// Continue processing despite errors

		case <-ctx.Done():
			fw.log.Info("Watcher shutdown requested")
			return ctx.Err()
		}
	}
}

// handleFileCreated processes a file creation event
func (fw *FileWatcher) handleFileCreated(filePath string) {
	fw.log.WithField("file", filePath).Info("File created")

	// Wait for file stability before processing
	if !fw.WaitForStability(filePath) {
		fw.log.WithField("file", filePath).Warn("File failed stability check, skipping")
		return
	}

	if fw.callback != nil {
		fw.callback(filePath)
	}
}
