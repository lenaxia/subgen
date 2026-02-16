# Story 05: Configuration & Error Handling

**Epic**: EPIC_02 - Python Worker Refactor  
**Status**: Not Started  
**Priority**: High  
**Estimated Effort**: 6-8 hours  
**Assignee**: TBD

---

## User Story

As a **DevOps engineer deploying Subgen**,  
I want **centralized configuration management with validation and clear error messages**,  
So that **misconfigurations are caught early and debugging is straightforward**.

---

## Background

The legacy `subgen.py` has **40+ configuration variables** loaded from environment variables with:
- Inconsistent naming (old vs new names for backwards compatibility)
- No validation (invalid values crash at runtime)
- Scattered across lines 77-186
- Type conversions inline (error-prone)
- No config file support

**Problems in Production**:
1. User sets `WHISPER_THREADS=abc` → crashes on first transcription
2. User forgets `PLEX_TOKEN` → vague error "401 Unauthorized"
3. Invalid `COMPUTE_TYPE` → model loading fails with cryptic error
4. Wrong `PATH_MAPPING_FROM` → files not found with no hint why

**This story creates**:
- Pydantic-based configuration with validation
- Clear error messages for invalid config
- Config file support (.env and YAML)
- Backwards compatibility with legacy names
- Comprehensive error handling patterns

---

## Acceptance Criteria

- [ ] `worker/config/settings.py` created with pydantic Config class
- [ ] All 40+ env variables migrated to Config
- [ ] Validation for all fields (min/max, enum values)
- [ ] Backwards compatibility for legacy env names
- [ ] Clear error messages for invalid config
- [ ] Support for .env file
- [ ] Support for YAML config file (optional)
- [ ] Environment-specific configs (dev, prod)
- [ ] Custom exceptions for configuration errors
- [ ] Error handling module (`worker/utils/errors.py`)
- [ ] Unit tests for config validation (10+ tests)
- [ ] Integration test loading config from file
- [ ] Documentation of all config options
- [ ] Work log created

---

## Legacy Configuration Analysis

### Configuration Variables (Lines 77-186)

**Location**: `subgen.py:77-186`

**Current Structure**:
```python
# Helper functions (lines 77-104)
def convert_to_bool(in_bool): ...
def get_env_with_fallback(new_name, old_name, default_value, convert_func): ...

# Server Integration (lines 106-110)
plextoken = get_env_with_fallback('PLEX_TOKEN', 'PLEXTOKEN', 'token here')
plexserver = get_env_with_fallback('PLEX_SERVER', 'PLEXSERVER', 'http://192.168.1.111:32400')
jellyfintoken = get_env_with_fallback('JELLYFIN_TOKEN', 'JELLYFINTOKEN', 'token here')
jellyfinserver = get_env_with_fallback('JELLYFIN_SERVER', 'JELLYFINSERVER', 'http://192.168.1.111:8096')

# Whisper Configuration (lines 112-116)
whisper_model = os.getenv('WHISPER_MODEL', 'medium')
whisper_threads = int(os.getenv('WHISPER_THREADS', 4))
concurrent_transcriptions = int(os.getenv('CONCURRENT_TRANSCRIPTIONS', 2))
transcribe_device = os.getenv('TRANSCRIBE_DEVICE', 'cpu')

# Processing Control (lines 118-120)
procaddedmedia = get_env_with_fallback('PROCESS_ADDED_MEDIA', 'PROCADDEDMEDIA', True, convert_to_bool)
procmediaonplay = get_env_with_fallback('PROCESS_MEDIA_ON_PLAY', 'PROCMEDIAONPLAY', True, convert_to_bool)

# System Configuration (lines 122-145)
webhookport = get_env_with_fallback('WEBHOOK_PORT', 'WEBHOOKPORT', 9000, int)
word_level_highlight = convert_to_bool(os.getenv('WORD_LEVEL_HIGHLIGHT', False))
debug = convert_to_bool(os.getenv('DEBUG', True))
use_path_mapping = convert_to_bool(os.getenv('USE_PATH_MAPPING', False))
path_mapping_from = os.getenv('PATH_MAPPING_FROM', r'/tv')
path_mapping_to = os.getenv('PATH_MAPPING_TO', r'/Volumes/TV')
model_location = os.getenv('MODEL_PATH', './models')
monitor = convert_to_bool(os.getenv('MONITOR', False))
transcribe_folders = os.getenv('TRANSCRIBE_FOLDERS', '')
transcribe_or_translate = os.getenv('TRANSCRIBE_OR_TRANSLATE', 'transcribe').lower()
clear_vram_on_complete = convert_to_bool(os.getenv('CLEAR_VRAM_ON_COMPLETE', True))
compute_type = os.getenv('COMPUTE_TYPE', 'auto')
append = convert_to_bool(os.getenv('APPEND', False))
reload_script_on_change = convert_to_bool(os.getenv('RELOAD_SCRIPT_ON_CHANGE', False))
lrc_for_audio_files = convert_to_bool(os.getenv('LRC_FOR_AUDIO_FILES', True))
custom_regroup = os.getenv('CUSTOM_REGROUP', 'cm_sl=84_sl=42++++++1')
detect_language_length = int(os.getenv('DETECT_LANGUAGE_LENGTH', 30))
detect_language_offset = int(os.getenv('DETECT_LANGUAGE_OFFSET', 0))
model_cleanup_delay = int(os.getenv('MODEL_CLEANUP_DELAY', 30))
asr_timeout = int(os.getenv('ASR_TIMEOUT', 18000))

# Skip Configuration (lines 147-176)
skipifexternalsub = get_env_with_fallback('SKIP_IF_EXTERNAL_SUBTITLES_EXIST', 'SKIPIFEXTERNALSUB', False, convert_to_bool)
skip_if_to_transcribe_sub_already_exist = get_env_with_fallback('SKIP_IF_TARGET_SUBTITLES_EXIST', 'SKIP_IF_TO_TRANSCRIBE_SUB_ALREADY_EXIST', True, convert_to_bool)
# ... 10+ more skip options
```

