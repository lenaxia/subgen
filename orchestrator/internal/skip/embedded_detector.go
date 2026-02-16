package skip

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// SubtitleDetector detects embedded subtitles in media files using FFprobe
type SubtitleDetector struct {
	ffprobePath string
}

// NewSubtitleDetector creates a new SubtitleDetector
// By default, assumes 'ffprobe' is available in PATH
func NewSubtitleDetector() *SubtitleDetector {
	return &SubtitleDetector{
		ffprobePath: "ffprobe",
	}
}

// NewSubtitleDetectorWithPath creates a SubtitleDetector with a custom FFprobe path
func NewSubtitleDetectorWithPath(ffprobePath string) *SubtitleDetector {
	return &SubtitleDetector{
		ffprobePath: ffprobePath,
	}
}

// GetEmbeddedSubtitles detects embedded subtitle tracks in a media file
// Returns a slice of SubtitleTrack with metadata for each subtitle stream
func (d *SubtitleDetector) GetEmbeddedSubtitles(ctx context.Context, filePath string) ([]SubtitleTrack, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}

	// Run FFprobe to extract subtitle stream information
	output, err := d.runFFprobe(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	// Parse FFprobe JSON output
	probe, err := d.parseFFprobeOutput(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	// Extract and return subtitle tracks
	tracks := d.extractSubtitleTracks(probe)

	return tracks, nil
}

// runFFprobe executes FFprobe with arguments to extract subtitle stream information
// Returns the raw JSON output from FFprobe
func (d *SubtitleDetector) runFFprobe(ctx context.Context, filePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, d.ffprobePath,
		"-v", "quiet", // Suppress unnecessary output
		"-print_format", "json", // Output in JSON format
		"-show_streams",        // Show stream information
		"-select_streams", "s", // Select only subtitle streams
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		// Include stderr if available for better error messages
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("ffprobe execution error: %w (stderr: %s)", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("ffprobe execution error: %w", err)
	}

	return output, nil
}

// parseFFprobeOutput parses FFprobe JSON output into FFProbeOutput struct
func (d *SubtitleDetector) parseFFprobeOutput(output []byte) (*FFProbeOutput, error) {
	var probe FFProbeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("json unmarshal error: %w", err)
	}

	return &probe, nil
}

// extractSubtitleTracks extracts subtitle track information from FFprobe output
// Filters streams to only include subtitle streams and converts to SubtitleTrack format
func (d *SubtitleDetector) extractSubtitleTracks(probe *FFProbeOutput) []SubtitleTrack {
	var tracks []SubtitleTrack

	for _, stream := range probe.Streams {
		if stream.CodecType == "subtitle" {
			tracks = append(tracks, SubtitleTrack{
				Index:    stream.Index,
				Language: stream.Tags.Language,
				Title:    stream.Tags.Title,
				Codec:    stream.CodecName,
			})
		}
	}

	return tracks
}

// HasLanguage checks if any subtitle track in the slice matches the given language code
// Returns true if at least one track has the specified language, false otherwise
func (d *SubtitleDetector) HasLanguage(tracks []SubtitleTrack, language string) bool {
	if language == "" {
		return false
	}

	for _, track := range tracks {
		if track.Language == language {
			return true
		}
	}

	return false
}
