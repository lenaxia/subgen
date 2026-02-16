"""
gRPC TranscriptionServicer Implementation

Implements the three RPC methods defined in transcription.proto:
- Transcribe: Main transcription workload
- DetectLanguage: Language detection from audio sample
- HealthCheck: Worker health monitoring

Status: Stub implementation - actual transcription logic will be added in STORY_02
"""

import logging
import time
from typing import Optional

import grpc
import psutil

from config.settings import WorkerSettings
from pb import transcription_pb2
from pb import transcription_pb2_grpc


logger = logging.getLogger(__name__)


class TranscriptionServicer(transcription_pb2_grpc.TranscriptionServiceServicer):
    """
    gRPC service implementation for transcription worker.

    Implements all three RPC methods as defined in the protobuf schema.
    The actual transcription engine logic will be integrated in STORY_02.
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
        }
        self.start_time: Optional[float] = None
        self._model_loaded = False

        logger.info("TranscriptionServicer initialized")

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
            model_loaded=self._model_loaded,
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

        Stub implementation - actual language detection will be added in STORY_02.
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
        else:
            logger.debug(f"DetectLanguage: audio_content={len(request.audio_content)} bytes")

        # Use default sample length if not specified
        sample_length = request.sample_length if request.sample_length > 0 else 30
        sample_offset = request.sample_offset if request.sample_offset > 0 else 0

        logger.debug(
            f"DetectLanguage: sample_length={sample_length}, sample_offset={sample_offset}"
        )

        # Stub response - actual implementation in STORY_02
        logger.warning("DetectLanguage: Stub implementation - returning error")
        return transcription_pb2.DetectLanguageResponse(
            success=False, error_message="Language detection not yet implemented (STORY_02)"
        )

    def Transcribe(
        self, request: transcription_pb2.TranscribeRequest, context: grpc.ServicerContext
    ) -> transcription_pb2.TranscribeResponse:
        """
        Transcribe audio file to subtitles.

        Stub implementation - actual transcription will be added in STORY_02.
        """
        logger.info(f"Transcribe request received: file_path={request.file_path}")

        # Validate file_path
        if not request.file_path or request.file_path.strip() == "":
            logger.error("Transcribe: file_path is required")
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, "file_path is required")

        # Log request details
        logger.debug(f"Transcribe: task_type={request.task_type}")
        logger.debug(f"Transcribe: force_language={request.force_language}")
        if request.metadata:
            logger.debug(f"Transcribe: metadata={dict(request.metadata)}")
        if request.options:
            logger.debug(f"Transcribe: whisper_model={request.options.whisper_model}")

        # Track active jobs
        self.stats["jobs_active"] += 1

        try:
            # Stub response - actual implementation in STORY_02
            logger.warning("Transcribe: Stub implementation - returning error")

            return transcription_pb2.TranscribeResponse(
                success=False, error_message="Transcription not yet implemented (STORY_02)"
            )

        finally:
            # Always decrement active jobs
            self.stats["jobs_active"] -= 1
            logger.debug(f"Transcribe: jobs_active={self.stats['jobs_active']}")
