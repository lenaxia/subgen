# ASR Endpoint Quick Reference

## TL;DR

**Does ASR return actual subtitles?** YES ✅  
**Is it blocking/synchronous?** YES ✅  
**Original claim "returns placeholder"?** INCORRECT ❌

---

## Basic Usage

```bash
# Get SRT subtitles (default)
curl -X POST "http://localhost:9000/asr?language=en" \
  -F "audio_file=@audio.wav"

# Get VTT subtitles
curl -X POST "http://localhost:9000/asr?language=en&output=vtt" \
  -F "audio_file=@audio.wav"

# Get plain text
curl -X POST "http://localhost:9000/asr?language=en&output=txt" \
  -F "audio_file=@audio.wav"

# Get JSON with metadata
curl -X POST "http://localhost:9000/asr?language=en&output=json" \
  -F "audio_file=@audio.wav"
```

---

## Key Facts

| Property | Value |
|----------|-------|
| Endpoint | `POST /asr` |
| Response Type | **Blocking/Synchronous** |
| Returns | **Actual subtitle content** |
| Default Format | SRT |
| Supported Formats | srt, vtt, txt, json, tsv, lrc |
| Default Timeout | 30 seconds |
| Response Time | ~4-5s for 33s audio (0.12-0.15x real-time) |
| Deduplication | SHA256 hash of audio content |

---

## Response Codes

| Code | Meaning |
|------|---------|
| 200 | Success - subtitle content in body |
| 400 | Bad request (missing audio, invalid format) |
| 409 | Conflict (duplicate request) |
| 413 | Audio file too large |
| 429 | Queue full |
| 504 | Timeout (transcription took >30s) |

---

## Query Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `task` | No | `transcribe` | Task type (transcribe/translate) |
| `language` | No | auto-detect | Force language (e.g., en, fr, es) |
| `output` | No | `srt` | Output format (srt/vtt/txt/json/tsv/lrc) |
| `video_file` | No | - | Optional video filename for context |

---

## Sample Outputs

### SRT
```
1
00:00:00,000 --> 00:00:04,000
This is the transcribed text.
```

### VTT
```
WEBVTT

00:00:00.000 --> 00:00:04.000
This is the transcribed text.
```

### TXT
```
This is the transcribed text.
```

### JSON
```json
{
  "language": "en",
  "duration": 4.5,
  "segments": [
    {"start": 0, "end": 4, "text": "This is the transcribed text."}
  ]
}
```

---

## Behavior

### ✅ What It Does
- Accepts audio file upload
- **Blocks** until transcription completes
- Returns formatted subtitles in HTTP response body
- Detects and rejects duplicate requests

### ❌ What It Doesn't Do
- Does NOT return task IDs
- Does NOT require polling
- Does NOT return placeholders
- Does NOT need async handling

---

## Testing

```bash
# Run full test suite
./test_asr_blocking.sh

# Manual test
curl -X POST "http://localhost:9000/asr?output=srt" \
  -F "audio_file=@test/testdata/speech_sample.wav" \
  -o output.srt

# Test results: 8/8 passed (100%)
```

---

## Performance

| Metric | Value |
|--------|-------|
| Processing Speed | 0.12-0.15x real-time |
| 33s audio | ~4-5 seconds |
| 1 min audio | ~8-10 seconds |
| 5 min audio | ~40-50 seconds |

---

## Code Location

- **Handler:** `orchestrator/internal/webhooks/server.go:740-952`
- **Blocking Logic:** Lines 884-951
- **Format Writers:** `orchestrator/pkg/formats/`

---

## Common Issues

### "File not found" error with video_file parameter
**Solution:** Don't use `video_file` for pure ASR requests, or ensure file exists on worker filesystem

### "Task already queued" (HTTP 409)
**Solution:** Wait for first request to complete, or use different audio content

### Timeout (HTTP 504)
**Solution:** Increase timeout via config: `ASR.Timeout=60s`

---

## Documentation

- **Full Report:** `docs/WORKLOGS/asr_blocking_test_results.md` (501 lines)
- **Summary:** `docs/WORKLOGS/ASR_ENDPOINT_SUMMARY.md`
- **This File:** `docs/WORKLOGS/ASR_QUICK_REFERENCE.md`

---

**Last Updated:** 2026-02-17  
**Test Status:** ✅ All tests passing (8/8)  
**Production Status:** ✅ Ready for use
