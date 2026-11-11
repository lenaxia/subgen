"""Unit tests for the dispatcher's in-process task queue.

Behavioral contract ported from the Go orchestrator's queue with deliberate
improvements (requeue-not-delete, persistence, path normalization).
"""

import json
import os

import pytest

from dispatcher.queue import (
    DuplicateTaskError,
    QueueFullError,
    Task,
    TaskQueue,
    compute_task_id,
)


def make_task(file_path: str = "/media/movie.mkv", **kwargs) -> Task:
    return Task(file_path=file_path, **kwargs)


class TestTaskIdentity:
    def test_file_path_is_dedup_key(self):
        t1 = make_task("/media/movie.mkv")
        t2 = make_task("/media/movie.mkv")
        assert t1.id == t2.id

    def test_path_normalized(self):
        t1 = make_task("/media/./show/../movie.mkv")
        t2 = make_task("/media/movie.mkv")
        assert t1.id == t2.id

    def test_plex_key_fallback(self):
        t = Task(plex_item_id="12345", plex_server="http://plex:32400")
        assert t.id == compute_task_id(
            plex_server="http://plex:32400", plex_item_id="12345"
        )

    def test_jellyfin_key_fallback(self):
        t = Task(jellyfin_item_id="abc", jellyfin_server="http://jf:8096")
        assert t.id == compute_task_id(jellyfin_server="http://jf:8096", jellyfin_item_id="abc")

    def test_audio_content_key(self):
        t = Task(audio_content=b"audio-bytes", task_type="asr")
        expected = compute_task_id(audio_content=b"audio-bytes")
        assert t.id == expected
        assert t.id != Task(audio_content=b"other", task_type="asr").id

    def test_priorities(self):
        assert Task(task_type="detect_language").priority == 0
        assert Task(task_type="asr").priority == 1
        assert Task(task_type="transcribe").priority == 2

    def test_invalid_type_defaults_to_transcribe(self):
        assert Task(task_type="nonsense").task_type == "transcribe"


class TestQueueSemantics:
    def test_enqueue_dequeue_fifo_within_priority(self):
        q = TaskQueue(max_size=10)
        a = make_task("/a.mkv")
        b = make_task("/b.mkv")
        q.enqueue(a)
        q.enqueue(b)
        assert q.dequeue() is a  # same priority, earlier queued_at wins
        assert q.dequeue() is b

    def test_priority_ordering(self):
        q = TaskQueue(max_size=10)
        transcribe = make_task("/movie.mkv", task_type="transcribe")
        asr = Task(task_type="asr", audio_content=b"x")
        detect = Task(task_type="detect_language", file_path="/clip.mkv")
        q.enqueue(transcribe)
        q.enqueue(asr)
        q.enqueue(detect)
        assert q.dequeue() is detect
        assert q.dequeue() is asr
        assert q.dequeue() is transcribe

    def test_duplicate_rejected(self):
        q = TaskQueue(max_size=10)
        q.enqueue(make_task("/movie.mkv"))
        with pytest.raises(DuplicateTaskError):
            q.enqueue(make_task("/movie.mkv"))

    def test_duplicate_across_queued_and_processing(self):
        q = TaskQueue(max_size=10)
        t = make_task("/movie.mkv")
        q.enqueue(t)
        q.dequeue()
        with pytest.raises(DuplicateTaskError):
            q.enqueue(make_task("/movie.mkv"))

    def test_requeue_after_completion_allows_same_file(self):
        q = TaskQueue(max_size=10)
        t = make_task("/movie.mkv")
        q.enqueue(t)
        q.dequeue()
        q.mark_done(t)
        q.enqueue(make_task("/movie.mkv"))  # must not raise

    def test_queue_full_rejects(self):
        q = TaskQueue(max_size=1)
        q.enqueue(make_task("/a.mkv"))
        with pytest.raises(QueueFullError):
            q.enqueue(make_task("/b.mkv"))

    def test_dequeue_empty_returns_none(self):
        assert TaskQueue(max_size=5).dequeue() is None

    def test_requeue_returns_to_queued(self):
        q = TaskQueue(max_size=10)
        t = make_task("/movie.mkv")
        q.enqueue(t)
        q.dequeue()
        t.attempts = 2
        q.requeue(t)
        assert q.size() == 1
        assert q.processing_size() == 0
        assert q.dequeue() is t
        assert t.attempts == 2  # attempt count survives

    def test_requeue_ignores_non_processing_task(self):
        q = TaskQueue(max_size=10)
        t = make_task("/movie.mkv")
        q.enqueue(t)
        q.requeue(t)
        assert q.size() == 1  # still queued once, not duplicated

    def test_mark_failed_records_error(self):
        q = TaskQueue(max_size=10)
        t = make_task("/movie.mkv")
        q.enqueue(t)
        q.dequeue()
        q.mark_failed(t, "boom")
        assert q.processing_size() == 0
        assert q.history()[-1]["status"] == "failed"
        assert q.history()[-1]["error"] == "boom"
        assert t.last_error == "boom"

    def test_is_idle(self):
        q = TaskQueue(max_size=10)
        assert q.is_idle()
        q.enqueue(make_task("/a.mkv"))
        assert not q.is_idle()
        t = q.dequeue()
        assert not q.is_idle()  # still processing
        q.mark_done(t)
        assert q.is_idle()


class TestPersistence:
    def test_roundtrip(self, tmp_path):
        path = str(tmp_path / "queue.json")
        q1 = TaskQueue(max_size=10, persist_path=path)
        q1.enqueue(make_task("/a.mkv"))
        q1.enqueue(make_task("/b.mkv"))
        processing = q1.dequeue()

        q2 = TaskQueue(max_size=10, persist_path=path)
        assert q2.size() == 1
        assert q2.processing_size() == 1
        restored = q2.dequeue()
        # /a.mkv was processing when snapshotted; /b.mkv was queued
        assert restored.file_path == "/b.mkv"
        assert processing.file_path == "/a.mkv"

    def test_no_file_starts_empty(self, tmp_path):
        q = TaskQueue(max_size=10, persist_path=str(tmp_path / "missing.json"))
        assert q.size() == 0

    def test_corrupt_file_starts_empty(self, tmp_path):
        path = tmp_path / "queue.json"
        path.write_text("{not json")
        q = TaskQueue(max_size=10, persist_path=str(path))
        assert q.size() == 0

    def test_snapshot_is_json_without_audio(self, tmp_path):
        path = tmp_path / "queue.json"
        q = TaskQueue(max_size=10, persist_path=str(path))
        q.enqueue(Task(task_type="asr", audio_content=b"x" * 1000, file_path=""))
        data = json.loads(path.read_text())
        assert "audio_content" not in data["queued"][0], "audio bytes must not be persisted"
