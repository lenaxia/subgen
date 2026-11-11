"""Dispatcher runner: pulls tasks and executes them in-process.

The dispatcher wraps the existing TranscriptionServicer with a fake gRPC
context, so the transcription path (skip logic, language policy, multi-language
handling, subtitle writing) is the exact tested code the gRPC path uses —
behavioral parity without a port.

Failure policy (differs from the Go orchestrator, deliberately):
REQUEUE-NOT-DELETE. A failed job goes back on the queue with an attempt
counter up to max_attempts, then lands in history as failed. Workers are
idempotent (skip-if-subtitle-exists), so retries are safe.
"""

import logging
import os
import queue as queue_mod
import threading
import time
from typing import Any, Dict, Optional

import grpc

from config.settings import WorkerSettings
from dispatcher.mediaserver import JellyfinClient, MediaServerError, PlexClient
from dispatcher.queue import Task, TaskQueue
from grpc_server.service import TranscriptionServicer
from pb import transcription_pb2
from utils.errors import TranscriptionError

logger = logging.getLogger(__name__)

# Video/audio extensions accepted by scans (/batch, monitor). Mirrors the Go
# monitor's mediaExtensions list.
MEDIA_EXTENSIONS = {
    ".mkv", ".mp4", ".avi", ".mov", ".m4v", ".webm", ".flv", ".wmv",
    ".mpg", ".mpeg", ".m2ts", ".ts",
    ".mp3", ".flac", ".m4a", ".wav", ".ogg", ".opus", ".wma", ".aac",
}


class DispatcherAbort(Exception):
    """Stands in for gRPC's context.abort inside the in-process path."""

    def __init__(self, status_code: Any, details: str):
        self.status_code = status_code
        self.details = details
        super().__init__(details)


class FakeGrpcContext:
    """Minimal grpc.ServicerContext stand-in: only abort() is used."""

    def abort(self, status_code: Any, details: str) -> None:
        raise DispatcherAbort(status_code, details)


