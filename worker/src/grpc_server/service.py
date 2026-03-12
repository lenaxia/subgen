"""
gRPC TranscriptionServicer Implementation

Implements the three RPC methods defined in transcription.proto:
- Transcribe: Main transcription workload
- DetectLanguage: Language detection from audio sample
- HealthCheck: Worker health monitoring
"""

import glob
import logging
import time
import os
from typing import Optional, List, Tuple

import grpc
import psutil

from config.settings import WorkerSettings
from pb import transcription_pb2
from pb import transcription_pb2_grpc
from transcription.model_manager import ModelManager, ModelConfig
from transcription.engine import TranscriptionEngine, TranscribeOptions
from subtitles.skip_checker import SkipChecker, SkipReason


logger = logging.getLogger(__name__)


class TranscriptionServicer(transcription_pb2_grpc.TranscriptionServiceServicer):
    """
    gRPC service implementation for transcription worker.

    Implements all three RPC methods as defined in the protobuf schema.
    Integrates TranscriptionEngine and ModelManager for actual transcription.
    """

    def __init__(self, config: WorkerSettings):
        """
        Initialize the TranscriptionServicer.

        Args:
            config: Worker configuration settings
        """
        self.config = config
        self.stats = {
            "jobs_processed": 0,
            "jobs_active": 0,
            "jobs_failed": 0,
            "consecutive_errors": 0,
            "last_job_timestamp": 0,
            "memory_mb": 0,
        }
        self.start_time: Optional[float] = time.time()

        # Progress tracking for stuck job detection
        self._progress_lock = __import__("threading").Lock()
        self._current_job_id: Optional[str] = None
        self._segments_processed: int = 0
        self._last_progress_timestamp: float = 0
        self._progress_timeout_seconds: int = 300  # 5 minutes without progress = stuck

        # Initialize model manager
        model_config = ModelConfig(
            model_name=config.whisper.model_name,
            model_path=str(config.whisper.model_path),
            device=config.whisper.device,
            cpu_threads=config.whisper.cpu_threads,
            num_workers=config.system.max_workers,
            compute_type=config.whisper.compute_type,
            cleanup_delay=config.model_lifecycle.cleanup_delay,
            clear_vram=config.model_lifecycle.clear_vram_on_complete,
        )
        self.model_manager = ModelManager(model_config)

        # Initialize transcription engine
        self.engine = TranscriptionEngine(config)

        # Initialize skip checker
        self.skip_checker = SkipChecker(config)

        logger.info("TranscriptionServicer initialized with model manager and skip checker")

    def HealthCheck(
        self, request: transcription_pb2.HealthCheckRequest, context: grpc.ServicerContext
    ) -> transcription_pb2.HealthCheckResponse:
        """
        Check worker health and return current status.

        Returns memory usage, job statistics, model status, and uptime.
        """
        logger.debug("HealthCheck request received")

        # Get current memory usage
        process = psutil.Process()
        memory_mb = int(process.memory_info().rss / (1024 * 1024))

        # Update stats with current memory
        self.stats["memory_mb"] = memory_mb

        # Determine health status based on memory and stuck jobs
        is_stuck = self._is_job_stuck()
        if memory_mb > self.config.system.memory_threshold_mb:
            status = transcription_pb2.HealthCheckResponse.UNHEALTHY
            logger.warning(
                f"Worker unhealthy: memory {memory_mb}MB exceeds threshold {self.config.system.memory_threshold_mb}MB"
            )
        elif is_stuck:
            status = transcription_pb2.HealthCheckResponse.UNHEALTHY
            logger.warning(
                f"Worker unhealthy: job stuck (no progress for {self._progress_timeout_seconds}s)"
            )
        else:
            status = transcription_pb2.HealthCheckResponse.HEALTHY

        # Calculate uptime
        if self.start_time is not None:
            uptime = int(time.time() - self.start_time)
        else:
            uptime = 0

        response = transcription_pb2.HealthCheckResponse(
            status=status,
            memory_mb=memory_mb,
            model_loaded=self.model_manager.is_loaded(),
            jobs_processed=self.stats["jobs_processed"],
            jobs_active=self.stats["jobs_active"],
            version=self.config.version,
            uptime_seconds=uptime,
        )

        logger.debug(
            f"HealthCheck response: status={status}, memory={memory_mb}MB, uptime={uptime}s"
        )
        return response

    def _update_progress(self, segments_count: int) -> None:
        """
        Update progress tracking for the current job.
        Called during transcription as segments are processed.
        """
        with self._progress_lock:
            self._segments_processed = segments_count
            self._last_progress_timestamp = time.time()

    def _start_job_tracking(self, job_id: str) -> None:
        """Initialize progress tracking for a new job."""
        with self._progress_lock:
            self._current_job_id = job_id
            self._segments_processed = 0
            self._last_progress_timestamp = time.time()

    def _end_job_tracking(self) -> None:
        """Clear progress tracking after job completes."""
        with self._progress_lock:
            self._current_job_id = None
            self._segments_processed = 0
            self._last_progress_timestamp = 0

    def _is_job_stuck(self) -> bool:
        """
        Check if the current job is stuck (no progress for too long).
        Returns True if job appears stuck, False otherwise.
        """
        with self._progress_lock:
            if self._current_job_id is None:
                return False

            if self._last_progress_timestamp == 0:
                return False

            elapsed = time.time() - self._last_progress_timestamp
            return elapsed > self._progress_timeout_seconds

    def _transcribe_multi_language(
        self,
        source: str,
        file_path: str,
        target_languages: List[str],
        preferred_languages: List[str],
        transcribe_preferred: bool,
        force_language: Optional[str],
        options: TranscribeOptions,
        start_time: float,
    ) -> transcription_pb2.TranscribeResponse:
        """
        Handle multi-language subtitle generation.

        Generates subtitles in multiple target languages based on language policy.
        Only works with file paths (not bytes).
        """
        logger.info(
            f"Multi-language transcription: targets={target_languages}, preferred={preferred_languages}"
        )

        subtitle_paths: List[str] = []
        output_languages: List[str] = []
        detected_language = ""
        total_segments = 0

        try:
            # Load model
            model = self.model_manager.load()
            self.engine.model = model

            # Detect audio language first (needed for policy decision)
            detection_result = self.engine.detect_language(
                source, sample_length=30, sample_offset=0
            )
            audio_language = (
                detection_result.language_code.lower() if detection_result.success else ""
            )

            if not audio_language:
                logger.warning("Could not detect audio language, defaulting to transcribe")
                audio_language = force_language.lower() if force_language else ""

            detected_language = audio_language
            logger.info(f"Detected audio language: {audio_language}")

            # Determine output languages based on policy
            output_tasks = self._determine_output_languages(
                audio_language=audio_language,
                target_languages=target_languages,
                preferred_languages=[lang.lower() for lang in preferred_languages],
                transcribe_preferred=transcribe_preferred,
            )

            logger.info(f"Output tasks: {output_tasks}")

            # Generate subtitle for each output language
            for output_lang, task_type in output_tasks:
                output_lang_lower = output_lang.lower()

                # Skip check for specific output language
                skip_result = self.skip_checker.check(file_path, output_language=output_lang_lower)
                if skip_result.should_skip:
                    logger.info(f"Skipping {output_lang_lower}: {skip_result.details}")
                    continue

                # Set target language for filename
                options_copy = TranscribeOptions(
                    whisper_model=options.whisper_model,
                    whisper_threads=options.whisper_threads,
                    word_level_highlight=options.word_level_highlight,
                    custom_regroup=options.custom_regroup,
                    lrc_for_audio=options.lrc_for_audio,
                    custom_prompt=options.custom_prompt,
                    append_footer=options.append_footer,
                    subtitle_language_name=options.subtitle_language_name,
                    show_model_in_filename=options.show_model_in_filename,
                    show_subgen_in_filename=options.show_subgen_in_filename,
                    target_language=output_lang_lower,
                )

                # Perform transcription
                result = self.engine.transcribe(
                    source=source,
                    task_type=task_type,
                    force_language=force_language,
                    options=options_copy,
                    return_segments=False,
                    progress_callback=self._update_progress,
                )

                if result.success:
                    subtitle_paths.append(result.subtitle_path)
                    output_languages.append(output_lang_lower)
                    total_segments += result.segment_count
                    logger.info(f"Generated {output_lang_lower} subtitle: {result.subtitle_path}")
                else:
                    logger.error(
                        f"Failed to generate {output_lang_lower} subtitle: {result.error_message}"
                    )

            # Update stats
            self.stats["jobs_processed"] += 1
            self.stats["last_job_timestamp"] = int(time.time())
            processing_time = time.time() - start_time

            if subtitle_paths:
                self.stats["consecutive_errors"] = 0
                logger.info(
                    f"Multi-language transcription completed: {len(subtitle_paths)} files "
                    f"({total_segments} total segments in {processing_time:.2f}s)"
                )

                return transcription_pb2.TranscribeResponse(
                    success=True,
                    subtitle_path=subtitle_paths[0] if subtitle_paths else "",
                    detected_language=detected_language,
                    subtitle_paths=subtitle_paths,
                    output_languages=output_languages,
                    stats=transcription_pb2.TranscriptionStats(
                        duration_seconds=processing_time,
                        segment_count=total_segments,
                        transcription_time_ms=int(processing_time * 1000),
                    ),
                )
            else:
                return transcription_pb2.TranscribeResponse(
                    success=True,
                    subtitle_path="",
                    detected_language=detected_language,
                    subtitle_paths=[],
                    output_languages=[],
                )

        except Exception as e:
            logger.exception(f"Multi-language transcription error: {e}")
            return transcription_pb2.TranscribeResponse(
                success=False,
                error_message=f"Multi-language transcription failed: {str(e)}",
            )

    def _determine_output_languages(
        self,
        audio_language: str,
        target_languages: List[str],
        preferred_languages: List[str],
        transcribe_preferred: bool,
    ) -> List[Tuple[str, str]]:
        """
        Determine output languages based on audio language and policy.

        Args:
            audio_language: Detected audio language (ISO 639-1)
            target_languages: List of target output languages
            preferred_languages: List of preferred audio languages
            transcribe_preferred: Whether to transcribe when audio matches preferred

        Returns:
            List of (language_code, task_type) tuples
        """
        outputs = []
        audio_lang_lower = audio_language.lower() if audio_language else ""
        preferred_lower = [lang.lower() for lang in preferred_languages]
        target_lower = [lang.lower() for lang in target_languages]

        # 1. Transcribe preferred language if enabled and audio matches
        if transcribe_preferred and audio_lang_lower in preferred_lower:
            outputs.append((audio_lang_lower, "transcribe"))
            logger.info(f"Audio {audio_lang_lower} is preferred: will transcribe")

        # 2. Translate to each target language
        for target_lang in target_lower:
            if target_lang == audio_lang_lower:
                # Skip if same as audio (already handled by transcribe)
                if not (transcribe_preferred and audio_lang_lower in preferred_lower):
                    # If not transcribing, still need to "translate" to same lang
                    # This handles the edge case where audio is in target language
                    # but transcribe_preferred is False
                    pass
            else:
                outputs.append((target_lang, "translate"))
                logger.info(
                    f"Target {target_lang} differs from audio {audio_lang_lower}: will translate"
                )

        # 3. If no targets specified and no transcribe, default to transcribe audio language
        if not outputs and audio_lang_lower:
            outputs.append((audio_lang_lower, "transcribe"))
            logger.info(f"No targets specified: default transcribe {audio_lang_lower}")

        return outputs

    def DetectLanguage(
        self, request: transcription_pb2.DetectLanguageRequest, context: grpc.ServicerContext
    ) -> transcription_pb2.DetectLanguageResponse:
        """
        Detect language from audio sample.

        Uses the TranscriptionEngine to detect language.
        """
        logger.info("DetectLanguage request received")

        # Validate that audio source is provided
        has_file_path = request.HasField("file_path")
        has_audio_content = request.HasField("audio_content")

        if not has_file_path and not has_audio_content:
            logger.error("DetectLanguage: neither file_path nor audio_content provided")
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Either file_path or audio_content is required"
            )

        # Log the audio source
        if has_file_path:
            logger.debug(f"DetectLanguage: file_path={request.file_path}")
            source = request.file_path
        else:
            logger.debug(f"DetectLanguage: audio_content={len(request.audio_content)} bytes")
            source = request.audio_content

        # Use default sample length if not specified
        sample_length = request.sample_length if request.sample_length > 0 else 30
        sample_offset = request.sample_offset if request.sample_offset > 0 else 0

        logger.debug(
            f"DetectLanguage: sample_length={sample_length}, sample_offset={sample_offset}"
        )

        try:
            # Load model
            model = self.model_manager.load()
            self.engine.model = model

            # Detect language
            detection_result = self.engine.detect_language(source, sample_length, sample_offset)

            logger.info(f"Language detected: {detection_result.language_code}")

            return transcription_pb2.DetectLanguageResponse(
                success=True,
                language_code=detection_result.language_code,
                language_name=detection_result.language_name,
                confidence=detection_result.confidence,
            )

        except Exception as e:
            logger.error(f"Language detection failed: {e}", exc_info=True)
            return transcription_pb2.DetectLanguageResponse(
                success=False, error_message=f"Language detection failed: {str(e)}"
            )

        finally:
            # Schedule model cleanup if cleanup is enabled
            if self.config.model_lifecycle.clear_vram_on_complete:
                self.model_manager.schedule_cleanup()
                logger.debug("Scheduled model cleanup after language detection")

    def Transcribe(
        self, request: transcription_pb2.TranscribeRequest, context: grpc.ServicerContext
    ) -> transcription_pb2.TranscribeResponse:
        """
        Transcribe audio file to subtitles.

        Uses TranscriptionEngine and ModelManager for actual transcription.
        """
        # Determine audio source: file_path or audio_content
        if request.WhichOneof("audio_source") == "file_path":
            source_type = "file"
            source = request.file_path
            logger.info(f"Transcribe request received: file_path={request.file_path}")

            # Validate file exists
            if not os.path.exists(request.file_path):
                logger.error(f"Transcribe: file not found: {request.file_path}")
                context.abort(grpc.StatusCode.NOT_FOUND, f"File not found: {request.file_path}")

            # Comprehensive skip check
            skip_result = self.skip_checker.check(request.file_path)
            if skip_result.should_skip:
                reason_str = skip_result.reason.value if skip_result.reason else "unknown"
                logger.info(f"Skipping transcription: {reason_str} - {skip_result.details}")

                # For target subtitle existence, return the existing subtitle path
                if skip_result.reason in [SkipReason.SUBTITLE_EXISTS, SkipReason.LRC_EXISTS]:
                    base = os.path.splitext(request.file_path)[0]
                    existing = glob.glob(f"{base}.subgen.*.*.srt") + glob.glob(
                        f"{base}.subgen.*.*.lrc"
                    )
                    existing += glob.glob(f"{base}.srt") + glob.glob(f"{base}.lrc")
                    if existing:
                        return transcription_pb2.TranscribeResponse(
                            success=True,
                            subtitle_path=existing[0],
                            detected_language="",
                        )

                # For other skip reasons, return success with no subtitle
                return transcription_pb2.TranscribeResponse(
                    success=True,
                    subtitle_path="",
                    detected_language="",
                )

                # For target subtitle existence, return the existing subtitle path
                if skip_result.reason in [SkipReason.SUBTITLE_EXISTS, SkipReason.LRC_EXISTS]:
                    base = os.path.splitext(request.file_path)[0]
                    existing = glob.glob(f"{base}.subgen.*.*.srt") + glob.glob(
                        f"{base}.subgen.*.*.lrc"
                    )
                    existing += glob.glob(f"{base}.srt") + glob.glob(f"{base}.lrc")
                    if existing:
                        return transcription_pb2.TranscribeResponse(
                            success=True,
                            subtitle_path=existing[0],
                            detected_language="",
                        )

                # For other skip reasons, return success with no subtitle
                return transcription_pb2.TranscribeResponse(
                    success=True,
                    subtitle_path="",
                    detected_language="",
                )

        elif request.WhichOneof("audio_source") == "audio_content":
            source_type = "bytes"
            source = request.audio_content
            logger.info(
                f"Transcribe request received: audio_content={len(request.audio_content)} bytes"
            )

            # Validate audio content is not empty
            if len(request.audio_content) == 0:
                logger.error("Transcribe: audio_content is empty")
                context.abort(grpc.StatusCode.INVALID_ARGUMENT, "audio_content is empty")
        else:
            logger.error("Transcribe: neither file_path nor audio_content provided")
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "Either file_path or audio_content is required"
            )

        # Log request details
        task_type = request.task_type if request.task_type else "transcribe"
        logger.debug(f"Transcribe: task_type={task_type}")
        logger.debug(f"Transcribe: force_language={request.force_language}")
        if request.metadata:
            logger.debug(f"Transcribe: metadata={dict(request.metadata)}")

        # Track active jobs
        self.stats["jobs_active"] += 1
        start_time = time.time()

        try:
            # Load model
            logger.info("Loading Whisper model...")
            model = self.model_manager.load()
            self.engine.model = model
            logger.info("Model loaded successfully")

            # Prepare transcription options
            options = TranscribeOptions(
                whisper_model=self.config.whisper.model_name,
                whisper_threads=self.config.whisper.cpu_threads,
                word_level_highlight=self.config.transcription.word_level_highlight,
                lrc_for_audio=self.config.transcription.lrc_for_audio_files,
                append_footer=self.config.subtitle.append_footer,
                show_model_in_filename=self.config.subtitle.show_model_in_filename,
                show_subgen_in_filename=self.config.subtitle.show_subgen_in_filename,
            )

            # Override with request options if provided
            if request.options:
                if request.options.whisper_model:
                    options.whisper_model = request.options.whisper_model
                if request.options.word_level_highlight:
                    options.word_level_highlight = request.options.word_level_highlight
                if request.options.custom_regroup:
                    options.custom_regroup = request.options.custom_regroup

            # Check if multi-language transcription is requested
            target_languages = list(request.target_languages) if request.target_languages else []
            preferred_languages = (
                list(request.preferred_audio_languages) if request.preferred_audio_languages else []
            )
            transcribe_preferred = request.transcribe_preferred

            # If no target_languages from request, check config
            if not target_languages:
                target_languages = self.config.skip.get_target_languages()
                preferred_languages = self.config.skip.get_preferred_audio_languages()
                transcribe_preferred = self.config.skip.transcribe_preferred

            # Multi-language transcription mode
            if target_languages and source_type == "file":
                return self._transcribe_multi_language(
                    source=request.file_path,  # Always string path for multi-language
                    file_path=request.file_path,
                    target_languages=target_languages,
                    preferred_languages=preferred_languages,
                    transcribe_preferred=transcribe_preferred,
                    force_language=request.force_language if request.force_language else None,
                    options=options,
                    start_time=start_time,
                )

            # Single-language transcription (existing path)
            logger.info(f"Starting transcription (source_type={source_type})")

            # Start progress tracking
            job_id = f"job_{int(time.time() * 1000)}"
            self._start_job_tracking(job_id)

            try:
                result = self.engine.transcribe(
                    source=source,
                    task_type=task_type,
                    force_language=request.force_language if request.force_language else None,
                    options=options,
                    return_segments=(source_type == "bytes"),
                    progress_callback=self._update_progress,
                )
            finally:
                self._end_job_tracking()

            # Update stats
            self.stats["jobs_processed"] += 1
            self.stats["last_job_timestamp"] = int(time.time())
            processing_time = time.time() - start_time

            if result.success:
                # Reset consecutive errors on success
                self.stats["consecutive_errors"] = 0

                logger.info(
                    f"Transcription completed successfully: {result.subtitle_path} "
                    f"({result.segment_count} segments in {processing_time:.2f}s)"
                )

                # Create stats message
                stats = transcription_pb2.TranscriptionStats(
                    duration_seconds=result.duration_seconds,
                    segment_count=result.segment_count,
                    transcription_time_ms=result.transcription_time_ms,
                    peak_memory_mb=result.peak_memory_mb,
                )

                # Convert segments to protobuf format.
                # result.segments is populated only when return_segments=True
                # (i.e. bytes/ASR input).  For file-based input it is None and
                # the orchestrator reads segments from the written subtitle file.
                segments = []
                if result.segments:
                    for segment in result.segments:
                        pb_segment = transcription_pb2.SubtitleSegment(
                            start=segment.start, end=segment.end, text=segment.text
                        )
                        segments.append(pb_segment)

                return transcription_pb2.TranscribeResponse(
                    success=True,
                    subtitle_path=result.subtitle_path,
                    detected_language=result.detected_language,
                    stats=stats,
                    segments=segments,
                )
            else:
                # Track failure
                self.stats["jobs_failed"] += 1
                self.stats["consecutive_errors"] += 1

                logger.error(f"Transcription failed: {result.error_message}")
                return transcription_pb2.TranscribeResponse(
                    success=False,
                    error_message=result.error_message,
                )

        except Exception as e:
            # Track exception as failure
            self.stats["jobs_failed"] += 1
            self.stats["consecutive_errors"] += 1

            logger.exception(f"Transcription error: {e}")
            return transcription_pb2.TranscribeResponse(
                success=False,
                error_message=f"Transcription failed: {str(e)}",
            )

        finally:
            # Always decrement active jobs
            self.stats["jobs_active"] -= 1
            logger.debug(f"Transcribe: jobs_active={self.stats['jobs_active']}")

            # Schedule model cleanup if no active jobs and cleanup is enabled
            if (
                self.stats["jobs_active"] == 0
                and self.config.model_lifecycle.clear_vram_on_complete
            ):
                self.model_manager.schedule_cleanup()
                logger.debug("Scheduled model cleanup after transcription")
