"""Unit tests for the dispatcher runner (worker loop, failure policy, monitor)."""

import queue as queue_mod
from unittest.mock import MagicMock, Mock, patch

import pytest

from config.settings import DispatcherConfig, WorkerSettings
from dispatcher.queue import Task, TaskQueue
from dispatcher.runner import MEDIA_EXTENSIONS, Dispatcher, DispatcherAbort, FakeGrpcContext


def make_config(**dispatcher_overrides) -> WorkerSettings:
    d = DispatcherConfig(
        plex_enabled=True,
        plex_token="plex-token",
        jellyfin_enabled=False,
        **dispatcher_overrides,
    )
    return WorkerSettings(dispatcher=d)


class FakeServicer:
    """Records Transcribe/DetectLanguage calls; behavior injected per test."""

    def __init__(self):
        self.transcribe_calls = []
        self.detect_calls = []
        self.transcribe_response = None
        self.transcribe_side_effect = None

    def Transcribe(self, request, context):
        self.transcribe_calls.append(request)
        if self.transcribe_side_effect is not None:
            raise self.transcribe_side_effect
        return self.transcribe_response

    def DetectLanguage(self, request, context):
        self.detect_calls.append(request)
        return Mock(
            success=True, language_name="English", language_code="en", confidence=0.99
        )


def make_dispatcher(servicer=None, config=None, max_attempts=3) -> Dispatcher:
    servicer = servicer or FakeServicer()
    config = config or make_config()
    q = TaskQueue(max_size=50)
    return Dispatcher(config, servicer, q, max_attempts=max_attempts)


def transcribe_response(success=True, subtitle_path="/media/movie.srt", error=""):
    r = Mock()
    r.success = success
    r.subtitle_path = subtitle_path
    r.detected_language = "en"
    r.error_message = error
    r.segment_count = 1
    r.segments = []
    return r


class TestTranscribeFlow:
    def test_success_marks_done(self):
        d = make_dispatcher()
        d.servicer.transcribe_response = transcribe_response(success=True)
        task = Task(file_path="/media/movie.mkv")
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        assert d.queue.is_idle()
        assert d.queue.history()[-1]["status"] == "done"
        assert d.servicer.transcribe_calls[0].file_path == "/media/movie.mkv"

    def test_failure_requeues_up_to_max_attempts(self):
        d = make_dispatcher(max_attempts=3)
        d.servicer.transcribe_response = transcribe_response(success=False, error="gpu oom")
        task = Task(file_path="/media/movie.mkv")
        d.queue.enqueue(task)

        d.process(d.queue.dequeue())  # attempt 1
        assert d.queue.size() == 1, "failed task must be requeued"
        assert d.queue.history() == []

        task2 = d.queue.dequeue()
        d.process(task2)  # attempt 2
        assert d.queue.size() == 1

        task3 = d.queue.dequeue()
        d.process(task3)  # attempt 3 -> exhausted
        assert d.queue.is_idle()
        assert d.queue.history()[-1]["status"] == "failed"

    def test_invalid_argument_abort_fails_immediately(self):
        import grpc

        d = make_dispatcher()
        d.servicer.transcribe_side_effect = DispatcherAbort(
            grpc.StatusCode.NOT_FOUND, "File not found"
        )
        task = Task(file_path="/gone.mkv")
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        assert d.queue.is_idle()  # NOT retried
        assert d.queue.history()[-1]["status"] == "failed"

    def test_unavailable_abort_requeues(self):
        import grpc

        d = make_dispatcher()
        d.servicer.transcribe_side_effect = DispatcherAbort(
            grpc.StatusCode.UNAVAILABLE, "worker busy"
        )
        task = Task(file_path="/media/movie.mkv")
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        assert d.queue.size() == 1  # retried

    def test_asr_options_applied(self):
        d = make_dispatcher()
        d.servicer.transcribe_response = transcribe_response()
        task = Task(
            task_type="asr",
            audio_content=b"audio",
            asr_options={
                "word_level_highlight": "true",
                "custom_regroup": "cm_sl=84",
                "custom_prompt": "punctuate",
                "append_footer": "true",
                "show_model_in_filename": "false",
            },
        )
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        opts = d.servicer.transcribe_calls[0].options
        assert opts.word_level_highlight is True
        assert opts.custom_regroup == "cm_sl=84"
        assert opts.custom_prompt == "punctuate"
        assert opts.append_footer is True
        assert opts.show_model_in_filename is False
        assert opts.show_subgen_in_filename is True  # default true


