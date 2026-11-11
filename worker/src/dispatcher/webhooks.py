"""Webhook HTTP server — Flask port of orchestrator/internal/webhooks/server.go.

Routes, auth header checks, event filters, duplicate/full/other response
codes, and payload field names mirror the Go orchestrator exactly (verified
against server.go and its test fixtures).
"""

import json
import logging
import os
import queue as queue_mod
import time
from typing import Any, Callable, Optional

from flask import Flask, Response, jsonify, request

from config.settings import WorkerSettings
from dispatcher.queue import (
    DuplicateTaskError,
    QueueFullError,
    Task,
    TaskQueue,
)
from dispatcher.runner import Dispatcher
from dispatcher.pathmap import PathMapper
from pb import transcription_pb2

logger = logging.getLogger(__name__)

GET_ERROR_BODY = {
    "error": (
        "You accessed this request incorrectly via a GET request. "
        "See https://github.com/McCloudS/subgen for proper configuration"
    )
}

ASR_OUTPUT_FORMATS = {"srt", "vtt", "lrc", "txt", "tsv", "json"}


def _form(name: str, default: str = "") -> str:
    """Form value with query-string fallback (Go c.FormValue(name, c.Query(...)))."""
    return request.form.get(name, request.args.get(name, default))


def _seconds_ts(t: float) -> str:
    millis = int(round((t - int(t)) * 1000))
    secs = int(t) % 60
    mins = (int(t) // 60) % 60
    hours = int(t) // 3600
    return f"{hours:02d}:{mins:02d}:{secs:02d},{millis:03d}"


def _ts_vtt(t: float) -> str:
    return _seconds_ts(t).replace(",", ".")


def _lrc_ts(t: float) -> str:
    hundredths = int(round((t - int(t)) * 100))
    secs = int(t) % 60
    mins = int(t) // 60
    return f"[{mins:02d}:{secs:02d}.{hundredths:02d}]"


def format_segments(segments: Any, output: str) -> str:
    """Render segments in the requested /asr output format."""
    segs = list(segments or [])
    if output == "json":
        return json.dumps(
            [
                {"start": s["start"], "end": s["end"], "text": s["text"]}
                for s in segs
            ],
            ensure_ascii=False,
        )
    if output == "txt":
        return "\n".join(s["text"].strip() for s in segs)
    if output == "tsv":
        return "\n".join(
            f"{s['start']:.3f}\t{s['end']:.3f}\t{s['text'].strip()}" for s in segs
        )
    if output == "lrc":
        return "\n".join(f"{_lrc_ts(s['start'])}{s['text'].strip()}" for s in segs)
    lines = []
    if output == "vtt":
        lines.append("WEBVTT")
    for i, s in enumerate(segs, 1):
        ts = _ts_vtt if output == "vtt" else _seconds_ts
        arrow = " --> "
        lines.append("")
        lines.append(f"{i}" if output == "srt" else "")
        lines.append(f"{ts(s['start'])}{arrow}{ts(s['end'])}")
        lines.append(s["text"].strip())
    return "\n".join(lines)


class WebhookServer:
    def __init__(
        self,
        config: WorkerSettings,
        task_queue: TaskQueue,
        dispatcher: Dispatcher,
        path_mapper: PathMapper,
        version: str = "subgen-dispatcher",
    ):
        self.config = config
        self.queue = task_queue
        self.dispatcher = dispatcher
        self.path_mapper = path_mapper
        self.version = version
        self.started_at = time.time()
        self.app = Flask("subgen-dispatcher")
        self._register_routes()

    # -- helpers -------------------------------------------------------------

    def _enqueue_or_respond(self, task: Task, duplicate_response: Callable[[], Response]):
        try:
            self.queue.enqueue(task)
            return Response("", status=200)
        except DuplicateTaskError:
            return duplicate_response()
        except QueueFullError:
            return jsonify({"error": "Queue is full, please try again later"}), 429
        except Exception as e:
            logger.error("Failed to queue task: %s", e)
            return jsonify({"error": "Failed to queue task"}), 500

    # -- routes --------------------------------------------------------------

    def _register_routes(self) -> None:
        app = self.app

        def get_error():
            return jsonify(GET_ERROR_BODY), 400

        for path in ("/plex", "/webhook", "/jellyfin", "/emby", "/tautulli", "/asr"):
            app.add_url_rule(path, f"get_error_{path}", get_error, methods=["GET"])
        app.add_url_rule("/", "root", self.handle_root, methods=["GET"])
        app.add_url_rule("/status", "status", self.handle_status, methods=["GET"])

        app.add_url_rule("/plex", "plex", self.handle_plex, methods=["POST"])
        app.add_url_rule("/webhook/plex", "plex_alt", self.handle_plex, methods=["POST"])
        app.add_url_rule("/jellyfin", "jellyfin", self.handle_jellyfin, methods=["POST"])
        app.add_url_rule(
            "/webhook/jellyfin", "jellyfin_alt", self.handle_jellyfin, methods=["POST"]
        )
        app.add_url_rule("/emby", "emby", self.handle_emby, methods=["POST"])
        app.add_url_rule("/webhook/emby", "emby_alt", self.handle_emby, methods=["POST"])
        app.add_url_rule("/tautulli", "tautulli", self.handle_tautulli, methods=["POST"])
        app.add_url_rule("/asr", "asr", self.handle_asr, methods=["POST"])
        app.add_url_rule(
            "/detect-language", "detect_language", self.handle_detect_language, methods=["POST"]
        )
        app.add_url_rule("/batch", "batch", self.handle_batch, methods=["POST"])
        app.add_url_rule("/queue/status", "queue_status", self.handle_queue_status, methods=["GET"])
        app.add_url_rule("/health", "health", self.handle_health, methods=["GET"])
        app.add_url_rule("/ready", "ready", self.handle_ready, methods=["GET"])

    # -- simple endpoints ----------------------------------------------------

    def handle_root(self):
        return jsonify(
            {
                "message": (
                    "The webui for configuration was removed; configure via "
                    "environment variables. See the subgen dispatcher README."
                )
            }
        )

    def handle_status(self):
        return jsonify({"version": self.version, "status": "operational"})

    def handle_health(self):
        return jsonify({"status": "alive", "timestamp": int(time.time())})

    def handle_ready(self):
        if self.dispatcher is None:
            return jsonify({"status": "error", "error": "dispatcher_not_initialized"}), 503
        return jsonify(
            {
                "status": "ready",
                "queue_size": self.queue.size(),
                "workers_available": 1,
            }
        )

    def handle_queue_status(self):
        return jsonify(
            {
                "queue_size": self.queue.size(),
                "processing": self.queue.processing_size(),
                "is_idle": self.queue.is_idle(),
                "history": self.queue.history()[-20:],
            }
        )

    # -- Plex ----------------------------------------------------------------

    def handle_plex(self):
        ua = request.headers.get("User-Agent", "")
        if "PlexMediaServer" not in ua:
            return jsonify(
                {"error": "This doesn't appear to be a properly configured Plex webhook"}
            ), 400
        payload_raw = request.form.get("payload")
        if not payload_raw:
            return jsonify({"error": "Missing payload field"}), 400
        try:
            payload = json.loads(payload_raw)
        except (ValueError, TypeError):
            return jsonify({"error": "Invalid JSON payload"}), 400

        event = payload.get("event", "")
        d = self.config.dispatcher
        if event == "library.new" and not d.process_added_media:
            return Response("", status=200)
        if event == "media.play" and not d.process_media_on_play:
            return Response("", status=200)
        if event not in ("library.new", "media.play"):
            return Response("", status=200)

        metadata = payload.get("Metadata", {}) or {}
        rating_key = str(metadata.get("ratingKey", "") or "")
        if not rating_key:
            return jsonify({"error": "Missing ratingKey in metadata"}), 400
        if not d.plex_enabled:
            return jsonify({"error": "Plex support is disabled (PLEX_ENABLED=false)"}), 400

        task = Task(
            task_type="transcribe",
            plex_item_id=rating_key,
            plex_server=d.plex_server,
            plex_token=d.plex_token,
        )
        return self._enqueue_or_respond(task, lambda: Response("OK", status=200))

    # -- Jellyfin ------------------------------------------------------------

    def handle_jellyfin(self):
        ua = request.headers.get("User-Agent", "")
        if "Jellyfin-Server" not in ua:
            return jsonify(
                {"error": "This doesn't appear to be a properly configured Jellyfin webhook"}
            ), 400
        notification_type = request.form.get("NotificationType", "")
        item_id = request.form.get("ItemId", "")
        if not item_id:
            return jsonify({"error": "Missing ItemId field"}), 400

        d = self.config.dispatcher
        if notification_type == "ItemAdded" and not d.process_added_media:
            return Response("", status=200)
        if notification_type == "PlaybackStart" and not d.process_media_on_play:
            return Response("", status=200)
        if notification_type not in ("ItemAdded", "PlaybackStart"):
            return Response("", status=200)
        if not d.jellyfin_enabled:
            return jsonify(
                {"error": "Jellyfin support is disabled (JELLYFIN_ENABLED=false)"}
            ), 400

        task = Task(
            task_type="transcribe",
            jellyfin_item_id=item_id,
            jellyfin_server=d.jellyfin_server,
            jellyfin_token=d.jellyfin_token,
        )
        return self._enqueue_or_respond(task, lambda: Response("", status=200))

    # -- Emby ----------------------------------------------------------------

    def handle_emby(self):
        data_raw = request.form.get("data", "")
        if not data_raw:
            return Response("", status=200)
        try:
            payload = json.loads(data_raw)
        except (ValueError, TypeError):
            return jsonify({"error": "Invalid JSON payload"}), 400

        event = payload.get("Event", "")
        d = self.config.dispatcher
        if event == "system.notificationtest":
            return jsonify({"message": "Notification test received successfully!"})
        if event == "library.new" and not d.process_added_media:
            return Response("", status=200)
        if event == "playback.start" and not d.process_media_on_play:
            return Response("", status=200)
        if event not in ("library.new", "playback.start"):
            return Response("", status=200)

        item_path = (payload.get("Item", {}) or {}).get("Path", "")
        if not item_path:
            return jsonify({"error": "Missing file path in Item"}), 400
        try:
            mapped = self.path_mapper.map(item_path)
        except (FileNotFoundError, ValueError) as e:
            return jsonify({"error": f"Path mapping failed: {e}"}), 400

        return self._enqueue_or_respond(
            Task(task_type="transcribe", file_path=mapped), lambda: Response("", status=200)
        )

    # -- Tautulli ------------------------------------------------------------

    def handle_tautulli(self):
        if request.headers.get("source", "") != "Tautulli":
            return jsonify(
                {"error": "This doesn't appear to be a properly configured Tautulli webhook"}
            ), 400
        event = request.form.get("event", "")
        file_path = request.form.get("file", "")
        d = self.config.dispatcher
        if event == "added" and not d.process_added_media:
            return Response("", status=200)
        if event == "played" and not d.process_media_on_play:
            return Response("", status=200)
        if event not in ("added", "played"):
            return Response("", status=200)
        if not file_path:
            return jsonify({"error": "Missing file field"}), 400
        try:
            mapped = self.path_mapper.map(file_path)
        except (FileNotFoundError, ValueError) as e:
            return jsonify({"error": f"Path mapping failed: {e}"}), 400

        return self._enqueue_or_respond(
            Task(task_type="transcribe", file_path=mapped), lambda: Response("", status=200)
        )

    # -- ASR (Bazarr-style, blocking) -----------------------------------------

    def handle_asr(self):
        task_param = _form("task", "transcribe")
        language = _form("language", "")
        video_file = _form("video_file", "")
        output = _form("output", "srt").lower().strip()
        if output not in ASR_OUTPUT_FORMATS:
            return jsonify(
                {"error": f"invalid format: {output} (supported: srt, vtt, lrc, txt, tsv, json)"}
            ), 400
        try:
            video_file = self.path_mapper.map(video_file) if video_file else ""
        except (FileNotFoundError, ValueError) as e:
            return jsonify({"error": f"Path mapping failed: {e}"}), 400

        audio_file = request.files.get("audio_file")
        if audio_file is None:
            return jsonify({"error": "Missing or invalid audio_file field"}), 400
        content = audio_file.read()
        max_size = self.config.dispatcher.queue_max_audio_content_size
        if len(content) > max_size:
            return jsonify({"error": "Audio file too large"}), 413
        if len(content) == 0:
            return jsonify({"error": "Empty audio file"}), 400

        asr_options = {
            "output": output,
            "word_level_highlight": _form("word_level_highlight", "false"),
            "custom_regroup": _form("custom_regroup", ""),
            "custom_prompt": _form("custom_prompt", ""),
            "append_footer": _form("append_footer", "false"),
            "subtitle_language_name": _form("subtitle_language_name", ""),
            "show_model_in_filename": _form("show_model_in_filename", "true"),
            "show_subgen_in_filename": _form("show_subgen_in_filename", "true"),
        }
        result_queue: queue_mod.Queue = queue_mod.Queue(maxsize=1)
        task = Task(
            task_type="asr",
            file_path=video_file,
            transcription_type=task_param or "transcribe",
            force_language=language,
            audio_content=content,
            asr_options=asr_options,
            result_queue=result_queue,
        )
        try:
            self.queue.enqueue(task)
        except DuplicateTaskError:
            return jsonify({"error": "Task already queued or processing"}), 409
        except QueueFullError:
            return jsonify({"error": "Queue is full, please try again later"}), 429
        except Exception as e:
            logger.error("Failed to queue ASR task: %s", e)
            return jsonify({"error": "Failed to queue task"}), 500

        timeout = self.config.dispatcher.asr_timeout
        try:
            result = result_queue.get(timeout=timeout)
        except queue_mod.Empty:
            return jsonify({"error": "ASR task timed out"}), 504

        if not result.get("success"):
            return jsonify({"error": result.get("error", "transcription failed")}), 500

        body = format_segments(result.get("segments"), output)
        mime = {
            "vtt": "text/vtt; charset=utf-8",
            "json": "application/json; charset=utf-8",
        }.get(output, "text/plain; charset=utf-8")
        return Response(body, status=200, content_type=mime)

    # -- detect-language (direct, no queue) -----------------------------------

    def handle_detect_language(self):
        try:
            offset = float(request.args.get("offset", 0))
            length = float(request.args.get("length", 30))
        except ValueError:
            return jsonify({"error": "invalid offset/length"}), 400
        if offset < 0:
            return jsonify({"error": "offset must be >= 0"}), 400
        if length <= 0 or length > 300:
            return jsonify({"error": "length must be in (0, 300]"}), 400

        audio_file = request.files.get("audio_file")
        if audio_file is None:
            return jsonify({"error": "Missing or invalid audio_file field"}), 400
        content = audio_file.read()
        if len(content) > 500 * 1024 * 1024:
            return jsonify({"error": "Audio file too large (max 500MB)"}), 400
        if len(content) == 0:
            return jsonify({"error": "Empty audio file"}), 400

        req = transcription_pb2.DetectLanguageRequest(
            audio_content=content, sample_length=int(length), sample_offset=int(offset)
        )
        from dispatcher.runner import DispatcherAbort, FakeGrpcContext

        try:
            resp = self.dispatcher.servicer.DetectLanguage(req, FakeGrpcContext())
        except DispatcherAbort as e:
            return jsonify({"error": e.details}), 400
        except Exception as e:
            logger.error("detect-language failed: %s", e)
            return jsonify({"error": "language detection failed"}), 500
        return jsonify(
            {
                "language": resp.language_name,
                "code": resp.language_code,
                "confidence": resp.confidence,
            }
        )

    # -- batch ----------------------------------------------------------------

    def handle_batch(self):
        directory = request.args.get("directory", "")
        if not directory:
            return jsonify({"error": "directory parameter is required"}), 400
        if not os.path.isdir(directory):
            return jsonify({"error": "directory does not exist"}), 400
        recursive = request.args.get("recursive", "false").lower() == "true"
        before = self.queue.size()
        try:
            queued = self.dispatcher.scan(directory, recursive=recursive)
        except Exception as e:
            return jsonify({"status": "error", "error": str(e)}), 500
        return jsonify(
            {
                "status": "success",
                "queued": queued,
                "queue_size": before + queued,
            }
        )

    # -- serving ---------------------------------------------------------------

    def run(self, host: str = "0.0.0.0", port: Optional[int] = None) -> None:
        port = port or self.config.dispatcher.webhook_port
        # The dispatcher's own Flask server is the only HTTP surface; the
        # worker's separate gRPC http_server is not started in this mode.
        self.app.run(host=host, port=port, threaded=True, debug=False, use_reloader=False)
