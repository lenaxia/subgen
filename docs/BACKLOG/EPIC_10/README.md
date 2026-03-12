# EPIC_10: Multi-Language Subtitle Generation

**Status:** IN PROGRESS  
**Estimated Effort:** 16-24 hours  
**Duration:** 3-4 days  
**Can Parallelize:** Limited (proto changes first, then worker changes in sequence)  
**Priority:** High

---

## Overview

Enable intelligent multi-language subtitle generation based on audio language and user preferences:
- Transcribe audio in preferred language to same language subtitles
- Translate non-preferred audio to multiple target languages
- Generate multiple subtitle files from a single media file in one pass

---

## Problem Statement

**Current Behavior:**
- Each file generates only ONE subtitle file
- No way to specify output language(s)
- Translation always outputs English (hardcoded in legacy)
- Skip logic checks for ANY subtitle, preventing additional languages

**User Requirements:**
- Japanese audio → Japanese subtitles (transcribe) + English subtitles (translate) + Chinese subtitles (translate)
- English audio → English subtitles only (transcribe, skip translation to same language)
- Support multiple target languages for family members who speak different languages

---

## Goals

1. Add `TARGET_LANGUAGES` configuration for specifying output languages
2. Add `TRANSCRIBE_PREFERRED` configuration to transcribe preferred language audio
3. Implement worker-side language policy logic
4. Fix skip logic to check specific output language (not any language)
5. Fix subtitle filename to use target language for translated subtitles
6. Deprecate unused `TRANSCRIBE_OR_TRANSLATE` configuration

---

## Architecture Decision: Worker-Centric

The **worker** handles multi-language generation because:
1. It already has access to the audio file for language detection
2. It already handles the transcription logic
3. Minimizes orchestrator changes
4. Keeps the single gRPC call pattern

### Flow

```
1. Orchestrator queues task with:
   - FilePath
   - TargetLanguages: ["eng", "zho-tw"]
   - PreferredLanguages: ["jpn", "eng"]
   - TranscribePreferred: true

2. Worker receives task:
   a. Detect audio language
   b. Apply language policy:
      - If audio in Preferred AND TranscribePreferred: transcribe (same language)
      - For each TargetLanguage: translate to that language
   c. For each output language:
      - Check skip logic for that specific language
      - If not skipped: transcribe/translate
      - Generate subtitle with correct language in filename

3. Worker returns results:
   - List of generated subtitle paths
   - List of output languages
```

---

## Configuration

### New Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TARGET_LANGUAGES` | `""` | Target languages for translation (comma or pipe-separated, e.g., `eng,zho-tw` or `eng\|zho-tw`) |
| `TRANSCRIBE_PREFERRED` | `true` | Transcribe when audio matches preferred language |

### Deprecation

| Variable | Status | Replacement |
|----------|--------|-------------|
| `TRANSCRIBE_OR_TRANSLATE` | DEPRECATED | Use `TARGET_LANGUAGES` instead |

### Configuration Examples

**Example 1: Anime Library (Japanese + English + Chinese)**
```yaml
PREFERRED_AUDIO_LANGUAGES: "jpn,eng"
TARGET_LANGUAGES: "eng,zho-tw"
TRANSCRIBE_PREFERRED: "true"
```

**Behavior for Japanese audio:**
1. Transcribe: `movie.jpn.subgen.medium.srt`
2. Translate to English: `movie.eng.subgen.medium.srt`
3. Translate to Chinese: `movie.zho-tw.subgen.medium.srt`

**Example 2: English Library (Transcribe Only)**
```yaml
PREFERRED_AUDIO_LANGUAGES: "eng"
TARGET_LANGUAGES: ""  # Empty = no translation
TRANSCRIBE_PREFERRED: "true"
```

**Example 3: Foreign Films (All to English)**
```yaml
PREFERRED_AUDIO_LANGUAGES: "eng"
TARGET_LANGUAGES: "eng"
TRANSCRIBE_PREFERRED: "true"
```

---

## Stories

### STORY_01: Proto Changes
**Effort:** 1 hour  
**Status:** DONE

Add new fields to `TranscribeRequest` and `TranscribeResponse`:
- `repeated string target_languages`
- `bool transcribe_preferred`
- `repeated string preferred_audio_languages`
- `repeated string subtitle_paths` (response)
- `repeated string output_languages` (response)

### STORY_02: Orchestrator Configuration
**Effort:** 1 hour  
**Status:** DONE

Add to `config.go`:
- `TargetLanguages []string`
- `TranscribePreferred bool`

### STORY_03: gRPC Client Updates
**Effort:** 1 hour  
**Status:** PENDING

Update `client.go` to pass new fields in `TranscribeRequest`:
- `TargetLanguages`
- `TranscribePreferred`
- `PreferredAudioLanguages`

### STORY_04: Worker Configuration
**Effort:** 1 hour  
**Status:** PENDING

Add to `settings.py`:
- `target_languages: str`
- `transcribe_preferred: bool`

### STORY_05: Language Policy Implementation
**Effort:** 4 hours  
**Status:** PENDING