class TestMediaServerResolution:
    def test_plex_path_resolved_before_transcribe(self):
        d = make_dispatcher()
        d.plex = MagicMock()
        d.plex.get_file_path.return_value = "/media/movie.mkv"
        d.servicer.transcribe_response = transcribe_response()
        task = Task(plex_item_id="12345", plex_server="http://p", plex_token="t")
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        d.plex.get_file_path.assert_called_once_with("12345")
        assert d.servicer.transcribe_calls[0].file_path == "/media/movie.mkv"

    def test_plex_resolution_failure_requeues(self):
        from dispatcher.mediaserver import MediaServerError

        d = make_dispatcher()
        d.plex = MagicMock()
        d.plex.get_file_path.side_effect = MediaServerError("plex down")
        task = Task(plex_item_id="12345", plex_server="http://p", plex_token="t")
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        assert d.queue.size() == 1  # retried, not dropped (Go dropped here!)

    def test_metadata_refresh_on_success(self):
        d = make_dispatcher()
        d.servicer.transcribe_response = transcribe_response()
        d.plex = MagicMock()
        task = Task(file_path="/m.mkv", plex_item_id="77", plex_server="http://p", plex_token="t")
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        d.plex.refresh_metadata.assert_called_once_with("77")


class TestDetectLanguageTask:
    def test_detect_language_uses_servicer(self):
        d = make_dispatcher()
        task = Task(
            task_type="detect_language",
            file_path="/media/clip.mkv",
            asr_options={"sample_length": "45", "sample_offset": "5"},
            result_queue=queue_mod.Queue(maxsize=1),
        )
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        req = d.servicer.detect_calls[0]
        assert req.file_path == "/media/clip.mkv"
        assert req.sample_length == 45
        assert req.sample_offset == 5
        result = task.result_queue.get_nowait()
        assert result["success"] is True
        assert result["code"] == "en"


class TestASRResultDelivery:
    def test_result_delivered_to_blocking_caller(self):
        d = make_dispatcher()
        d.servicer.transcribe_response = transcribe_response()
        task = Task(
            task_type="asr",
            audio_content=b"audio",
            file_path="",
            result_queue=queue_mod.Queue(maxsize=1),
        )
        d.queue.enqueue(task)
        d.process(d.queue.dequeue())
        result = task.result_queue.get_nowait()
        assert result["success"] is True
        assert result["subtitle_path"] == "/media/movie.srt"


class TestMonitorScan:
    def test_scan_skips_files_with_subtitles(self, tmp_path):
        d = make_dispatcher(config=make_config(batch_scan_limit=0))
        (tmp_path / "movie1.mkv").write_bytes(b"x")
        (tmp_path / "movie2.mkv").write_bytes(b"x")
        (tmp_path / "movie2.srt").write_text("1\n00:00:00,000 --> 00:00:01,000\nhi\n")

        queued = d.scan(str(tmp_path))
        assert queued == 1
        task = d.queue.dequeue()
        assert task.file_path == str(tmp_path / "movie1.mkv")

    def test_scan_respects_extension_filter(self, tmp_path):
        d = make_dispatcher(config=make_config(batch_scan_limit=0))
        (tmp_path / "movie.mkv").write_bytes(b"x")
        (tmp_path / "notes.txt").write_text("x")
        (tmp_path / "sample.nfo").write_text("x")
        queued = d.scan(str(tmp_path))
        assert queued == 1

    def test_scan_batch_limit(self, tmp_path):
        d = make_dispatcher(config=make_config(batch_scan_limit=2))
        for i in range(5):
            (tmp_path / f"m{i}.mkv").write_bytes(b"x")
        assert d.scan(str(tmp_path)) == 2

    def test_scan_recursive(self, tmp_path):
        d = make_dispatcher(config=make_config(batch_scan_limit=0))
        sub = tmp_path / "season1"
        sub.mkdir()
        (sub / "e1.mkv").write_bytes(b"x")
        assert d.scan(str(tmp_path), recursive=True) == 1

    def test_scan_nonrecursive(self, tmp_path):
        d = make_dispatcher(config=make_config(batch_scan_limit=0))
        sub = tmp_path / "season1"
        sub.mkdir()
        (sub / "e1.mkv").write_bytes(b"x")
        assert d.scan(str(tmp_path), recursive=False) == 0

    def test_subgen_marker_files_count_as_subtitles(self, tmp_path):
        d = make_dispatcher(config=make_config(batch_scan_limit=0))
        (tmp_path / "movie.mkv").write_bytes(b"x")
        (tmp_path / "movie.en.subgen.medium.srt").write_text("1\n")
        assert d.scan(str(tmp_path)) == 0


class TestExtensions:
    def test_media_extension_list_matches_go_monitor(self):
        for ext in (".mkv", ".mp4", ".avi", ".mov", ".m4v", ".webm", ".mp3", ".flac", ".wav"):
            assert ext in MEDIA_EXTENSIONS
        for ext in (".txt", ".nfo", ".jpg", ".srt"):
            assert ext not in MEDIA_EXTENSIONS


class TestFakeContext:
    def test_abort_raises_dispatcher_abort(self):
        import grpc

        ctx = FakeGrpcContext()
        with pytest.raises(DispatcherAbort) as exc:
            ctx.abort(grpc.StatusCode.NOT_FOUND, "nope")
        assert exc.value.status_code == grpc.StatusCode.NOT_FOUND
        assert exc.value.details == "nope"
