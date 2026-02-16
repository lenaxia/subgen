package skip

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// AudioTrack represents an audio track detected in a media file
type AudioTrack struct {
	// Index is the stream index in the container
	Index int
	// Language is the ISO 639-2/639-1 language code (e.g., "eng", "jpn")
	Language string
	// Title is the audio track title/description
	Title string
	// Codec is the audio codec name (e.g., "aac", "ac3", "dts")
	Codec string
	// Channels is the number of audio channels
	Channels int
}

// AudioDetector detects audio tracks using FFprobe
type AudioDetector struct {
	ffprobePath string
}

// NewAudioDetector creates a new AudioDetector
func NewAudioDetector() *AudioDetector {
	return &AudioDetector{
		ffprobePath: "ffprobe", // Assumes ffprobe is in PATH
	}
}

// GetAudioTracks detects audio tracks in a media file
func (d *AudioDetector) GetAudioTracks(ctx context.Context, filePath string) ([]AudioTrack, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}

	// Run FFprobe
	output, err := d.runFFprobe(ctx, filePath)
	if err != nil {
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	// Parse JSON output
	probe, err := parseFFprobeOutput(output)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	// Extract audio tracks
	tracks := d.extractAudioTracks(probe)

	return tracks, nil
}

// runFFprobe executes FFprobe and returns the JSON output
func (d *AudioDetector) runFFprobe(ctx context.Context, filePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, d.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "a", // Select only audio streams
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe execution error: %w", err)
	}

	return output, nil
}

// parseFFprobeOutput parses FFprobe JSON output
func parseFFprobeOutput(output []byte) (*FFProbeOutput, error) {
	var probe FFProbeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("json unmarshal error: %w", err)
	}

	return &probe, nil
}

// extractAudioTracks extracts audio track information from FFprobe output
func (d *AudioDetector) extractAudioTracks(probe *FFProbeOutput) []AudioTrack {
	var tracks []AudioTrack

	for _, stream := range probe.Streams {
		if stream.CodecType == "audio" {
			tracks = append(tracks, AudioTrack{
				Index:    stream.Index,
				Language: stream.Tags.Language,
				Title:    stream.Tags.Title,
				Codec:    stream.CodecName,
				Channels: stream.Channels,
			})
		}
	}

	return tracks
}

// HasLanguage checks if any audio track matches the given language
func (d *AudioDetector) HasLanguage(tracks []AudioTrack, language string) bool {
	if language == "" {
		return false
	}

	for _, track := range tracks {
		if track.Language == "" {
			continue
		}

		// Exact match (case insensitive)
		if strings.EqualFold(track.Language, language) {
			return true
		}

		// ISO 639 code translation (en <-> eng, ja <-> jpn, etc.)
		if languagesMatch(track.Language, language) {
			return true
		}
	}

	return false
}

// ParseLanguageList parses a pipe-separated language list
// "eng|jpn|kor" → ["eng", "jpn", "kor"]
// Handles whitespace: "eng | jpn | kor" → ["eng", "jpn", "kor"]
func ParseLanguageList(langStr string) []string {
	if langStr == "" {
		return nil
	}

	parts := strings.Split(langStr, "|")
	var languages []string

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			languages = append(languages, strings.ToLower(trimmed))
		}
	}

	return languages
}

// MatchesAnyLanguage checks if a language matches any language in the list
// Supports ISO 639-1 vs 639-2 matching (e.g., "en" matches "eng")
func MatchesAnyLanguage(targetLang string, langList []string) bool {
	if targetLang == "" || len(langList) == 0 {
		return false
	}

	targetLower := strings.ToLower(targetLang)

	for _, lang := range langList {
		langLower := strings.ToLower(lang)

		// Exact match
		if targetLower == langLower {
			return true
		}

		// ISO 639 code translation
		if languagesMatch(targetLower, langLower) {
			return true
		}
	}

	return false
}

// languagesMatch checks if two language codes refer to the same language
// Handles ISO 639-1 vs ISO 639-2 matching (e.g., "en" matches "eng")
func languagesMatch(lang1, lang2 string) bool {
	// Simple mapping for common languages
	// In production, this should use a proper ISO 639 library
	mappings := map[string]string{
		"en":  "eng",
		"eng": "en",
		"ja":  "jpn",
		"jpn": "ja",
		"fr":  "fre",
		"fre": "fr",
		"de":  "ger",
		"ger": "de",
		"es":  "spa",
		"spa": "es",
		"it":  "ita",
		"ita": "it",
		"pt":  "por",
		"por": "pt",
		"ko":  "kor",
		"kor": "ko",
		"zh":  "chi",
		"chi": "zh",
		"ru":  "rus",
		"rus": "ru",
	}

	lang1Lower := strings.ToLower(lang1)
	lang2Lower := strings.ToLower(lang2)

	// Check if they're the same
	if lang1Lower == lang2Lower {
		return true
	}

	// Check mapping
	if mapped, ok := mappings[lang1Lower]; ok {
		if mapped == lang2Lower {
			return true
		}
	}

	return false
}
