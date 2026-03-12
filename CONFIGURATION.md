# Subgen Configuration Reference

**Complete guide to all configuration options, environment variables, and tuning dials.**

## 📋 Quick Reference

| Category | Variables | Purpose |
|----------|-----------|---------|
| **Core** | `WORKER_ADDRESS`, `WEBHOOK_PORT`, `METRICS_PORT` | Basic service configuration |
| **Media Servers** | `PLEX_*`, `JELLYFIN_*`, `EMBY_*` | Media server integration |
| **Whisper Models** | `WHISPER_MODEL`, `TRANSCRIBE_DEVICE`, `COMPUTE_TYPE` | Transcription engine settings |
| **Performance** | `CONCURRENT_TRANSCRIPTIONS`, `WHISPER_THREADS`, `MAX_WORKERS` | Resource allocation |
| **Skip Logic** | `SKIP_IF_*`, `SKIP_*_LANGUAGES` | Control when to skip processing |
| **Language** | `FORCE_LANGUAGE`, `PREFERRED_AUDIO_LANGUAGES`, `TARGET_LANGUAGES` | Language detection and multi-language output |
| **Subtitle Format** | `WORD_LEVEL_HIGHLIGHT`, `LRC_FOR_AUDIO_FILES`, `APPEND_FOOTER` | Subtitle output formatting |
| **File Monitoring** | `MONITOR`, `TRANSCRIBE_FOLDERS`, `SCANNER_INITIALIZED` | File system watching |
| **Path Mapping** | `USE_PATH_MAPPING`, `PATH_MAPPING_FROM`, `PATH_MAPPING_TO` | Path translation for containers |
| **Memory Management** | `MODEL_CLEANUP_DELAY`, `MEMORY_THRESHOLD_MB`, `CLEAR_VRAM_ON_COMPLETE` | Resource cleanup |
| **Queue Management** | `QUEUE_MAX_SIZE`, `QUEUE_WORKER_TIMEOUT` | Task queue settings |
| **Plex Advanced** | `PLEX_QUEUE_NEXT_EPISODE`, `PLEX_QUEUE_SEASON`, `PLEX_QUEUE_SERIES` | TV episode queueing |

---

## 🏗️ Core Configuration

### Orchestrator Settings

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `WORKER_ADDRESS` | `worker:50051` | **Required.** gRPC address of transcription worker | `localhost:50051`, `192.168.1.100:50051` |
| `WEBHOOK_PORT` | `9000` | Port for webhooks and API endpoints | `9000` |
| `METRICS_PORT` | `9090` | Port for Prometheus metrics | `9090` |
| `LOG_LEVEL` | `debug` | Logging level: `debug`, `info`, `warn`, `error` | `info` |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` | `json` |
| `WORKER_DISCOVERY` | `kubernetes` | Worker discovery mode: `kubernetes`, `static` | `static` |
| `WORKER_SERVICE_NAME` | `subgen-worker` | Kubernetes service name for workers | `subgen-worker` |
| `WORKER_NAMESPACE` | `default` | Kubernetes namespace for workers | `default` |
| `WORKER_PORT` | `50051` | Worker gRPC port | `50051` |

### Worker Settings

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `GRPC_PORT` | `50051` | gRPC server port for transcription requests | `50051` |
| `HTTP_PORT` | `8080` | HTTP health check server port (v0.2.18+) | `8080` |
| `MAX_WORKERS` | `10` | Total gRPC thread pool size | `10` |
| `CONCURRENT_TRANSCRIPTIONS` | `2` | Max concurrent transcription jobs | `2` |
| `MEMORY_THRESHOLD_MB` | `3000` | Memory threshold for model unload (MB) | `3000` |

**Thread Pool Formula**: `MAX_WORKERS ≥ CONCURRENT_TRANSCRIPTIONS × 2 + 2`
- Example: 2 concurrent jobs → `2×2 + 2 = 6` threads minimum
- System needs 2 threads for health checks and metrics

---

## 🎬 Media Server Integration

### Plex Configuration

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `PLEX_ENABLED` | `true` | Enable Plex integration | `true` |
| `PLEX_SERVER` | `http://192.168.1.111:32400` | Plex server URL | `http://plex:32400` |
| `PLEX_TOKEN` | `""` | Plex authentication token | `your-plex-token` |
| `PLEX_QUEUE_NEXT_EPISODE` | `false` | Auto-queue next episode in series | `true` |
| `PLEX_QUEUE_SEASON` | `false` | Auto-queue rest of season | `false` |
| `PLEX_QUEUE_SERIES` | `false` | Auto-queue entire series | `false` |

