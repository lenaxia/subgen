# Story 08: Advanced Whisper Options

**Epic**: EPIC_08  
**Status**: Not Started  
**Effort**: 4-6 hours  
**Priority**: LOW  
**Assignee**: Delegation Agent

---

## User Story

As a power user of Subgen,
I want to configure advanced Whisper transcription parameters,
So that I can fine-tune transcription quality for specific use cases.

---

## Background

The original subgen.py (lines 138-139, 1389-1418) supported several advanced Whisper options:
- `SUBGEN_KWARGS` - JSON string with arbitrary Whisper parameters
- `USE_MODEL_PROMPT` - Enable prompt usage to force punctuation
- `CUSTOM_MODEL_PROMPT` - Custom prompt text for transcription
- `CUSTOM_REGROUP` - stable-ts regrouping algorithm

These options allow power users to:
- Adjust temperature, beam_search, compression_ratio_threshold
- Force punctuation and capitalization via prompts
- Control subtitle grouping and timing
- Fine-tune for specific languages or audio quality

---

## Acceptance Criteria

- [ ] `SUBGEN_KWARGS` - JSON string parsed and passed to worker
- [ ] `USE_MODEL_PROMPT` - Boolean to enable custom prompts
- [ ] `CUSTOM_MODEL_PROMPT` - Text prompt passed to Whisper model
- [ ] `CUSTOM_REGROUP` - stable-ts regrouping algorithm string
- [ ] Configuration validation before transcription
- [ ] Parameters passed via gRPC to worker
- [ ] Worker applies parameters during transcription
- [ ] Invalid JSON returns clear error
- [ ] Documentation with examples
- [ ] Unit tests for config parsing
- [ ] Integration tests with worker
- [ ] Type checking passes
- [ ] Work log created

---

## Technical Design

### Configuration

```bash
# Advanced Whisper parameters (JSON)
SUBGEN_KWARGS='{"temperature": 0.0, "compression_ratio_threshold": 2.4, "condition_on_previous_text": false, "beam_size": 5}'

# Custom prompt (forces punctuation)
USE_MODEL_PROMPT=true
CUSTOM_MODEL_PROMPT="This is a transcript with proper punctuation and capitalization."

# Stable-TS regrouping algorithm
# Format: [method]_[options]
# Example: cm=consecutive merging, sl=segment length
CUSTOM_REGROUP="cm_sl=84_sl=42++++++1"
```

### Config Structure

```go
// orchestrator/internal/config/whisper.go
package config

import (
	"encoding/json"
	"fmt"
)

// WhisperConfig contains Whisper transcription settings
type WhisperConfig struct {
	Model       string
	Device      string
	ComputeType string
	Threads     int
	
	// Advanced options
	UsePrompt       bool
	Prompt          string
	Regroup         string
	ExtraKwargs     map[string]interface{}
}

// ParseSubgenKwargs parses SUBGEN_KWARGS JSON string
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
		"temperature":                  true,
		"compression_ratio_threshold":  true,
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

// ValidateRegroupAlgorithm validates CUSTOM_REGROUP string
func ValidateRegroupAlgorithm(regroup string) error {
	if regroup == "" {
		return nil // Empty is valid (use default)
	}
	
	// Basic validation: should contain underscore and alphanumeric chars
	if !strings.Contains(regroup, "_") {
		return fmt.Errorf("invalid CUSTOM_REGROUP format: %s (expected format: method_options)", regroup)
	}
	
	// More detailed validation would check against stable-ts valid methods
	// For now, just pass through to worker for validation
	return nil
}

// LoadWhisperConfig loads Whisper configuration from Viper
func LoadWhisperConfig(v *viper.Viper) (*WhisperConfig, error) {
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
```

### gRPC Protocol Extension

```protobuf
// proto/worker.proto additions
message TranscribeRequest {
  // ... existing fields ...
  
  // Advanced options
  optional string prompt = 10;
  optional string regroup_algorithm = 11;
  map<string, string> extra_kwargs = 12;  // JSON-encoded parameters
}
```