class Dispatcher:
    """Owns the queue, the worker loop, the monitor, and media-server clients."""

    def __init__(
        self,
        config: WorkerSettings,
        servicer: TranscriptionServicer,
        task_queue: TaskQueue,
        max_attempts: int = 3,
    ):
        self.config = config
        self.servicer = servicer
        self.queue = task_queue
        self.max_attempts = max_attempts
        self._context = FakeGrpcContext()
        self._stop = threading.Event()
        self._thread: Optional[threading.Thread] = None
        self._monitor_thread: Optional[threading.Thread] = None

        d = config.dispatcher
        self.plex = PlexClient(d.plex_server, d.plex_token) if d.plex_enabled else None
        self.jellyfin = (
            JellyfinClient(d.jellyfin_server, d.jellyfin_token) if d.jellyfin_enabled else None
        )

    # -- lifecycle -----------------------------------------------------------

    def start(self) -> None:
        self._thread = threading.Thread(target=self.run, name="dispatcher", daemon=True)
        self._thread.start()
        if self.config.dispatcher.monitor_enabled:
            self._monitor_thread = threading.Thread(
                target=self.run_monitor, name="dispatcher-monitor", daemon=True
            )
            self._monitor_thread.start()

    def stop(self, timeout: float = 10.0) -> None:
        self._stop.set()
        if self._thread:
            self._thread.join(timeout=timeout)
        if self._monitor_thread:
            self._monitor_thread.join(timeout=5)

    # -- worker loop ---------------------------------------------------------

    def run(self) -> None:
        logger.info("Dispatcher worker loop started")
        while not self._stop.is_set():
            task = None
            try:
                task = self.queue.dequeue()
            except Exception as e:
                logger.error("Queue dequeue failed: %s", e)
                self._stop.wait(1.0)
                continue
            if task is None:
                self._stop.wait(0.2)
                continue
            try:
                self.process(task)
            except Exception as e:
                # process() handles its own failure policy; this is a
                # last-resort guard so the loop itself can never die.
                logger.exception("Unexpected dispatcher error on task %s", task.id[:12])
                # Record the failure: dropping silently would hide the loss
                self.queue.mark_failed(task, f"unexpected dispatcher error: {e}")
                self._deliver_result(task, {"success": False, "error": str(e)})

    def process(self, task: Task) -> None:
        task.attempts += 1
        try:
            self._resolve_media_path(task)
        except MediaServerError as e:
            self._fail_or_requeue(task, f"media path resolution failed: {e}")
            return

        if task.task_type == "detect_language":
            self._process_detect_language(task)
            return

        result = self._transcribe(task)
        if result is None:
            return  # failure already handled (requeue or mark_failed)

        if task.result_queue is not None:
            self._deliver_result(task, result)

        if result.get("success"):
            logger.info(
                "Task %s completed: %s", task.id[:12], result.get("subtitle_path", "")
            )
            self.queue.mark_done(task)
            self._refresh_metadata(task)
        else:
            self._fail_or_requeue(task, result.get("error", "transcription failed"))

    # -- task execution ------------------------------------------------------

    def _transcribe(self, task: Task) -> Optional[Dict[str, Any]]:
        """Run one transcription through the real servicer. None on requeue."""
        request = transcription_pb2.TranscribeRequest()
        if task.audio_content:
            request.audio_content = task.audio_content
        elif task.file_path:
            request.file_path = task.file_path
        else:
            self._fail_or_requeue(task, "task has neither file_path nor audio_content")
            return None

        request.task_type = task.transcription_type
        request.force_language = task.force_language
        if task.task_type == "asr":
            self._apply_asr_options(request.options, task.asr_options)

        try:
            response = self.servicer.Transcribe(request, self._context)
        except DispatcherAbort as e:
            # INVALID_ARGUMENT class errors are permanent (missing file etc.);
            # anything else retries.
            if e.status_code in (
                grpc.StatusCode.INVALID_ARGUMENT,
                grpc.StatusCode.NOT_FOUND,
            ):
                self.queue.mark_failed(task, f"rejected: {e.details}")
                self._deliver_result(task, {"success": False, "error": e.details})
                return None
            self._fail_or_requeue(task, f"aborted: {e.details}")
            return None
        except TranscriptionError as e:
            self._fail_or_requeue(task, f"transcription error: {e}")
            return None

        return {
            "success": response.success,
            "subtitle_path": response.subtitle_path,
            "detected_language": response.detected_language,
            "error": response.error_message,
            "segments": [
                {"start": s.start, "end": s.end, "text": s.text}
                for s in getattr(response, "segments", [])
            ],
            "metadata": {
                "language": response.detected_language,
                "model": self.config.whisper.model_name,
            },
        }

    def _process_detect_language(self, task: Task) -> None:
        request = transcription_pb2.DetectLanguageRequest()
        if task.audio_content:
            request.audio_content = task.audio_content
        else:
            request.file_path = task.file_path
        request.sample_length = int(task.asr_options.get("sample_length", 30))
        request.sample_offset = int(task.asr_options.get("sample_offset", 0))
        try:
            response = self.servicer.DetectLanguage(request, self._context)
            self._deliver_result(
                task,
                {
                    "success": True,
                    "language": response.language_name,
                    "code": response.language_code,
                    "confidence": response.confidence,
                },
            )
            self.queue.mark_done(task)
        except DispatcherAbort as e:
            self._deliver_result(task, {"success": False, "error": e.details})
            self.queue.mark_failed(task, e.details)

    def _apply_asr_options(self, options: Any, asr: Dict[str, str]) -> None:
        def as_bool(name: str, default: bool = False) -> bool:
            val = asr.get(name, "")
            return val.lower() == "true" if val != "" else default

        options.word_level_highlight = as_bool("word_level_highlight")
        options.custom_regroup = asr.get("custom_regroup", "")
        options.custom_prompt = asr.get("custom_prompt", "")
        options.append_footer = as_bool("append_footer")
        options.subtitle_language_name = asr.get("subtitle_language_name", "")
        options.show_model_in_filename = as_bool("show_model_in_filename", True)
        options.show_subgen_in_filename = as_bool("show_subgen_in_filename", True)

    # -- failure policy ------------------------------------------------------

    def _fail_or_requeue(self, task: Task, error: str) -> None:
        """Requeue-not-delete: retry up to max_attempts, then fail for good.

        Blocking /asr tasks are exempt from retries: after a 504 the caller
        is gone, so re-running the job would only burn CPU with no reader.
        """
        self._deliver_result(task, {"success": False, "error": error})
        if task.result_queue is not None:
            logger.error("ASR task %s failed: %s", task.id[:12], error)
            self.queue.mark_failed(task, error)
        elif task.attempts < self.max_attempts:
            self.queue.requeue(task)
        else:
            logger.error(
                "Task %s failed permanently after %d attempts: %s",
                task.id[:12],
                task.attempts,
                error,
            )
            self.queue.mark_failed(task, error)

    # -- media-server integration -------------------------------------------

    def _resolve_media_path(self, task: Task) -> None:
        if task.file_path or not (task.plex_item_id or task.jellyfin_item_id):
            return
        if task.plex_item_id and self.plex:
            task.file_path = self.plex.get_file_path(task.plex_item_id)
        elif task.jellyfin_item_id and self.jellyfin:
            task.file_path = self.jellyfin.get_file_path(task.jellyfin_item_id)
        logger.info("Resolved %s -> %s", task.id[:12], task.file_path)

    def _refresh_metadata(self, task: Task) -> None:
        if not self.config.dispatcher.metadata_refresh:
            return
        try:
            if task.plex_item_id and self.plex:
                self.plex.refresh_metadata(task.plex_item_id)
            if task.jellyfin_item_id and self.jellyfin:
                self.jellyfin.refresh_metadata(task.jellyfin_item_id)
        except Exception as e:
            logger.warning("Metadata refresh failed for %s: %s", task.id[:12], e)

    def _deliver_result(self, task: Task, result: Dict[str, Any]) -> None:
        if task.result_queue is not None:
            try:
                task.result_queue.put_nowait(result)
            except Exception:
                pass  # caller stopped waiting; nothing to deliver to

    # -- monitor -------------------------------------------------------------

    def run_monitor(self) -> None:
        """Startup scan (optional) + periodic rescans.

        The scan enqueues media files that do NOT already have a generated
        subtitle next to them. This cheap filename-level filter keeps periodic
        rescans from re-transcribing the library; the full skip logic still
        runs per job inside the servicer.
        """
        d = self.config.dispatcher
        folders = [f.strip() for f in d.transcribe_folders.split("|") if f.strip()]
        if not folders:
            logger.warning("MONITOR enabled but TRANSCRIBE_FOLDERS is empty")
            return
        if d.scan_on_startup:
            try:
                queued = self.scan(folders)
                logger.info("Startup scan queued %d files", queued)
            except Exception as e:
                logger.error("Startup scan failed: %s", e)
        while not self._stop.is_set():
            self._stop.wait(d.monitor_interval)
            if self._stop.is_set():
                return
            try:
                queued = self.scan(folders)
                if queued:
                    logger.info("Monitor rescan queued %d files", queued)
            except Exception as e:
                logger.error("Monitor rescan failed: %s", e)

    def scan(self, folders: Any, recursive: bool = True) -> int:
        if isinstance(folders, str):
            folders = [f.strip() for f in folders.split("|") if f.strip()]
        limit = self.config.dispatcher.batch_scan_limit
        queued = 0
        for folder in folders:
            if not os.path.isdir(folder):
                logger.warning("Scan folder missing or not a directory: %s", folder)
                continue
            for root, _dirs, files in os.walk(folder):
                if not recursive:
                    # os.walk descends regardless; prune in-place instead
                    _dirs[:] = []
                for name in files:
                    if os.path.splitext(name)[1].lower() not in MEDIA_EXTENSIONS:
                        continue
                    path = os.path.join(root, name)
                    if self._has_generated_subtitle(path):
                        continue
                    try:
                        self.queue.enqueue(Task(file_path=path))
                        queued += 1
                    except Exception as e:
                        logger.debug("Scan skip %s: %s", path, e)
                    if limit and queued >= limit:
                        logger.warning("Scan limit %d reached; stopping", limit)
                        return queued
        return queued

    @staticmethod
    def _has_generated_subtitle(media_path: str) -> bool:
        base = os.path.splitext(media_path)[0]
        for suffix in (".srt", ".vtt", ".lrc"):
            if os.path.exists(base + suffix):
                return True
        # subgen-marked files with language/model infixes
        directory = os.path.dirname(media_path)
        stem = os.path.basename(base)
        try:
            for name in os.listdir(directory):
                # "." guard: "movie.mkv" must not match "movie2.thing.subgen.srt"
                if name.startswith(stem + ".") and ".subgen." in name:
                    return True
        except OSError:
            pass
        return False
