package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// WhisperConfig contains Whisper transcription settings including advanced options
type WhisperConfig struct {
	Model       string
	Device      string
	ComputeType string
	Threads     int

	// Advanced options
	UsePrompt   bool
	Prompt      string
	Regroup     string
	ExtraKwargs map[string]interface{}
}

// ParseSubgenKwargs parses SUBGEN_KWARGS JSON string into a map
// Returns an error if JSON is invalid or contains unknown parameters
func ParseSubgenKwargs(jsonStr string) (map[string]interface{}, error) {
	if jsonStr == "" {
		return make(map[string]interface{}), nil
	}

	var kwargs map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &kwargs); err != nil {
		return nil, fmt.Errorf("invalid SUBGEN_KWARGS JSON: %w", err)
	}

	// Validate known parameters
	validParams := map[string]bool{
		"temperature":                 true,
		"compression_ratio_threshold": true,
		"logprob_threshold":           true,
		"no_speech_threshold":         true,
		"condition_on_previous_text":  true,
		"beam_size":                   true,
		"patience":                    true,
		"length_penalty":              true,
		"repetition_penalty":          true,
		"no_repeat_ngram_size":        true,
	}

	for param := range kwargs {
		if !validParams[param] {
			return nil, fmt.Errorf("unknown parameter in SUBGEN_KWARGS: %s", param)
		}
	}

	return kwargs, nil
}

// ValidateRegroupAlgorithm validates CUSTOM_REGROUP string format
// Valid formats include: "cm_sl=84_sl=42++++++1", "cm_sl=84", "da", or empty string
// Returns an error if format is invalid
func ValidateRegroupAlgorithm(regroup string) error {
	if regroup == "" {
		return nil // Empty is valid (use default)
	}

	// Check if it's only whitespace
	if strings.TrimSpace(regroup) == "" {
		return fmt.Errorf("invalid CUSTOM_REGROUP format: %s (cannot be whitespace)", regroup)
	}

	// Check if it's only digits (probably invalid)
	if _, err := strconv.Atoi(regroup); err == nil {
		return fmt.Errorf("invalid CUSTOM_REGROUP format: %s (expected format: method_options)", regroup)
	}

	// Must be at least 2 characters
	if len(regroup) < 2 {
		return fmt.Errorf("invalid CUSTOM_REGROUP format: %s (expected format: method_options or method)", regroup)
	}

	// Known valid simple methods that don't require underscores
	validSimpleMethods := map[string]bool{
		"da": true, // distribute audio
	}

	// If it's a known simple method, it's valid
	if validSimpleMethods[regroup] {
		return nil
	}

	// Otherwise, it should contain underscore (method with options)
	if !strings.Contains(regroup, "_") {
		return fmt.Errorf("invalid CUSTOM_REGROUP format: %s (expected format: method_options)", regroup)
	}

	// More detailed validation would check against stable-ts valid methods
	// For now, just pass through to worker for validation
	return nil
}

// SerializeExtraKwargs serializes kwargs map to string map for gRPC transmission
// Converts all values to JSON strings for safe transmission
func SerializeExtraKwargs(kwargs map[string]interface{}) (map[string]string, error) {
	result := make(map[string]string)

	for key, value := range kwargs {
		// Convert value to JSON string
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize kwarg %s: %w", key, err)
		}
		result[key] = string(jsonBytes)
	}

	return result, nil
}

// DeserializeExtraKwargs deserializes string map back to interface map
// Converts JSON strings back to appropriate Go types
func DeserializeExtraKwargs(serialized map[string]string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, valueStr := range serialized {
		var value interface{}
		if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
			return nil, fmt.Errorf("failed to deserialize kwarg %s: %w", key, err)
		}
		result[key] = value
	}

	return result, nil
}

// loadWhisperConfig loads Whisper configuration from Viper with validation
func loadWhisperConfig(v interface {
	GetString(string) string
	GetBool(string) bool
	GetInt(string) int
}) (*WhisperConfig, error) {
	// Parse SUBGEN_KWARGS JSON
	kwargs, err := ParseSubgenKwargs(v.GetString("SUBGEN_KWARGS"))
	if err != nil {
		return nil, err
	}

	// Validate regroup algorithm
	regroup := v.GetString("CUSTOM_REGROUP")
	if err := ValidateRegroupAlgorithm(regroup); err != nil {
		return nil, err
	}

	return &WhisperConfig{
		Model:       v.GetString("WHISPER_MODEL"),
		Device:      v.GetString("TRANSCRIBE_DEVICE"),
		ComputeType: v.GetString("COMPUTE_TYPE"),
		Threads:     v.GetInt("WHISPER_THREADS"),
		UsePrompt:   v.GetBool("USE_MODEL_PROMPT"),
		Prompt:      v.GetString("CUSTOM_MODEL_PROMPT"),
		Regroup:     regroup,
		ExtraKwargs: kwargs,
	}, nil
}