Implement in `service.py`:
- Detect audio language
- Apply language policy to determine output languages
- Loop through output languages
- Handle skip logic per output language

### STORY_06: Skip Logic Fix
**Effort:** 2 hours  
**Status:** PENDING

Update `skip_checker.py`:
- Add `output_language` parameter to `check()`
- Check for specific language in subtitle filename
- Support glob patterns for target language

### STORY_07: Subtitle Filename Fix
**Effort:** 2 hours  
**Status:** PENDING

Update `writer.py`:
- Add `target_language` parameter to `generate_subtitle_path()`
- Use target language for translated subtitles
- Use detected language for transcribed subtitles

### STORY_08: Engine Updates
**Effort:** 2 hours  
**Status:** PENDING

Update `engine.py`:
- Support multiple output languages
- Accept target language parameter
- Pass target language to subtitle writer

### STORY_09: Unit Tests
**Effort:** 3 hours  
**Status:** PENDING

Test coverage:
- Language policy logic (transcribe vs translate)
- Skip logic with output language
- Filename generation with target language
- Multi-language generation

### STORY_10: Documentation
**Effort:** 1 hour  
**Status:** PENDING

Update:
- `CONFIGURATION.md` with new variables
- `README.md` with examples
- Deprecation notice for `TRANSCRIBE_OR_TRANSLATE`

---

## Technical Details

### Files Modified

| File | Changes |
|------|---------|
| `api/transcription.proto` | Add target_languages, transcribe_preferred, subtitle_paths, output_languages |
| `orchestrator/internal/config/config.go` | Add TargetLanguages, TranscribePreferred |
| `orchestrator/internal/grpc_client/client.go` | Pass new fields |
| `worker/src/config/settings.py` | Add target_languages, transcribe_preferred |
| `worker/src/grpc_server/service.py` | Implement language policy |
| `worker/src/subtitles/skip_checker.py` | Check output language |
| `worker/src/subtitles/writer.py` | Use target language for filename |
| `worker/src/transcription/engine.py` | Support target language |

### Proto Changes

```protobuf
message TranscribeRequest {
  // ... existing fields ...
  
  repeated string target_languages = 7;
  bool transcribe_preferred = 8;
  repeated string preferred_audio_languages = 9;
}

message TranscribeResponse {
  // ... existing fields ...
  
  repeated string subtitle_paths = 7;
  repeated string output_languages = 8;
}
```

### Language Policy Logic (Pseudocode)

```python
def get_output_languages(audio_lang, preferred_langs, target_langs, transcribe_preferred):
    outputs = []
    
    # 1. Transcribe preferred language
    if transcribe_preferred and audio_lang in preferred_langs:
        outputs.append((audio_lang, "transcribe"))
    
    # 2. Translate to each target language
    for target in target_langs:
        # Skip if same as audio (already handled by transcribe)
        if target != audio_lang:
            outputs.append((target, "translate"))
    
    return outputs
```

### Skip Logic (Pseudocode)

```python
def check_output_subtitle_exists(file_path, output_language):
    base = os.path.splitext(file_path)[0]
    
    # Check for subgen subtitle in specific output language
    pattern = f"{base}.subgen.*.{output_language}.srt"
    if glob.glob(pattern):
        return True
    
    # Check for simple subtitle in output language
    simple = f"{base}.{output_language}.srt"
    if os.path.exists(simple):
        return True
    
    return False
```

---

## Validation Criteria

### Functional Tests

1. **Japanese audio with multi-target**
   - Config: `PREFERRED=jpn, TARGETS=eng|zho-tw, TRANSCRIBE_PREFERRED=true`
   - Input: Japanese audio file
   - Expected: 3 subtitle files (jpn, eng, zho-tw)

2. **English audio with multi-target**
   - Config: `PREFERRED=eng, TARGETS=eng|zho-tw, TRANSCRIBE_PREFERRED=true`
   - Input: English audio file
   - Expected: 2 subtitle files (eng from transcribe, zho-tw from translate)

3. **Skip existing language**
   - Config: `TARGETS=eng|zho-tw`
   - Input: File with existing `movie.eng.subgen.medium.srt`
   - Expected: Only `zho-tw` subtitle generated (eng skipped)

4. **Empty target languages**
   - Config: `TARGETS=""` (empty)
   - Input: Any audio file
   - Expected: Single subtitle in detected language (backward compatible)

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing behavior | High | Empty TARGET_LANGUAGES falls back to current behavior |
| Performance impact (multiple transcriptions) | Medium | Sequential processing, model stays loaded |
| Complex skip logic | Medium | Comprehensive unit tests |
| Language code format mismatches | Low | Standardize on ISO 639-1 |

---

## Dependencies

- Proto regeneration tools (grpc_tools.protoc, protoc)
- Language code handling (existing `language_code.py`)

---

## References

- Original discussion: User request for multi-language subtitles
- Legacy implementation: `subgen.py` lines 1297-1298 (translate always outputs English)
- Skip logic: `worker/src/subtitles/skip_checker.py`
- Filename generation: `worker/src/subtitles/writer.py`

---

**Last Updated:** 2026-03-12  
**Created By:** opencode