**Webhook URL**: `http://your-subgen-server:9000/plex`

### Jellyfin Configuration

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `JELLYFIN_ENABLED` | `false` | Enable Jellyfin integration | `true` |
| `JELLYFIN_SERVER` | `http://192.168.1.111:8096` | Jellyfin server URL | `http://jellyfin:8096` |
| `JELLYFIN_TOKEN` | `""` | Jellyfin API token | `your-jellyfin-token` |

**Webhook URL**: `http://your-subgen-server:9000/jellyfin`

### Emby Configuration

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `EMBY_ENABLED` | `false` | Enable Emby integration | `true` |
| `EMBY_SERVER` | `""` | Emby server URL | `http://emby:8096` |
| `EMBY_TOKEN` | `""` | Emby API token | `your-emby-token` |

**Webhook URL**: `http://your-subgen-server:9000/emby`

### Processing Triggers

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `PROCESS_ADDED_MEDIA` | `true` | Process media when added to library | `true` |
| `PROCESS_MEDIA_ON_PLAY` | `true` | Process media when played | `false` |
| `TRANSCRIBE_OR_TRANSLATE` | `transcribe` | **DEPRECATED**: Use `TARGET_LANGUAGES` instead | `transcribe` |

> **Note**: `TRANSCRIBE_OR_TRANSLATE` is deprecated as of EPIC_10. Use `TARGET_LANGUAGES` for multi-language subtitle generation. If `TARGET_LANGUAGES` is empty, the system falls back to single-language mode (transcribe detected language).

---

## 🤖 Whisper Model Configuration

### Model Selection

| Variable | Default | Description | Options |
|----------|---------|-------------|---------|
| `WHISPER_MODEL` | `medium` | Whisper model size | `tiny`, `base`, `small`, `medium`, `large`, `large-v2`, `large-v3`, `distil-small.en`, `distil-medium.en`, `distil-large-v2`, `distil-large-v3` |
| `TRANSCRIBE_DEVICE` | `cpu` | Device for inference | `cpu`, `cuda` |
| `COMPUTE_TYPE` | `auto` | Quantization type | `auto`, `int8`, `int8_float16`, `float16`, `float32` |
| `MODEL_PATH` | `./models` | Path to model storage | `/models`, `/subgen/models` |

### Model Performance Guide

| Model | VRAM Required | Speed | Quality | Use Case |
|-------|---------------|-------|---------|----------|
| `tiny` | 1GB | ⚡ Fastest | ⭐ Basic | Quick testing, low-resource systems |
| `base` | 1GB | ⚡ Fast | ⭐⭐ Acceptable | General purpose, good speed/quality balance |
| `small` | 2GB | ⚡ Fast | ⭐⭐⭐ Good | Recommended for most users |
| `medium` | 5GB | ⚖️ Balanced | ⭐⭐⭐⭐ Very Good | **Recommended default** |
| `large-v3` | 10GB | 🐢 Slowest | ⭐⭐⭐⭐⭐ Best | Maximum accuracy, high-resource systems |
| `distil-*` | 2-5GB | ⚡ Faster | ⭐⭐⭐ Good | Faster variants with similar quality |

### Hardware Recommendations

**GPU (CUDA)**:
```yaml
TRANSCRIBE_DEVICE: "cuda"
WHISPER_MODEL: "medium"  # or "large-v3" for 10GB+ VRAM
CONCURRENT_TRANSCRIPTIONS: "4"  # Depends on VRAM
```