### Configuration Categories

**1. Server Integration** (4 variables):
- `PLEX_TOKEN`, `PLEX_SERVER`
- `JELLYFIN_TOKEN`, `JELLYFIN_SERVER`

**2. Whisper Model** (4 variables):
- `WHISPER_MODEL`: Model size (tiny, base, small, medium, large, large-v2, large-v3)
- `WHISPER_THREADS`: CPU threads for transcription
- `CONCURRENT_TRANSCRIPTIONS`: Number of parallel workers
- `TRANSCRIBE_DEVICE`: cpu or cuda

**3. Processing Control** (2 variables):
- `PROCESS_ADDED_MEDIA`: Process when media added
- `PROCESS_MEDIA_ON_PLAY`: Process when media played

**4. System** (20 variables):
- Ports, paths, debugging, monitoring
- Path mapping for Docker
- Model location
- VRAM management
- Timeouts

**5. Subtitle Options** (10+ variables):
- Language naming formats
- LRC for audio files
- Word-level timestamps
- Custom regrouping

**6. Skip Logic** (10+ variables):
- Skip if subtitles exist
- Skip specific languages
- Skip specific audio tracks

**Total**: 40+ configuration variables

---

## Technical Design

### Configuration System

**File: `worker/config/settings.py`**

```python
"""
Centralized configuration management with validation.

Supports:
- Environment variables
- .env files
- YAML config files
- Backwards compatibility with legacy names
- Validation with clear error messages
"""

import os
from typing import Optional, List, Literal
from pathlib import Path
from pydantic import BaseSettings, Field, validator, ValidationError
from pydantic.env_settings import SettingsSourceCallable


class ServerConfig(BaseSettings):
    """Server integration configuration."""
    
    # Plex
    plex_token: str = Field(
        default="",
        description="Plex authentication token"
    )
    plex_server: str = Field(
        default="http://192.168.1.111:32400",
        description="Plex server URL"
    )
    
    # Jellyfin
    jellyfin_token: str = Field(
        default="",
        description="Jellyfin authentication token"
    )
    jellyfin_server: str = Field(
        default="http://192.168.1.111:8096",
        description="Jellyfin server URL"
    )
    
    @validator('plex_server', 'jellyfin_server')
    def validate_server_url(cls, v):
        """Validate server URL format."""
        if v and not v.startswith(('http://', 'https://')):
            raise ValueError("Server URL must start with http:// or https://")
        return v
    
    class Config:
        env_file = ".env"
        # Backwards compatibility
        fields = {
            'plex_token': {'env': ['PLEX_TOKEN', 'PLEXTOKEN']},
            'plex_server': {'env': ['PLEX_SERVER', 'PLEXSERVER']},
            'jellyfin_token': {'env': ['JELLYFIN_TOKEN', 'JELLYFINTOKEN']},
            'jellyfin_server': {'env': ['JELLYFIN_SERVER', 'JELLYFINSERVER']},
        }


class WhisperConfig(BaseSettings):
    """Whisper model configuration."""
    
    model_name: Literal['tiny', 'base', 'small', 'medium', 'large', 'large-v2', 'large-v3'] = Field(
        default='medium',
        description="Whisper model size"
    )
    
    model_path: Path = Field(
        default=Path('./models'),
        description="Directory to store model files"
    )
    
    device: Literal['cpu', 'cuda'] = Field(
        default='cpu',
        description="Device for transcription"
    )
    
    cpu_threads: int = Field(
        default=4,
        ge=1,
        le=32,
        description="CPU threads for transcription"
    )
    
    concurrent_transcriptions: int = Field(
        default=2,
        ge=1,
        le=10,
        description="Number of parallel transcriptions"
    )
    
    compute_type: Literal['auto', 'int8', 'float16', 'float32'] = Field(
        default='auto',
        description="Compute type for model"
    )
    
    @validator('device')
    def validate_device(cls, v):
        """Check if CUDA is available when device is cuda."""
        if v == 'cuda':
            try:
                import torch
                if not torch.cuda.is_available():
                    raise ValueError("CUDA device specified but CUDA is not available")
            except ImportError:
                raise ValueError("PyTorch is required for CUDA support")
        return v
    
    @validator('model_path')
    def create_model_path(cls, v):
        """Create model directory if it doesn't exist."""
        v = Path(v)
        v.mkdir(parents=True, exist_ok=True)
        return v
    
    class Config:
        env_file = ".env"
        fields = {
            'model_name': {'env': 'WHISPER_MODEL'},
            'model_path': {'env': 'MODEL_PATH'},
            'device': {'env': 'TRANSCRIBE_DEVICE'},
            'cpu_threads': {'env': 'WHISPER_THREADS'},
        }


class ProcessingConfig(BaseSettings):
    """Media processing control."""
    
    process_added_media: bool = Field(
        default=True,
        description="Process media when added to library"
    )
    
    process_media_on_play: bool = Field(
        default=True,
        description="Process media when played"
    )
    
    monitor_folders: List[str] = Field(
        default_factory=list,
        description="Folders to monitor for new media"
    )
    
    @validator('monitor_folders', pre=True)
    def parse_folders(cls, v):
        """Parse comma-separated folder list."""
        if isinstance(v, str):
            return [f.strip() for f in v.split(',') if f.strip()]
        return v
    
    class Config:
        env_file = ".env"
        fields = {
            'process_added_media': {'env': ['PROCESS_ADDED_MEDIA', 'PROCADDEDMEDIA']},
            'process_media_on_play': {'env': ['PROCESS_MEDIA_ON_PLAY', 'PROCMEDIAONPLAY']},
            'monitor_folders': {'env': 'TRANSCRIBE_FOLDERS'},
        }


class SystemConfig(BaseSettings):
    """System-level configuration."""
    
    grpc_port: int = Field(
        default=50051,
        ge=1024,
        le=65535,
        description="gRPC server port"
    )
    
    max_workers: int = Field(
        default=4,
        ge=1,
        le=32,
        description="Max gRPC worker threads"
    )
    
    memory_threshold_mb: int = Field(
        default=3000,
        ge=512,
        description="Memory threshold for health checks (MB)"
    )
    
    log_level: Literal['DEBUG', 'INFO', 'WARNING', 'ERROR'] = Field(
        default='INFO',
        description="Logging level"
    )
    
    debug: bool = Field(
        default=False,
        description="Enable debug mode"
    )
    
    # Path mapping for Docker
    use_path_mapping: bool = Field(
        default=False,
        description="Enable path mapping"
    )
    
    path_mapping_from: str = Field(
        default='/tv',
        description="Source path for mapping"
    )
    
    path_mapping_to: str = Field(
        default='/Volumes/TV',
        description="Destination path for mapping"
    )
    
    class Config:
        env_file = ".env"


class TranscriptionConfig(BaseSettings):
    """Transcription options."""
    
    task: Literal['transcribe', 'translate'] = Field(
        default='transcribe',
        description="Task type"
    )
    
    word_level_highlight: bool = Field(
        default=False,
        description="Enable word-level timestamps"
    )
    
    custom_regroup: str = Field(
        default='cm_sl=84_sl=42++++++1',
        description="Custom regrouping algorithm"
    )
    
    lrc_for_audio_files: bool = Field(
        default=True,
        description="Generate LRC files for audio"
    )
    
    detect_language_length: int = Field(
        default=30,
        ge=1,
        le=300,
        description="Audio length for language detection (seconds)"
    )
    
    detect_language_offset: int = Field(
        default=0,
        ge=0,
        description="Start offset for language detection (seconds)"
    )
    
    asr_timeout: int = Field(
        default=18000,
        ge=60,
        description="ASR request timeout (seconds)"
    )
    
    class Config:
        env_file = ".env"
        fields = {
            'task': {'env': 'TRANSCRIBE_OR_TRANSLATE'},
        }


class SubtitleConfig(BaseSettings):
    """Subtitle generation options."""
    
    language_naming_type: Literal[
        'ISO_639_1',
        'ISO_639_2_T',
        'ISO_639_2_B',
        'NAME',
        'NATIVE'
    ] = Field(
        default='ISO_639_2_B',
        description="Language code format for filenames"
    )
    
    show_subgen_in_filename: bool = Field(
        default=True,
        description="Include 'subgen' in filename"
    )
    
    show_model_in_filename: bool = Field(
        default=True,
        description="Include model name in filename"
    )
    
    custom_language_name: str = Field(
        default='',
        description="Override language name in filename"
    )
    
    append_footer: bool = Field(
        default=False,
        description="Append generation info to subtitles"
    )
    
    class Config:
        env_file = ".env"
        fields = {
            'custom_language_name': {'env': ['SUBTITLE_LANGUAGE_NAME', 'NAMESUBLANG']},
        }


class SkipConfig(BaseSettings):
    """Skip logic configuration."""
    
    skip_if_external_subtitles_exist: bool = Field(
        default=False,
        description="Skip if external subtitles exist"
    )
    
    skip_if_target_subtitles_exist: bool = Field(
        default=True,
        description="Skip if target language subtitles exist"
    )
    
    skip_subtitle_languages: List[str] = Field(
        default_factory=list,
        description="Skip these subtitle languages"
    )
    
    skip_audio_languages: List[str] = Field(
        default_factory=list,
        description="Skip these audio languages"
    )
    
    skip_only_subgen_subtitles: bool = Field(
        default=False,
        description="Only skip if subtitles created by subgen"
    )
    
    @validator('skip_subtitle_languages', 'skip_audio_languages', pre=True)
    def parse_language_list(cls, v):
        """Parse pipe-separated language list."""
        if isinstance(v, str):
            return [lang.strip() for lang in v.split('|') if lang.strip()]
        return v
    
    class Config:
        env_file = ".env"
        fields = {
            'skip_if_external_subtitles_exist': {
                'env': ['SKIP_IF_EXTERNAL_SUBTITLES_EXIST', 'SKIPIFEXTERNALSUB']
            },
            'skip_subtitle_languages': {
                'env': ['SKIP_SUBTITLE_LANGUAGES', 'SKIP_LANG_CODES']
            },
        }


class ModelLifecycleConfig(BaseSettings):
    """Model lifecycle management."""
    
    cleanup_delay: int = Field(
        default=30,
        ge=0,
        description="Model cleanup delay (seconds)"
    )
    
    clear_vram_on_complete: bool = Field(
        default=True,
        description="Clear VRAM after completion"
    )
    
    class Config:
        env_file = ".env"
        fields = {
            'cleanup_delay': {'env': 'MODEL_CLEANUP_DELAY'},
        }


class Config(BaseSettings):
    """
    Master configuration combining all sub-configs.
    
    Usage:
        config = Config()
        print(config.whisper.model_name)
        print(config.system.grpc_port)
    """
    
    # Sub-configurations
    server: ServerConfig = Field(default_factory=ServerConfig)
    whisper: WhisperConfig = Field(default_factory=WhisperConfig)
    processing: ProcessingConfig = Field(default_factory=ProcessingConfig)
    system: SystemConfig = Field(default_factory=SystemConfig)
    transcription: TranscriptionConfig = Field(default_factory=TranscriptionConfig)
    subtitle: SubtitleConfig = Field(default_factory=SubtitleConfig)
    skip: SkipConfig = Field(default_factory=SkipConfig)
    model_lifecycle: ModelLifecycleConfig = Field(default_factory=ModelLifecycleConfig)
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        
    @classmethod
    def load_from_yaml(cls, path: Path) -> 'Config':
        """Load configuration from YAML file."""
        import yaml
        
        with open(path) as f:
            data = yaml.safe_load(f)
        
        return cls(**data)
    
    def to_yaml(self, path: Path) -> None:
        """Save configuration to YAML file."""
        import yaml
        
        data = self.dict()
        
        with open(path, 'w') as f:
            yaml.dump(data, f, default_flow_style=False)


def load_config(
    env_file: Optional[Path] = None,
    yaml_file: Optional[Path] = None
) -> Config:
    """
    Load configuration with validation.
    
    Args:
        env_file: Optional .env file path
        yaml_file: Optional YAML config file path
        
    Returns:
        Validated Config object
        
    Raises:
        ConfigurationError: If configuration is invalid
    """
    try:
        if yaml_file and yaml_file.exists():
            config = Config.load_from_yaml(yaml_file)
        else:
            config = Config(_env_file=env_file)
        
        return config
        
    except ValidationError as e:
        # Convert pydantic error to friendly message
        errors = []
        for error in e.errors():
            field = '.'.join(str(loc) for loc in error['loc'])
            msg = error['msg']
            errors.append(f"  - {field}: {msg}")
        
        error_msg = "Configuration validation failed:\n" + "\n".join(errors)
        
        raise ConfigurationError(error_msg)
```

