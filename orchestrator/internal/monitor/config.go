package monitor

import "time"

// Config holds configuration for file system monitoring
type Config struct {
	// Enabled determines if monitoring is active
	Enabled bool

	// Folders is a list of directories to watch for new files
	Folders []string

	// StabilityChecks is the number of file size checks (for STORY_02)
	StabilityChecks int

	// StabilityWait is the interval between stability checks (for STORY_02)
	StabilityWait time.Duration

	// StabilityTimeout is the maximum wait time for stability (for STORY_02)
	StabilityTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Enabled:          false,
		Folders:          []string{},
		StabilityChecks:  3,
		StabilityWait:    2 * time.Second,
		StabilityTimeout: 60 * time.Second,
	}
}