**CPU**:
```yaml
TRANSCRIBE_DEVICE: "cpu"
WHISPER_MODEL: "small"  # or "tiny" for faster processing
WHISPER_THREADS: "4"
CONCURRENT_TRANSCRIPTIONS: "2"  # Depends on CPU cores
```

---

## ⚡ Performance Tuning

### Concurrency Settings

| Variable | Default | Description | Formula |
|----------|---------|-------------|---------|
| `CONCURRENT_TRANSCRIPTIONS` | `2` | Max concurrent transcription jobs | Based on hardware |
| `WHISPER_THREADS` | `4` | CPU threads per transcription job | `CPU cores ÷ CONCURRENT_TRANSCRIPTIONS` |
| `MAX_WORKERS` | `10` | Total gRPC thread pool size | `CONCURRENT_TRANSCRIPTIONS × 2 + 2` |

**Example Calculations**:
- **4-core CPU, 2 concurrent jobs**: `WHISPER_THREADS=2`, `MAX_WORKERS=6`
- **8-core CPU, 4 concurrent jobs**: `WHISPER_THREADS=2`, `MAX_WORKERS=10`
- **GPU with 8GB VRAM**: `CONCURRENT_TRANSCRIPTIONS=2` (medium model), `MAX_WORKERS=6`

### Memory Management

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `MODEL_CLEANUP_DELAY` | `300` | Seconds before unloading idle model | `300` (5 minutes) |
| `CLEAR_VRAM_ON_COMPLETE` | `true` | Clear CUDA VRAM after transcription | `false` for CPU workers |
| `QUEUE_MAX_SIZE` | `1000` | Maximum tasks in queue | `1000` |
| `QUEUE_WORKER_TIMEOUT` | `18000` | Worker timeout in seconds (5 hours) | `18000` |

**Cleanup Strategy**:
- `MODEL_CLEANUP_DELAY=0`: Unload immediately after each job (slow, saves memory)
- `MODEL_CLEANUP_DELAY=300`: Keep loaded for 5 minutes (balanced)
- `MODEL_CLEANUP_DELAY=1800`: Keep loaded for 30 minutes (fast, uses more memory)

---

## 🚫 Skip Logic Configuration

### Basic Skip Conditions

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `SKIP_IF_EXTERNAL_SUBTITLES_EXIST` | `false` | Skip if any external subtitles exist | `true` |
| `SKIP_IF_TARGET_SUBTITLES_EXIST` | `true` | Skip if target language subtitles exist | `true` |
| `SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE` | `""` | Skip if internal subs in this language | `eng` |
| `SKIP_ONLY_SUBGEN_SUBTITLES` | `false` | Only skip subgen-generated subtitles | `false` |

### Language-Based Skipping

| Variable | Default | Description | Format |
|----------|---------|-------------|---------|
| `SKIP_SUBTITLE_LANGUAGES` | `""` | Subtitle languages to skip | `eng|jpn|fra` or `eng,jpn,fra` |
| `SKIP_AUDIO_LANGUAGES` | `""` | Audio languages to skip | `eng|jpn|fra` or `eng,jpn,fra` |
| `SKIP_UNKNOWN_LANGUAGE` | `false` | Skip if language detection fails | `true` |
| `SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST` | `false` | Skip if no language but subs exist | `true` |

### Advanced Skip Logic

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `PREFERRED_AUDIO_LANGUAGES` | `""` | Preferred audio track languages | `eng|jpn|fra` |
| `LIMIT_TO_PREFERRED_AUDIO_LANGUAGE` | `false` | Only process preferred language tracks | `true` |
| `SCANNER_INITIALIZED` | `false` | Skip initial scan if already done | `true` |

**Skip Logic Flow**:
1. Check if file exists and has audio
2. Check external subtitles (`SKIP_IF_EXTERNAL_SUBTITLES_EXIST`)
3. Check target subtitles (`SKIP_IF_TARGET_SUBTITLES_EXIST`)
4. Check internal subtitles language (`SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE`)
5. Check skip language lists (`SKIP_SUBTITLE_LANGUAGES`, `SKIP_AUDIO_LANGUAGES`)
6. Check unknown language (`SKIP_UNKNOWN_LANGUAGE`)