### Error Handling

**File: `worker/utils/errors.py`**

```python
"""Custom exception classes for worker."""


class WorkerError(Exception):
    """Base exception for worker errors."""
    pass


class ConfigurationError(WorkerError):
    """Raised when configuration is invalid."""
    pass


class ModelLoadError(WorkerError):
    """Raised when model fails to load."""
    pass


class TranscriptionError(WorkerError):
    """Raised when transcription fails."""
    pass


class AudioExtractionError(WorkerError):
    """Raised when audio extraction fails."""
    pass


class LanguageDetectionError(WorkerError):
    """Raised when language detection fails."""
    pass


class SubtitleGenerationError(WorkerError):
    """Raised when subtitle generation fails."""
    pass
```

---

## Testing Strategy

### Configuration Tests

**File: `worker/tests/unit/test_config.py`**

```python
"""Unit tests for configuration."""

import pytest
from pathlib import Path
from pydantic import ValidationError
from config.settings import (
    Config,
    WhisperConfig,
    SystemConfig,
    load_config
)
from utils.errors import ConfigurationError


def test_default_config():
    """Test default configuration loads."""
    config = Config()
    
    assert config.whisper.model_name == 'medium'
    assert config.system.grpc_port == 50051
    assert config.transcription.task == 'transcribe'


def test_whisper_config_validation():
    """Test Whisper config validates model name."""
    # Valid model
    config = WhisperConfig(model_name='tiny')
    assert config.model_name == 'tiny'
    
    # Invalid model
    with pytest.raises(ValidationError) as exc_info:
        WhisperConfig(model_name='invalid')
    
    assert 'model_name' in str(exc_info.value)


def test_system_config_port_validation():
    """Test port number validation."""
    # Valid port
    config = SystemConfig(grpc_port=8080)
    assert config.grpc_port == 8080
    
    # Port too low
    with pytest.raises(ValidationError):
        SystemConfig(grpc_port=100)
    
    # Port too high
    with pytest.raises(ValidationError):
        SystemConfig(grpc_port=99999)


def test_whisper_threads_validation():
    """Test CPU threads validation."""
    # Valid threads
    config = WhisperConfig(cpu_threads=8)
    assert config.cpu_threads == 8
    
    # Too few threads
    with pytest.raises(ValidationError):
        WhisperConfig(cpu_threads=0)
    
    # Too many threads
    with pytest.raises(ValidationError):
        WhisperConfig(cpu_threads=100)


def test_device_cuda_validation():
    """Test CUDA device validation."""
    # Should work (or raise clear error)
    try:
        config = WhisperConfig(device='cuda')
        # If we get here, CUDA is available
        assert config.device == 'cuda'
    except ValidationError as e:
        # CUDA not available, should have clear message
        assert 'CUDA' in str(e)


def test_backwards_compatibility(monkeypatch):
    """Test legacy environment variable names."""
    # Set legacy names
    monkeypatch.setenv('PLEXTOKEN', 'test-token')
    monkeypatch.setenv('WEBHOOKPORT', '8080')
    
    config = Config()
    
    # Should load from legacy names
    assert config.server.plex_token == 'test-token'
    assert config.system.webhook_port == 8080


def test_new_names_override_legacy(monkeypatch):
    """Test new names take precedence."""
    monkeypatch.setenv('PLEXTOKEN', 'old-token')
    monkeypatch.setenv('PLEX_TOKEN', 'new-token')
    
    config = Config()
    
    # New name should win
    assert config.server.plex_token == 'new-token'


def test_load_from_env_file(tmp_path):
    """Test loading config from .env file."""
    env_file = tmp_path / ".env"
    env_file.write_text("""
WHISPER_MODEL=small
TRANSCRIBE_DEVICE=cpu
GRPC_PORT=9090
    """)
    
    config = load_config(env_file=env_file)
    
    assert config.whisper.model_name == 'small'
    assert config.whisper.device == 'cpu'
    assert config.system.grpc_port == 9090


def test_load_from_yaml(tmp_path):
    """Test loading config from YAML."""
    yaml_file = tmp_path / "config.yaml"
    yaml_file.write_text("""
whisper:
  model_name: tiny
  device: cpu
system:
  grpc_port: 7070
    """)
    
    config = load_config(yaml_file=yaml_file)
    
    assert config.whisper.model_name == 'tiny'
    assert config.system.grpc_port == 7070


def test_invalid_config_clear_error():
    """Test invalid config produces clear error message."""
    with pytest.raises(ConfigurationError) as exc_info:
        config = Config()
        config.whisper.model_name = 'invalid-model'
        config.validate()
    
    error_msg = str(exc_info.value)
    assert 'model_name' in error_msg
    assert 'invalid' in error_msg.lower()


def test_model_path_created():
    """Test model path is created if missing."""
    import tempfile
    import shutil
    
    temp_dir = tempfile.mkdtemp()
    model_path = Path(temp_dir) / "models" / "subdir"
    
    config = WhisperConfig(model_path=str(model_path))
    
    # Should create directory
    assert model_path.exists()
    assert model_path.is_dir()
    
    # Cleanup
    shutil.rmtree(temp_dir)
```