### Worker Implementation

```python
# worker/src/transcriber.py
class Transcriber:
    def transcribe(self, request: TranscribeRequest) -> TranscribeResponse:
        # Load model if needed
        if not self.model:
            self._load_model()
        
        # Build transcription kwargs
        kwargs = {
            "language": request.language,
            "task": request.task,
        }
        
        # Add custom prompt if provided
        if request.prompt:
            kwargs["initial_prompt"] = request.prompt
        
        # Add extra kwargs from SUBGEN_KWARGS
        if request.extra_kwargs:
            for key, value in request.extra_kwargs.items():
                # Parse JSON-encoded value
                kwargs[key] = json.loads(value)
        
        # Perform transcription
        result = self.model.transcribe(request.audio_file, **kwargs)
        
        # Apply custom regrouping if specified
        if request.regroup_algorithm:
            result = result.regroup(request.regroup_algorithm)
        
        # ... rest of implementation ...
```

### Files to Create/Modify

**New Files:**
- `orchestrator/internal/config/whisper.go` - WhisperConfig struct and parsing
- `orchestrator/internal/config/whisper_test.go` - Unit tests

**Modified Files:**
- `orchestrator/internal/config/config.go` - Add Whisper field to Config struct
- `proto/worker.proto` - Add advanced options to TranscribeRequest
- `worker/src/transcriber.py` - Apply advanced options during transcription

---

## Testing Strategy

### Unit Tests

**whisper_test.go:**
```go
func TestParseSubgenKwargs_Empty(t *testing.T) {
	// Test empty string returns empty map
}

func TestParseSubgenKwargs_Valid(t *testing.T) {
	// Test valid JSON parses correctly
}

func TestParseSubgenKwargs_InvalidJSON(t *testing.T) {
	// Test malformed JSON returns error
}

func TestParseSubgenKwargs_UnknownParameter(t *testing.T) {
	// Test unknown parameter returns error
}

func TestParseSubgenKwargs_MixedTypes(t *testing.T) {
	// Test JSON with strings, numbers, booleans
}

func TestValidateRegroupAlgorithm_Valid(t *testing.T) {
	// Test valid regroup algorithm strings
}

func TestValidateRegroupAlgorithm_Invalid(t *testing.T) {
	// Test invalid formats return error
}

func TestValidateRegroupAlgorithm_Empty(t *testing.T) {
	// Test empty string is valid (use default)
}

func TestLoadWhisperConfig_AllOptions(t *testing.T) {
	// Test loading all options together
}
```

### Integration Tests

```go
func TestWhisperAdvancedOptions_Integration(t *testing.T) {
	// Start worker with advanced options configured
	// Queue transcription task
	// Verify parameters passed to worker
	// Verify transcription uses custom settings
}
```

### Manual Testing

```bash
# Test 1: Custom temperature
export SUBGEN_KWARGS='{"temperature": 0.0}'
# Trigger transcription
# Verify lower temperature (more conservative) in output

# Test 2: Custom prompt
export USE_MODEL_PROMPT=true
export CUSTOM_MODEL_PROMPT="This transcript has proper punctuation."
# Trigger transcription
# Verify better punctuation in output

# Test 3: Custom regroup
export CUSTOM_REGROUP="cm_sl=84_sl=42++++++1"
# Trigger transcription
# Verify subtitle timing follows custom algorithm

# Test 4: All options combined
export SUBGEN_KWARGS='{"temperature": 0.0, "beam_size": 5}'
export USE_MODEL_PROMPT=true
export CUSTOM_MODEL_PROMPT="This is a test."
export CUSTOM_REGROUP="cm_sl=84"
# Trigger transcription
# Verify all options applied

# Test 5: Invalid JSON
export SUBGEN_KWARGS='{"temperature": invalid}'
# Start orchestrator
# Expected: Error on startup "invalid SUBGEN_KWARGS JSON"

# Test 6: Unknown parameter
export SUBGEN_KWARGS='{"invalid_param": 123}'
# Start orchestrator
# Expected: Error "unknown parameter in SUBGEN_KWARGS: invalid_param"
```

