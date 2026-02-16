package skip

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ExternalSubtitle represents a subtitle file found in the same directory
type ExternalSubtitle struct {
	// FilePath is the full path to the subtitle file
	FilePath string
	// Language is the detected language code (ISO 639-2/639-1/name)
	Language string
	// IsSubgenGenerated indicates if "subgen" is in the filename
	IsSubgenGenerated bool
	// Format is the subtitle format extension (e.g., ".srt", ".vtt")
	Format string
}

// ExternalScanner scans directories for external subtitle files
type ExternalScanner struct {
	// supportedExtensions is the list of subtitle file extensions to scan for
	supportedExtensions []string
}

// NewExternalScanner creates a new ExternalScanner
func NewExternalScanner() *ExternalScanner {
	return &ExternalScanner{
		supportedExtensions: []string{
			".srt", ".vtt", ".sub", ".ass", ".ssa",
			".idx", ".sbv", ".pgs", ".ttml", ".lrc", ".smi",
		},
	}
}

// ScanForSubtitles scans the directory containing the media file for subtitle files
// Returns a list of ExternalSubtitle found
func (s *ExternalScanner) ScanForSubtitles(filePath string) ([]ExternalSubtitle, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}

	// Get directory and base filename
	dir := filepath.Dir(filePath)
	baseName := getBaseFileName(filePath)

	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var subtitles []ExternalSubtitle

	// Scan for subtitle files
	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		fullPath := filepath.Join(dir, filename)

		// Check if file has subtitle extension
		if !s.isSubtitleFile(filename) {
			continue
		}

		// Check if filename starts with base video name
		if !s.matchesVideoFile(filename, baseName) {
			continue
		}

		// Parse language from filename (pass base name to filter it out)
		language, _ := s.ParseLanguageFromFilename(filename, baseName)

		// Create ExternalSubtitle entry
		subtitle := ExternalSubtitle{
			FilePath:          fullPath,
			Language:          language,
			IsSubgenGenerated: s.IsSubgenGenerated(filename),
			Format:            filepath.Ext(filename),
		}

		subtitles = append(subtitles, subtitle)
	}

	return subtitles, nil
}

// ParseLanguageFromFilename extracts language code from subtitle filename
// Returns (language_code, true) if found, ("", false) otherwise
// Supports ISO 639-1 (en), ISO 639-2 (eng), and full names (english)
// Takes videoBaseName to filter out the video filename portion
func (s *ExternalScanner) ParseLanguageFromFilename(filename string, videoBaseName string) (string, bool) {
	// Remove extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Remove the video base name prefix to get only the subtitle-specific parts
	// e.g., "movie.eng.srt" with baseName "movie" → ".eng"
	if strings.HasPrefix(nameWithoutExt, videoBaseName) {
		nameWithoutExt = strings.TrimPrefix(nameWithoutExt, videoBaseName)
	}

	// Remove leading dot if present
	nameWithoutExt = strings.TrimPrefix(nameWithoutExt, ".")

	// If nothing remains after removing base name, no language code
	if nameWithoutExt == "" {
		return "", false
	}

	// Split by dots to get parts
	parts := strings.Split(nameWithoutExt, ".")

	// Check each part for language code
	for _, part := range parts {
		// Convert to lowercase for comparison
		partLower := strings.ToLower(part)

		// Skip common non-language parts
		if s.isNonLanguagePart(partLower) {
			continue
		}

		// Check if it's a valid language code
		if s.isValidLanguageCode(partLower) {
			return partLower, true
		}
	}

	return "", false
}

// HasLanguage checks if any subtitle matches the target language
func (s *ExternalScanner) HasLanguage(subtitles []ExternalSubtitle, targetLang string) bool {
	if targetLang == "" {
		return false
	}

	targetLower := strings.ToLower(targetLang)

	for _, subtitle := range subtitles {
		if subtitle.Language == "" {
			continue
		}

		// Match if languages are equal
		if strings.ToLower(subtitle.Language) == targetLower {
			return true
		}

		// Also try to match ISO 639-1 vs ISO 639-2
		// e.g., "en" matches "eng", "ja" matches "jpn"
		if s.languagesMatch(subtitle.Language, targetLang) {
			return true
		}
	}

	return false
}

// IsSubgenGenerated checks if "subgen" appears in the filename
func (s *ExternalScanner) IsSubgenGenerated(filename string) bool {
	// Remove extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Split by dots and check each part
	parts := strings.Split(nameWithoutExt, ".")

	for _, part := range parts {
		if strings.ToLower(part) == "subgen" {
			return true
		}
	}

	return false
}

// Helper functions

// getBaseFileName returns the base filename without extension and path
func getBaseFileName(filePath string) string {
	filename := filepath.Base(filePath)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

// isSubtitleFile checks if filename has a subtitle extension
func (s *ExternalScanner) isSubtitleFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	for _, supportedExt := range s.supportedExtensions {
		if ext == supportedExt {
			return true
		}
	}

	return false
}

// matchesVideoFile checks if subtitle filename matches video filename
func (s *ExternalScanner) matchesVideoFile(subtitleName, videoBaseName string) bool {
	// Remove extension from subtitle
	subtitleBase := strings.TrimSuffix(subtitleName, filepath.Ext(subtitleName))

	// Subtitle should start with video base name
	return strings.HasPrefix(subtitleBase, videoBaseName)
}

// isNonLanguagePart checks if a part is not a language code
func (s *ExternalScanner) isNonLanguagePart(part string) bool {
	nonLanguageParts := []string{
		"subgen", "forced", "sdh", "cc", "hi", "full",
		"signs", "songs", "commentary", "default",
	}

	for _, nonLang := range nonLanguageParts {
		if part == nonLang {
			return true
		}
	}

	// Check if it's a number (track number)
	if _, err := strconv.Atoi(part); err == nil {
		return true
	}

	return false
}

// isValidLanguageCode checks if a string is a valid language code
// Supports ISO 639-1 (2 chars), ISO 639-2 (3 chars), and full names
func (s *ExternalScanner) isValidLanguageCode(code string) bool {
	// Must be at least 2 characters
	if len(code) < 2 {
		return false
	}

	// Must be alphabetic
	for _, ch := range code {
		if !('a' <= ch && ch <= 'z') {
			return false
		}
	}

	// Valid if 2-10 characters (reasonable language code length)
	return len(code) >= 2 && len(code) <= 10
}

// languagesMatch checks if two language codes refer to the same language
// Handles ISO 639-1 vs ISO 639-2 matching (e.g., "en" matches "eng")
func (s *ExternalScanner) languagesMatch(lang1, lang2 string) bool {
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
