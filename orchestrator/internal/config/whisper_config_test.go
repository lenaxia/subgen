package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseSubgenKwargs_Empty tests parsing empty SUBGEN_KWARGS
func TestParseSubgenKwargs_Empty(t *testing.T) {
	kwargs, err := ParseSubgenKwargs("")
	require.NoError(t, err)
	assert.NotNil(t, kwargs)
	assert.Equal(t, 0, len(kwargs))
}

// TestParseSubgenKwargs_ValidJSON tests parsing valid JSON
func TestParseSubgenKwargs_Valid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]interface{}
	}{
		{
			name:  "single parameter",
			input: `{"temperature": 0.0}`,
			expected: map[string]interface{}{
				"temperature": 0.0,
			},
		},
		{
			name:  "multiple parameters",
			input: `{"temperature": 0.0, "beam_size": 5, "condition_on_previous_text": false}`,
			expected: map[string]interface{}{
				"temperature":                0.0,
				"beam_size":                  float64(5), // JSON numbers are float64
				"condition_on_previous_text": false,
			},
		},
		{
			name:  "all valid parameters",
			input: `{"temperature": 0.5, "compression_ratio_threshold": 2.4, "logprob_threshold": -1.0, "no_speech_threshold": 0.6, "condition_on_previous_text": true, "beam_size": 5, "patience": 1.0, "length_penalty": 1.0, "repetition_penalty": 1.2, "no_repeat_ngram_size": 3}`,
			expected: map[string]interface{}{
				"temperature":                 0.5,
				"compression_ratio_threshold": 2.4,
				"logprob_threshold":           -1.0,
				"no_speech_threshold":         0.6,
				"condition_on_previous_text":  true,
				"beam_size":                   float64(5),
				"patience":                    1.0,
				"length_penalty":              1.0,
				"repetition_penalty":          1.2,
				"no_repeat_ngram_size":        float64(3),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kwargs, err := ParseSubgenKwargs(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, kwargs)
		})
	}
}

// TestParseSubgenKwargs_InvalidJSON tests parsing invalid JSON
func TestParseSubgenKwargs_InvalidJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "malformed JSON",
			input: `{"temperature": invalid}`,
		},
		{
			name:  "missing closing brace",
			input: `{"temperature": 0.0`,
		},
		{
			name:  "invalid structure",
			input: `["temperature", 0.0]`,
		},
		{
			name:  "not JSON",
			input: `temperature=0.0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSubgenKwargs(tt.input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid SUBGEN_KWARGS JSON")
		})
	}
}

// TestParseSubgenKwargs_UnknownParameter tests unknown parameter detection
func TestParseSubgenKwargs_UnknownParameter(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "single unknown parameter",
			input: `{"invalid_param": 123}`,
		},
		{
			name:  "mixed valid and unknown",
			input: `{"temperature": 0.0, "invalid_param": 123}`,
		},
		{
			name:  "typo in parameter name",
			input: `{"temperture": 0.0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSubgenKwargs(tt.input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unknown parameter in SUBGEN_KWARGS")
		})
	}
}

// TestParseSubgenKwargs_MixedTypes tests JSON with different data types
func TestParseSubgenKwargs_MixedTypes(t *testing.T) {
	input := `{"temperature": 0.0, "beam_size": 5, "condition_on_previous_text": false, "length_penalty": 1.5}`
	kwargs, err := ParseSubgenKwargs(input)
	require.NoError(t, err)

	// Verify types
	assert.IsType(t, float64(0), kwargs["temperature"])
	assert.IsType(t, float64(0), kwargs["beam_size"])
	assert.IsType(t, false, kwargs["condition_on_previous_text"])
	assert.IsType(t, float64(0), kwargs["length_penalty"])
}

// TestValidateRegroupAlgorithm_Valid tests valid regroup algorithms
func TestValidateRegroupAlgorithm_Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "consecutive merging with options",
			input: "cm_sl=84_sl=42++++++1",
		},
		{
			name:  "simple consecutive merging",
			input: "cm_sl=84",
		},
		{
			name:  "distribute audio",
			input: "da",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "underscore present",
			input: "method_option=value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegroupAlgorithm(tt.input)
			assert.NoError(t, err)
		})
	}
}