---

## 🌐 Language Configuration

### Language Detection

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `FORCE_LANGUAGE` | `""` | Force specific language for all files | `jpn`, `eng`, `fra` |
| `PREFERRED_AUDIO_LANGUAGES` | `""` | Preferred audio track languages | `eng|jpn|fra` |
| `DETECT_LANGUAGE_LENGTH` | `30` | Seconds of audio for detection | `30` |
| `DETECT_LANGUAGE_OFFSET` | `0` | Offset before detecting language | `0` |
| `SHOULD_WHISPER_DETECT_AUDIO_LANGUAGE` | `false` | Let Whisper auto-detect language | `true` |

### MKV Audio Track Selection

**Priority Order**:
1. **Preferred Language** (`PREFERRED_AUDIO_LANGUAGES` or `FORCE_LANGUAGE`)
2. **Default Track** (marked as default in metadata)
3. **First Track** (track 0 as fallback)

**Example MKV with multiple tracks**:
```
Track 0: aac eng (default)    ← English dub
Track 1: aac jpn              ← Japanese original
Track 2: aac eng [Commentary] ← Director commentary
```

- `FORCE_LANGUAGE="jpn"` → Selects **Track 1**
- `PREFERRED_AUDIO_LANGUAGES="jpn|eng"` → Selects **Track 1** (Japanese preferred)
- No language preference → Selects **Track 0** (default track)

### Subtitle Language Naming

| Variable | Default | Description | Example Output |
|----------|---------|-------------|----------------|
| `SUBTITLE_LANGUAGE_NAMING_TYPE` | `ISO_639_2_B` | Language code format | `eng`, `jpn`, `fra` |
| `SUBTITLE_LANGUAGE_NAME` | `aa` | Custom language name | `custom` |
| `SHOW_IN_SUBNAME_SUBGEN` | `true` | Add "subgen" to filename | `movie.eng.subgen.srt` |
| `SHOW_IN_SUBNAME_MODEL` | `true` | Add model name to filename | `movie.eng.medium.srt` |

**Naming Type Options**:
- `ISO_639_1`: 2-letter codes (`en`, `ja`, `fr`)
- `ISO_639_2_T`: 3-letter terminology codes (`eng`, `jpn`, `fra`)
- `ISO_639_2_B`: 3-letter bibliographic codes (`eng`, `jpn`, `fre`)
- `NAME`: Full names (`English`, `Japanese`, `French`)
- `NATIVE`: Native names (`English`, `日本語`, `Français`)

### Multi-Language Subtitle Generation (EPIC_10)

Generate subtitles in multiple languages from a single media file. This is useful for:
- Families with members who speak different languages
- International content that needs multiple subtitle options
- Anime with Japanese audio + English/Chinese subtitles

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `TARGET_LANGUAGES` | `""` | Target output languages (comma or pipe-separated) | `eng,zho-tw` or `eng|zho-tw` |
| `TRANSCRIBE_PREFERRED` | `true` | Transcribe when audio matches preferred language | `true` |

**How It Works**:
1. Detect audio language from the media file
2. If audio language matches `PREFERRED_AUDIO_LANGUAGES` and `TRANSCRIBE_PREFERRED=true`:
   - Generate subtitle in the same language (transcribe)
3. For each language in `TARGET_LANGUAGES`:
   - If different from audio language: generate subtitle via translation
   - If same as audio language: skip (already handled by transcribe)

**Configuration Examples**:

**Example 1: Anime Library (Japanese + English + Chinese)**
```yaml
PREFERRED_AUDIO_LANGUAGES: "jpn,eng"
TARGET_LANGUAGES: "eng,zho-tw"
TRANSCRIBE_PREFERRED: "true"
```

For **Japanese audio**:
- Transcribe Japanese → `movie.jpn.subgen.medium.srt`
- Translate to English → `movie.eng.subgen.medium.srt`
- Translate to Chinese → `movie.zho-tw.subgen.medium.srt`

