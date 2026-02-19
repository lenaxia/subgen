"""
gRPC TranscriptionServicer Implementation

Implements the three RPC methods defined in transcription.proto:
- Transcribe: Main transcription workload
- DetectLanguage: Language detection from audio sample
- HealthCheck: Worker health monitoring
"""

import logging
import time
import os
from typing import Optional

import grpc
import psutil

from config.settings import WorkerSettings
from pb import transcription_pb2
from pb import transcription_pb2_grpc
from transcription.model_manager import ModelManager, ModelConfig
from transcription.engine import TranscriptionEngine, TranscribeOptions


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

        logger.info("TranscriptionServicer initialized with model manager")

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

        # Determine health status based on memory
        if memory_mb > self.config.system.memory_threshold_mb:
            status = transcription_pb2.HealthCheckResponse.UNHEALTHY
            logger.warning(
                f"Worker unhealthy: memory {memory_mb}MB exceeds threshold {self.config.system.memory_threshold_mb}MB"
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

            # Perform transcription
            logger.info(f"Starting transcription (source_type={source_type})")
            result = self.engine.transcribe(
                source=source,
                task_type=task_type,
                force_language=request.force_language if request.force_language else None,
                options=options,
            )

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

                return transcription_pb2.TranscribeResponse(
                    success=True,
                    subtitle_path=result.subtitle_path,
                    detected_language=result.detected_language,
                    stats=stats,
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