---

## Definition of Done

- [ ] Story file created (this document)
- [ ] Tests written FIRST (TDD)
- [ ] WhisperConfig struct implemented
- [ ] ParseSubgenKwargs implemented with validation
- [ ] Configuration loaded from environment
- [ ] gRPC protocol updated
- [ ] Worker applies advanced options
- [ ] All unit tests passing
- [ ] Integration tests passing
- [ ] Manual testing completed
- [ ] Type checking passes
- [ ] Documentation with examples
- [ ] Work log created (0026_2026-02-16_epic08_story08_advanced_whisper.md)
- [ ] Code committed and pushed

---

## Validation Rules

### SUBGEN_KWARGS Validation

**Valid Parameters:**
- `temperature` (float, 0.0-1.0) - Sampling temperature
- `compression_ratio_threshold` (float) - Compression threshold for hallucination detection
- `logprob_threshold` (float) - Log probability threshold
- `no_speech_threshold` (float) - Silence detection threshold
- `condition_on_previous_text` (bool) - Use previous context
- `beam_size` (int) - Beam search width
- `patience` (float) - Beam search patience
- `length_penalty` (float) - Length penalty for beam search
- `repetition_penalty` (float) - Repetition penalty
- `no_repeat_ngram_size` (int) - N-gram blocking size

**Invalid Examples:**
```json
{"invalid_param": 123}           // Unknown parameter
{"temperature": "high"}          // Wrong type (should be float)
{"temperature"}                  // Malformed JSON
```

### CUSTOM_REGROUP Validation

**Valid Formats:**
- `cm_sl=84_sl=42++++++1` - Consecutive merging with segment length constraints
- `cm_sl=84` - Simple consecutive merging
- `da` - Distribute audio evenly
- Empty string - Use default regrouping

**Invalid Examples:**
- `invalid` - No underscore or method specified
- `123` - Not a valid algorithm name

---

## Documentation Examples

```bash
# Example 1: Improve punctuation
SUBGEN_KWARGS='{"temperature": 0.0, "condition_on_previous_text": true}'
USE_MODEL_PROMPT=true
CUSTOM_MODEL_PROMPT="This is a professionally transcribed interview with proper grammar and punctuation."

# Example 2: Aggressive hallucination filtering
SUBGEN_KWARGS='{"compression_ratio_threshold": 2.2, "logprob_threshold": -1.0, "no_speech_threshold": 0.6}'

# Example 3: Beam search for better quality (slower)
SUBGEN_KWARGS='{"beam_size": 5, "patience": 1.0, "length_penalty": 1.0}'

# Example 4: Custom subtitle timing
CUSTOM_REGROUP="cm_sl=84_sl=42++++++1"  # Max 84 chars, prefer 42 chars per line

# Example 5: Reduce repetitions
SUBGEN_KWARGS='{"repetition_penalty": 1.2, "no_repeat_ngram_size": 3}'
```

---

## Success Criteria

1. **Validation**: Invalid configs rejected at startup
2. **Functionality**: All parameters actually affect transcription
3. **Documentation**: Clear examples for common use cases
4. **Performance**: Validation adds <10ms to startup time
5. **Reliability**: No panics from invalid parameter combinations

---

## References

- **Original Implementation**: subgen.py lines 138-139, 1389-1418
- **Whisper Parameters**: https://github.com/openai/whisper
- **Stable-TS Regroup**: https://github.com/jianfch/stable-ts
- **faster-whisper**: https://github.com/guillaumekln/faster-whisper

---

**Story Created**: 2026-02-16  
**Last Updated**: 2026-02-16