For **English audio**:
- Transcribe English → `movie.eng.subgen.medium.srt` (skip translate to same)

**Example 2: Foreign Films (All to English)**
```yaml
PREFERRED_AUDIO_LANGUAGES: "eng"
TARGET_LANGUAGES: "eng"
TRANSCRIBE_PREFERRED: "true"
```

For **Japanese audio**:
- Translate to English → `movie.eng.subgen.medium.srt`

For **English audio**:
- Transcribe English → `movie.eng.subgen.medium.srt`

**Example 3: Backward Compatible (No Multi-Language)**
```yaml
TARGET_LANGUAGES: ""  # Empty = single language mode
```

For any audio:
- Transcribe to detected language → `movie.{lang}.subgen.medium.srt`

**Skip Logic for Multi-Language**:
When `TARGET_LANGUAGES` is set, skip logic checks for subtitles in the **specific output language**, not just any subtitle. This allows generating multiple language subtitles independently.

> **Tip**: Both comma (`,`) and pipe (`|`) separators are supported. Comma is recommended for better YAML compatibility.

**Deprecation Notice**: `TRANSCRIBE_OR_TRANSLATE` is deprecated. Use `TARGET_LANGUAGES` instead.

---

## 📝 Subtitle Formatting

### Output Formats

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `WORD_LEVEL_HIGHLIGHT` | `false` | Karaoke-style word highlighting | `true` |
| `CUSTOM_REGROUP` | `cm_sl=84_sl=42++++++1` | Stable-TS regroup algorithm | `cm_sl=84_sl=42++++++1` |
| `LRC_FOR_AUDIO_FILES` | `true` | Generate LRC for audio files | `true` |
| `APPEND_FOOTER` | `false` | Append "Transcribed by whisperAI" footer | `true` |
| `USE_MODEL_PROMPT` | `false` | Use prompt to force punctuation | `true` |
| `CUSTOM_MODEL_PROMPT` | `""` | Custom Whisper prompt | `"Add punctuation."` |

### Filename Format

**Default format**: `{filename}.{language}.{model}.subgen.srt`

**Components**:
- `{filename}`: Original media filename
- `{language}`: Detected language code
- `{model}`: Whisper model name (if `SHOW_IN_SUBNAME_MODEL=true`)
- `subgen`: Marker (if `SHOW_IN_SUBNAME_SUBGEN=true`)
- `.srt`: Subtitle format

**Examples**:
- `movie.eng.medium.subgen.srt` (default)
- `movie.jpn.srt` (minimal)
- `movie.French.large-v3.srt` (with language name)

---

## 📁 File System Configuration

### File Monitoring

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `MONITOR` | `false` | Watch folders for file changes | `true` |
| `TRANSCRIBE_FOLDERS` | `""` | Folders to monitor | `/media/tv|/media/movies` |
| `SCANNER_INITIALIZED` | `false` | Skip initial scan if already done | `true` |
| `STABILITY_CHECKS` | `3` | Number of stability checks | `3` |
| `STABILITY_WAIT` | `5` | Seconds between stability checks | `5` |
| `STABILITY_TIMEOUT` | `30` | Timeout for stability checks | `30` |
| `BATCH_SCAN_LIMIT` | `0` | Max files to scan in batch (0=unlimited) | `1000` |

**Folder Format**: Pipe (`|`) or comma (`,`) separated
- `TRANSCRIBE_FOLDERS=/tv|/movies`
- `TRANSCRIBE_FOLDERS=/tv,/movies,/anime`