// TestValidateRegroupAlgorithm_Invalid tests invalid regroup algorithms
func TestValidateRegroupAlgorithm_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "no underscore",
			input: "invalid",
		},
		{
			name:  "only numbers",
			input: "123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegroupAlgorithm(tt.input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid CUSTOM_REGROUP format")
		})
	}
}

// TestWhisperConfig_Defaults tests WhisperConfig with default values
func TestWhisperConfig_Defaults(t *testing.T) {
	config := &WhisperConfig{
		Model:       "medium",
		Device:      "cpu",
		ComputeType: "auto",
		Threads:     4,
		UsePrompt:   false,
		Prompt:      "",
		Regroup:     "",
		ExtraKwargs: make(map[string]interface{}),
	}

	assert.Equal(t, "medium", config.Model)
	assert.Equal(t, "cpu", config.Device)
	assert.Equal(t, "auto", config.ComputeType)
	assert.Equal(t, 4, config.Threads)
	assert.False(t, config.UsePrompt)
	assert.Empty(t, config.Prompt)
	assert.Empty(t, config.Regroup)
	assert.NotNil(t, config.ExtraKwargs)
}

// TestWhisperConfig_WithAdvancedOptions tests WhisperConfig with advanced options
func TestWhisperConfig_WithAdvancedOptions(t *testing.T) {
	kwargs := map[string]interface{}{
		"temperature": 0.0,
		"beam_size":   float64(5),
	}

	config := &WhisperConfig{
		Model:       "large",
		Device:      "cuda",
		ComputeType: "float16",
		Threads:     8,
		UsePrompt:   true,
		Prompt:      "This is a test prompt.",
		Regroup:     "cm_sl=84_sl=42++++++1",
		ExtraKwargs: kwargs,
	}

	assert.Equal(t, "large", config.Model)
	assert.Equal(t, "cuda", config.Device)
	assert.Equal(t, "float16", config.ComputeType)
	assert.Equal(t, 8, config.Threads)
	assert.True(t, config.UsePrompt)
	assert.Equal(t, "This is a test prompt.", config.Prompt)
	assert.Equal(t, "cm_sl=84_sl=42++++++1", config.Regroup)
	assert.Equal(t, 2, len(config.ExtraKwargs))
	assert.Equal(t, 0.0, config.ExtraKwargs["temperature"])
	assert.Equal(t, float64(5), config.ExtraKwargs["beam_size"])
}

// TestSerializeExtraKwargs tests serializing kwargs for gRPC transmission
func TestSerializeExtraKwargs(t *testing.T) {
	tests := []struct {
		name     string
		kwargs   map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "empty kwargs",
			kwargs:   map[string]interface{}{},
			expected: map[string]string{},
		},
		{
			name: "single float",
			kwargs: map[string]interface{}{
				"temperature": 0.0,
			},
			expected: map[string]string{
				"temperature": "0",
			},
		},
		{
			name: "multiple types",
			kwargs: map[string]interface{}{
				"temperature":                0.5,
				"beam_size":                  float64(5),
				"condition_on_previous_text": false,
			},
			expected: map[string]string{
				"temperature":                "0.5",
				"beam_size":                  "5",
				"condition_on_previous_text": "false",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serialized, err := SerializeExtraKwargs(tt.kwargs)
			require.NoError(t, err)
			assert.Equal(t, len(tt.expected), len(serialized))

			for key, expectedVal := range tt.expected {
				assert.Contains(t, serialized, key)
				assert.Equal(t, expectedVal, serialized[key])
			}
		})
	}
}

// TestDeserializeExtraKwargs tests deserializing kwargs from gRPC
func TestDeserializeExtraKwargs(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]interface{}
	}{
		{
			name:     "empty map",
			input:    map[string]string{},
			expected: map[string]interface{}{},
		},
		{
			name: "single float",
			input: map[string]string{
				"temperature": "0.0",
			},
			expected: map[string]interface{}{
				"temperature": 0.0,
			},
		},
		{
			name: "multiple types",
			input: map[string]string{
				"temperature":                "0.5",
				"beam_size":                  "5",
				"condition_on_previous_text": "false",
			},
			expected: map[string]interface{}{
				"temperature":                0.5,
				"beam_size":                  float64(5),
				"condition_on_previous_text": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deserialized, err := DeserializeExtraKwargs(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, deserialized)
		})
	}
}