---

## Definition of Done

- [ ] All acceptance criteria met
- [ ] All 40+ config variables migrated to Pydantic
- [ ] Validation working for all fields
- [ ] Backwards compatibility preserved
- [ ] Error handling module created
- [ ] Unit tests passing (10+ tests)
- [ ] Integration tests passing
- [ ] Type hints throughout (mypy --strict passes)
- [ ] Documentation of all config options
- [ ] Work log created: `docs/WORKLOGS/NNNN_YYYY-MM-DD_EPIC_02_story_05_configuration.md`
- [ ] Code committed and pushed

---

## Validation Commands

```bash
# Run config tests
cd worker
pytest tests/unit/test_config.py -v

# Test loading from env file
echo "WHISPER_MODEL=small" > test.env
python -c "
from config.settings import load_config
from pathlib import Path
config = load_config(env_file=Path('test.env'))
print('Model:', config.whisper.model_name)
"

# Test validation
python -c "
from config.settings import WhisperConfig
try:
    config = WhisperConfig(model_name='invalid')
except Exception as e:
    print('Validation error:', e)
"

# Type checking
mypy config/settings.py --strict
```

---

## Dependencies

**Requires:**
- STORY_01 (gRPC Server) - needs config for server
- STORY_02 (Modular Refactor) - needs config for engine

**Blocks:**
- None (can be deployed independently)

---

## References

- Legacy code: `subgen.py:77-186` (configuration loading)
- Pydantic docs: https://docs.pydantic.dev/
- Pydantic settings: https://docs.pydantic.dev/latest/concepts/pydantic_settings/
- Python-dotenv: https://github.com/theskumar/python-dotenv

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15
