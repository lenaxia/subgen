from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class TranscribeRequest(_message.Message):
    __slots__ = ("file_path", "task_type", "force_language", "options", "metadata")
    class MetadataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    TASK_TYPE_FIELD_NUMBER: _ClassVar[int]
    FORCE_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    file_path: str
    task_type: str
    force_language: str
    options: TranscribeOptions
    metadata: _containers.ScalarMap[str, str]
    def __init__(self, file_path: _Optional[str] = ..., task_type: _Optional[str] = ..., force_language: _Optional[str] = ..., options: _Optional[_Union[TranscribeOptions, _Mapping]] = ..., metadata: _Optional[_Mapping[str, str]] = ...) -> None: ...

class TranscribeOptions(_message.Message):
    __slots__ = ("whisper_model", "whisper_threads", "word_level_highlight", "custom_regroup", "lrc_for_audio", "custom_prompt", "append_footer", "subtitle_language_name", "show_model_in_filename", "show_subgen_in_filename")
    WHISPER_MODEL_FIELD_NUMBER: _ClassVar[int]
    WHISPER_THREADS_FIELD_NUMBER: _ClassVar[int]
    WORD_LEVEL_HIGHLIGHT_FIELD_NUMBER: _ClassVar[int]
    CUSTOM_REGROUP_FIELD_NUMBER: _ClassVar[int]
    LRC_FOR_AUDIO_FIELD_NUMBER: _ClassVar[int]
    CUSTOM_PROMPT_FIELD_NUMBER: _ClassVar[int]
    APPEND_FOOTER_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_LANGUAGE_NAME_FIELD_NUMBER: _ClassVar[int]
    SHOW_MODEL_IN_FILENAME_FIELD_NUMBER: _ClassVar[int]
    SHOW_SUBGEN_IN_FILENAME_FIELD_NUMBER: _ClassVar[int]
    whisper_model: str
    whisper_threads: int
    word_level_highlight: bool
    custom_regroup: str
    lrc_for_audio: bool
    custom_prompt: str
    append_footer: bool
    subtitle_language_name: str
    show_model_in_filename: bool
    show_subgen_in_filename: bool
    def __init__(self, whisper_model: _Optional[str] = ..., whisper_threads: _Optional[int] = ..., word_level_highlight: bool = ..., custom_regroup: _Optional[str] = ..., lrc_for_audio: bool = ..., custom_prompt: _Optional[str] = ..., append_footer: bool = ..., subtitle_language_name: _Optional[str] = ..., show_model_in_filename: bool = ..., show_subgen_in_filename: bool = ...) -> None: ...

class TranscribeResponse(_message.Message):
    __slots__ = ("success", "subtitle_path", "detected_language", "error_message", "stats")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    SUBTITLE_PATH_FIELD_NUMBER: _ClassVar[int]
    DETECTED_LANGUAGE_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    STATS_FIELD_NUMBER: _ClassVar[int]
    success: bool
    subtitle_path: str
    detected_language: str
    error_message: str
    stats: TranscriptionStats
    def __init__(self, success: bool = ..., subtitle_path: _Optional[str] = ..., detected_language: _Optional[str] = ..., error_message: _Optional[str] = ..., stats: _Optional[_Union[TranscriptionStats, _Mapping]] = ...) -> None: ...

class TranscriptionStats(_message.Message):
    __slots__ = ("duration_seconds", "segment_count", "model_load_time_ms", "transcription_time_ms", "peak_memory_mb")
    DURATION_SECONDS_FIELD_NUMBER: _ClassVar[int]
    SEGMENT_COUNT_FIELD_NUMBER: _ClassVar[int]
    MODEL_LOAD_TIME_MS_FIELD_NUMBER: _ClassVar[int]
    TRANSCRIPTION_TIME_MS_FIELD_NUMBER: _ClassVar[int]
    PEAK_MEMORY_MB_FIELD_NUMBER: _ClassVar[int]
    duration_seconds: float
    segment_count: int
    model_load_time_ms: int
    transcription_time_ms: int
    peak_memory_mb: int
    def __init__(self, duration_seconds: _Optional[float] = ..., segment_count: _Optional[int] = ..., model_load_time_ms: _Optional[int] = ..., transcription_time_ms: _Optional[int] = ..., peak_memory_mb: _Optional[int] = ...) -> None: ...

class DetectLanguageRequest(_message.Message):
    __slots__ = ("file_path", "audio_content", "sample_length", "sample_offset")
    FILE_PATH_FIELD_NUMBER: _ClassVar[int]
    AUDIO_CONTENT_FIELD_NUMBER: _ClassVar[int]
    SAMPLE_LENGTH_FIELD_NUMBER: _ClassVar[int]
    SAMPLE_OFFSET_FIELD_NUMBER: _ClassVar[int]
    file_path: str
    audio_content: bytes
    sample_length: int
    sample_offset: int
    def __init__(self, file_path: _Optional[str] = ..., audio_content: _Optional[bytes] = ..., sample_length: _Optional[int] = ..., sample_offset: _Optional[int] = ...) -> None: ...

class DetectLanguageResponse(_message.Message):
    __slots__ = ("success", "language_code", "language_name", "confidence", "error_message")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_CODE_FIELD_NUMBER: _ClassVar[int]
    LANGUAGE_NAME_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    success: bool
    language_code: str
    language_name: str
    confidence: float
    error_message: str
    def __init__(self, success: bool = ..., language_code: _Optional[str] = ..., language_name: _Optional[str] = ..., confidence: _Optional[float] = ..., error_message: _Optional[str] = ...) -> None: ...

class HealthCheckRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HealthCheckResponse(_message.Message):
    __slots__ = ("status", "memory_mb", "model_loaded", "jobs_processed", "jobs_active", "version", "uptime_seconds")
    class Status(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
        __slots__ = ()
        UNKNOWN: _ClassVar[HealthCheckResponse.Status]
        HEALTHY: _ClassVar[HealthCheckResponse.Status]
        UNHEALTHY: _ClassVar[HealthCheckResponse.Status]
        STARTING: _ClassVar[HealthCheckResponse.Status]
    UNKNOWN: HealthCheckResponse.Status
    HEALTHY: HealthCheckResponse.Status
    UNHEALTHY: HealthCheckResponse.Status
    STARTING: HealthCheckResponse.Status
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MEMORY_MB_FIELD_NUMBER: _ClassVar[int]
    MODEL_LOADED_FIELD_NUMBER: _ClassVar[int]
    JOBS_PROCESSED_FIELD_NUMBER: _ClassVar[int]
    JOBS_ACTIVE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    UPTIME_SECONDS_FIELD_NUMBER: _ClassVar[int]
    status: HealthCheckResponse.Status
    memory_mb: int
    model_loaded: bool
    jobs_processed: int
    jobs_active: int
    version: str
    uptime_seconds: int
    def __init__(self, status: _Optional[_Union[HealthCheckResponse.Status, str]] = ..., memory_mb: _Optional[int] = ..., model_loaded: bool = ..., jobs_processed: _Optional[int] = ..., jobs_active: _Optional[int] = ..., version: _Optional[str] = ..., uptime_seconds: _Optional[int] = ...) -> None: ...
