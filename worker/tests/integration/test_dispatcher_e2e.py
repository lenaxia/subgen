"""End-to-end dispatcher test with the REAL model (skipped when unavailable).

Full path: Tautulli webhook POST -> in-process queue -> dispatcher worker loop
-> real TranscriptionServicer (cached tiny model) -> subtitle file on disk.
"""

import os
import shutil
import time

import pytest

from config.settings import DispatcherConfig, ModelLifecycleConfig, WhisperConfig, WorkerSettings
from dispatcher.pathmap import PathMapper
from dispatcher.queue import TaskQueue
from dispatcher.runner import Dispatcher
from dispatcher.webhooks import WebhookServer
from grpc_server.service import TranscriptionServicer

MODELS_DIR = os.environ.get("SUBGEN_TEST_MODELS", os.path.join(os.getcwd(), "models"))
TINY_MARKER = os.path.join(MODELS_DIR, "models--Systran--faster-whisper-tiny")
SPEECH_WAV = "/tmp/opencode/e2e_review/input_speech.wav"

pytestmark = [
    pytest.mark.skipif(
        not os.path.isdir(TINY_MARKER), reason="tiny model not cached locally"
    ),
    pytest.mark.skipif(
        not os.path.exists(SPEECH_WAV), reason="test speech wav not present"
    ),
]


def make_config(media_dir) -> WorkerSettings:
    d = DispatcherConfig(
        plex_enabled=False,
        jellyfin_enabled=False,
        scan_on_startup=False,
        monitor_enabled=False,
        metadata_refresh=False,
        queue_max_size=10,
    )
    return WorkerSettings(
        dispatcher=d,
        whisper=WhisperConfig(
            model_name="tiny",
            model_path=MODELS_DIR,
            device="cpu",
            compute_type="int8",
        ),
        model_lifecycle=ModelLifecycleConfig(cleanup_delay=1, unload_on_idle=False),
    )


@pytest.fixture
def e2e(tmp_path):
    media_dir = tmp_path / "media"
    media_dir.mkdir()
    config = make_config(media_dir)
    servicer = TranscriptionServicer(config)
    q = TaskQueue(max_size=10)
    dispatcher = Dispatcher(config, servicer, q, max_attempts=2)
    server = WebhookServer(
        config=config,
        task_queue=q,
        dispatcher=dispatcher,
        path_mapper=PathMapper(enabled=False, from_list="", to_list=""),
    )
    server.app.config["TESTING"] = True
    dispatcher.start()
    yield server, q, media_dir
    dispatcher.stop()


def wait_for_subtitle(media_dir, timeout=180):
    deadline = time.time() + timeout
    while time.time() < deadline:
        for name in os.listdir(media_dir):
            if name.endswith((".srt", ".lrc")):
                return os.path.join(media_dir, name)
        time.sleep(0.5)
    return None


class TestWebhookToSubtitleE2E:
    def test_tautulli_webhook_produces_subtitle(self, e2e):
        server, q, media_dir = e2e
        media = media_dir / "test_movie.mkv"
        shutil.copy(SPEECH_WAV, media)  # engine sniffs the container, extension agnostic

        resp = server.app.test_client().post(
            "/tautulli",
            data=f"event=added&file={media}",
            headers={"source": "Tautulli", "Content-Type": "application/x-www-form-urlencoded"},
        )
        assert resp.status_code == 200

        subtitle = wait_for_subtitle(media_dir)
        assert subtitle is not None, "webhook did not produce a subtitle in time"
        assert os.path.getsize(subtitle) > 0
        with open(subtitle) as f:
            content = f.read()
        assert "-->" in content or "[00:" in content  # srt or lrc cues present

    def test_duplicate_webhook_enqueues_once(self, e2e):
        server, q, media_dir = e2e
        media = media_dir / "dup.mkv"
        shutil.copy(SPEECH_WAV, media)
        client = server.app.test_client()
        data = f"event=added&file={media}"
        headers = {"source": "Tautulli", "Content-Type": "application/x-www-form-urlencoded"}
        assert client.post("/tautulli", data=data, headers=headers).status_code == 200
        assert client.post("/tautulli", data=data, headers=headers).status_code == 200
        # second post must NOT add a second task (dedup by path)
        assert q.size() <= 1

    def test_batch_endpoint_scans_directory(self, e2e):
        server, q, media_dir = e2e
        (media_dir / "batch1.mkv").write_bytes(b"\x00" * 16)  # not real media: job will fail-retry-fail
        resp = server.app.test_client().post(f"/batch?directory={media_dir}")
        assert resp.status_code == 200
        assert resp.json["status"] == "success"
        assert resp.json["queued"] == 1