### Path Mapping

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `USE_PATH_MAPPING` | `false` | Enable path translation | `true` |
| `PATH_MAPPING_FROM` | `""` | Source path (media server's view) | `/data/media` |
| `PATH_MAPPING_TO` | `""` | Destination path (Subgen's view) | `/media` |

**Use Case**: When media server and Subgen see different paths
- **Plex sees**: `/data/media/tv/Show S01E01.mkv`
- **Subgen sees**: `/media/tv/Show S01E01.mkv`
- **Configuration**: `PATH_MAPPING_FROM=/data/media`, `PATH_MAPPING_TO=/media`

**Multiple Mappings**: Comma-separated pairs
- `PATH_MAPPING_FROM=/data/media,/backup/media`
- `PATH_MAPPING_TO=/media,/media2`

---

## 🔧 Advanced Configuration

### ASR (Bazarr) Settings

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `ASR_TIMEOUT` | `18000` | Timeout for ASR requests (5 hours) | `18000` |
| `QUEUE_MAX_AUDIO_CONTENT_SIZE` | `104857600` | Max audio upload size (100MB) | `104857600` |

**Bazarr Integration**:
- **Endpoint**: `http://your-subgen-server:9000/asr`
- **Method**: POST with audio file
- **Response**: JSON with transcription results

### Cache Directories

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `XDG_CACHE_HOME` | `/cache` | Cache directory | `/cache` |
| `HF_HOME` | `/cache/huggingface` | HuggingFace cache | `/cache/huggingface` |
| `MPLCONFIGDIR` | `/cache/matplotlib` | Matplotlib config | `/cache/matplotlib` |

### Debugging & Logging

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `DEBUG` | `false` | Enable debug mode | `true` |
| `LOG_LEVEL` | `INFO` | Logging level | `DEBUG`, `INFO`, `WARNING`, `ERROR` |
| `LOG_FORMAT` | `json` | Log format | `json`, `text` |

### Container User/Group

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `PUID` | `99` | User ID for rootless Docker | `1000` |
| `PGID` | `100` | Group ID for rootless Docker | `1000` |

---

## 🎯 Configuration Examples

### Example 1: Basic Plex Setup

```yaml
# Orchestrator
WORKER_ADDRESS: "worker:50051"
WEBHOOK_PORT: "9000"
PLEX_ENABLED: "true"
PLEX_SERVER: "http://plex:32400"
PLEX_TOKEN: "your-plex-token"
PROCESS_ADDED_MEDIA: "true"
PROCESS_MEDIA_ON_PLAY: "false"
SKIP_IF_TARGET_SUBTITLES_EXIST: "true"

# Worker
TRANSCRIBE_DEVICE: "cuda"
WHISPER_MODEL: "medium"
CONCURRENT_TRANSCRIPTIONS: "2"
MODEL_CLEANUP_DELAY: "300"
```

### Example 2: High-Performance GPU Setup

```yaml
# Orchestrator
WORKER_ADDRESS: "worker:50051"
WEBHOOK_PORT: "9000"
PLEX_ENABLED: "true"
JELLYFIN_ENABLED: "true"
MONITOR: "true"
TRANSCRIBE_FOLDERS: "/media/tv|/media/movies"

# Worker (24GB VRAM GPU)
TRANSCRIBE_DEVICE: "cuda"
WHISPER_MODEL: "large-v3"
CONCURRENT_TRANSCRIPTIONS: "4"
WHISPER_THREADS: "8"
COMPUTE_TYPE: "float16"
MODEL_CLEANUP_DELAY: "600"
CLEAR_VRAM_ON_COMPLETE: "true"
MAX_WORKERS: "10"
```

### Example 3: CPU-Only Multi-Language Setup

```yaml
# Orchestrator
WORKER_ADDRESS: "worker:50051"
WEBHOOK_PORT: "9000"
PLEX_ENABLED: "true"
PROCESS_ADDED_MEDIA: "true"
SKIP_IF_TARGET_SUBTITLES_EXIST: "true"
SKIP_SUBTITLE_LANGUAGES: "eng|spa|fra"
PREFERRED_AUDIO_LANGUAGES: "jpn|eng|kor"
FORCE_LANGUAGE: ""  # Auto-detect

# Worker (8-core CPU)
TRANSCRIBE_DEVICE: "cpu"
WHISPER_MODEL: "small"
CONCURRENT_TRANSCRIPTIONS: "2"
WHISPER_THREADS: "4"
MODEL_CLEANUP_DELAY: "1800"  # Keep loaded for 30 minutes
MAX_WORKERS: "6"
```

### Example 4: Selective Processing with Path Mapping

```yaml
# Orchestrator
WORKER_ADDRESS: "worker:50051"
WEBHOOK_PORT: "9000"
PLEX_ENABLED: "true"
USE_PATH_MAPPING: "true"
PATH_MAPPING_FROM: "/data/media"
PATH_MAPPING_TO: "/media"
MONITOR: "true"
TRANSCRIBE_FOLDERS: "/media/anime|/media/foreign"
SKIP_IF_EXTERNAL_SUBTITLES_EXIST: "true"
SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE: "eng"
PLEX_QUEUE_NEXT_EPISODE: "true"

# Worker
TRANSCRIBE_DEVICE: "cuda"
WHISPER_MODEL: "medium"
CONCURRENT_TRANSCRIPTIONS: "2"
```

---

## 🔍 Environment Variable Reference

### Legacy Compatibility

| Legacy Variable | New Variable | Notes |
|-----------------|--------------|-------|
| `PLEXTOKEN` | `PLEX_TOKEN` | Both work, new preferred |
| `PLEXSERVER` | `PLEX_SERVER` | Both work, new preferred |
| `PROCADDEDMEDIA` | `PROCESS_ADDED_MEDIA` | Both work, new preferred |
| `PROCMEDIAONPLAY` | `PROCESS_MEDIA_ON_PLAY` | Both work, new preferred |
| `SKIPIFEXTERNALSUB` | `SKIP_IF_EXTERNAL_SUBTITLES_EXIST` | Both work, new preferred |
| `SKIP_LANG_CODES` | `SKIP_SUBTITLE_LANGUAGES` | Both work, new preferred |
| `NAMESUBLANG` | `SUBTITLE_LANGUAGE_NAME` | Both work, new preferred |

### Variable Priority (Highest to Lowest)

1. **Environment variables** (set in shell or Docker)
2. **.env file** (in working directory)
3. **YAML config file** (if using `load_config()`)
4. **Default values** (in code)

### Nested Configuration

For complex configurations, use nested syntax with `__`:

```bash
# Instead of:
WHISPER_MODEL=medium
TRANSCRIBE_DEVICE=cuda

# You can use:
WHISPER__MODEL=medium
WHISPER__DEVICE=cuda
```

---

## 🛠️ Configuration Validation

### Common Validation Rules

| Variable | Validation | Error Message |
|----------|------------|---------------|
| `PLEX_SERVER`, `JELLYFIN_SERVER` | Must start with `http://` or `https://` | "Server URL must start with http:// or https://" |
| `WHISPER_MODEL` | Must be valid model name | "Invalid Whisper model" |
| `TRANSCRIBE_DEVICE` | Must be `cpu` or `cuda` | "Invalid device" |
| `WHISPER_THREADS` | 1-32 range | "CPU threads must be between 1 and 32" |
| `GRPC_PORT`, `HTTP_PORT` | 1024-65535 range | "Port must be between 1024 and 65535" |

### Configuration Testing

```bash
# Test configuration loading
cd worker
python -c "from config.settings import get_settings; print(get_settings().model_dump_json(indent=2))"

# Check environment variables
docker exec subgen-orchestrator env | grep -E "(PLEX|JELLYFIN|WHISPER|SKIP)"

# Validate with test script
./test/validate_config.sh
```

---

## 📚 Additional Resources

- **README.md**: User-facing documentation with setup examples
- **README-LLM.md**: Developer documentation with architecture details
- **MIGRATION.md**: Migration guide from legacy to split architecture
- **deploy-working.yaml**: Production Kubernetes configuration example
- **docker-compose.yml**: Docker Compose example configuration

### Getting Help

- **Configuration Issues**: Check logs for validation errors
- **Performance Tuning**: Adjust `CONCURRENT_TRANSCRIPTIONS` and `WHISPER_THREADS`
- **Memory Issues**: Reduce `MODEL_CLEANUP_DELAY` or use smaller model
- **Path Issues**: Enable `USE_PATH_MAPPING` and verify paths match

---

**Last Updated**: 2026-02-21  
**Version**: v0.2.18  
**Status**: Production Ready ✅