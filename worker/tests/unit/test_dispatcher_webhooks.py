"""Unit tests for the dispatcher webhook server.

Payloads are ported verbatim from the Go orchestrator's test fixtures
(orchestrator/test/integration/webhook_payloads.go) — response codes and
body semantics mirror server.go exactly.
"""

import json
from unittest.mock import MagicMock

import pytest

from config.settings import DispatcherConfig, SystemConfig, WorkerSettings
from dispatcher.pathmap import PathMapper
from dispatcher.queue import Task, TaskQueue
from dispatcher.webhooks import WebhookServer, format_segments


@pytest.fixture
def config():
    d = DispatcherConfig(
        plex_enabled=True,
        plex_token="plex-token",
        jellyfin_enabled=True,
        jellyfin_token="jf-token",
        process_added_media=True,
        process_media_on_play=True,
    )
    return WorkerSettings(dispatcher=d)


@pytest.fixture
def ctx(config):
    q = TaskQueue(max_size=10)
    dispatcher = MagicMock()
    server = WebhookServer(
        config=config,
        task_queue=q,
        dispatcher=dispatcher,
        path_mapper=PathMapper(enabled=False, from_list="", to_list=""),
    )
    server.app.config["TESTING"] = True
    return server, q, dispatcher


# -- Plex --------------------------------------------------------------------


def plex_form(event, rating_key):
    return {
        "payload": json.dumps({"event": event, "Metadata": {"ratingKey": rating_key}}),
    }, {"User-Agent": "PlexMediaServer/1.0"}


class TestPlexWebhook:
    def test_library_new_enqueues_plex_task(self, ctx, config):
        server, q, _ = ctx
        data, headers = plex_form("library.new", "12345")
        resp = server.app.test_client().post("/plex", data=data, headers=headers)
        assert resp.status_code == 200
        assert q.size() == 1
        task = q.dequeue()
        assert task.plex_item_id == "12345"
        assert task.plex_server == config.dispatcher.plex_server
        assert task.file_path == ""  # resolved at dispatch time

    def test_media_play_enqueues(self, ctx):
        server, q, _ = ctx
        data, headers = plex_form("media.play", "67890")
        resp = server.app.test_client().post("/plex", data=data, headers=headers)
        assert resp.status_code == 200
        assert q.size() == 1

    def test_ignored_event_returns_200_empty(self, ctx):
        server, q, _ = ctx
        for event in ("item.exists", "item.added", "media.stop", "media.pause"):
            data, headers = plex_form(event, "1")
            resp = server.app.test_client().post("/plex", data=data, headers=headers)
            assert resp.status_code == 200
            assert resp.get_data(as_text=True) == ""
        assert q.size() == 0

    def test_event_gated_by_process_added_media(self, ctx):
        server, q, _ = ctx
        server.config.dispatcher.process_added_media = False
        data, headers = plex_form("library.new", "1")
        resp = server.app.test_client().post("/plex", data=data, headers=headers)
        assert resp.status_code == 200
        assert q.size() == 0

    def test_missing_plex_user_agent_rejected(self, ctx):
        server, q, _ = ctx
        data, _ = plex_form("library.new", "1")
        resp = server.app.test_client().post("/plex", data=data, headers={"User-Agent": "curl"})
        assert resp.status_code == 400
        assert q.size() == 0

    def test_missing_payload_field(self, ctx):
        server, _, _ = ctx
        resp = server.app.test_client().post(
            "/plex", data={}, headers={"User-Agent": "PlexMediaServer/1.0"}
        )
        assert resp.status_code == 400
        assert b"Missing payload field" in resp.data

    def test_invalid_json_payload(self, ctx):
        server, _, _ = ctx
        resp = server.app.test_client().post(
            "/plex",
            data={"payload": "{not json"},
            headers={"User-Agent": "PlexMediaServer/1.0"},
        )
        assert resp.status_code == 400
        assert b"Invalid JSON payload" in resp.data

    def test_missing_rating_key(self, ctx):
        server, q, _ = ctx
        data, headers = plex_form("library.new", "")
        resp = server.app.test_client().post("/plex", data=data, headers=headers)
        assert resp.status_code == 400
        assert b"Missing ratingKey" in resp.data

    def test_duplicate_returns_200_ok(self, ctx):
        server, q, _ = ctx
        client = server.app.test_client()
        data, headers = plex_form("library.new", "1")
        assert client.post("/plex", data=data, headers=headers).status_code == 200
        resp = client.post("/plex", data=data, headers=headers)
        assert resp.status_code == 200
        assert resp.get_data(as_text=True) == "OK"
        assert q.size() == 1

    def test_get_request_rejected(self, ctx):
        server, _, _ = ctx
        resp = server.app.test_client().get("/plex")
        assert resp.status_code == 400
        assert b"incorrectly via a GET" in resp.data


# -- Jellyfin ------------------------------------------------------------------


JELLYFIN_ADDED = (
    "NotificationType=ItemAdded&ItemId=abc123def456&ItemType=Episode&ItemName=Test%20Episode"
    "&SeriesName=Test%20Show&SeasonNumber=1&EpisodeNumber=1"
)
JF_HEADERS = {"User-Agent": "Jellyfin-Server/10.8.0", "Content-Type": "application/x-www-form-urlencoded"}


class TestJellyfinWebhook:
    def test_item_added_enqueues(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post("/jellyfin", data=JELLYFIN_ADDED, headers=JF_HEADERS)
        assert resp.status_code == 200
        assert q.size() == 1
        task = q.dequeue()
        assert task.jellyfin_item_id == "abc123def456"

    def test_playback_start_enqueues(self, ctx):
        server, q, _ = ctx
        data = "NotificationType=PlaybackStart&ItemId=xyz789abc123&ItemType=Episode"
        resp = server.app.test_client().post("/jellyfin", data=data, headers=JF_HEADERS)
        assert resp.status_code == 200
        assert q.size() == 1

    def test_ignored_notification_type(self, ctx):
        server, q, _ = ctx
        data = "NotificationType=ItemRemoved&ItemId=1"
        resp = server.app.test_client().post("/jellyfin", data=data, headers=JF_HEADERS)
        assert resp.status_code == 200
        assert q.size() == 0

    def test_wrong_user_agent_rejected(self, ctx):
        server, _, _ = ctx
        resp = server.app.test_client().post("/jellyfin", data=JELLYFIN_ADDED, headers={"User-Agent": "curl"})
        assert resp.status_code == 400

    def test_missing_item_id(self, ctx):
        server, _, _ = ctx
        resp = server.app.test_client().post(
            "/jellyfin",
            data="NotificationType=ItemAdded",
            headers=JF_HEADERS,
        )
        assert resp.status_code == 400
        assert b"Missing ItemId" in resp.data

    def test_duplicate_returns_200_empty(self, ctx):
        server, q, _ = ctx
        client = server.app.test_client()
        assert client.post("/jellyfin", data=JELLYFIN_ADDED, headers=JF_HEADERS).status_code == 200
        resp = client.post("/jellyfin", data=JELLYFIN_ADDED, headers=JF_HEADERS)
        assert resp.status_code == 200
        assert resp.get_data(as_text=True) == ""


# -- Emby ------------------------------------------------------------------------


EMBY_PAYLOAD = (
    'data={"Event":"library.new","Item":{"Name":"Test Episode",'
    '"Path":"/media/TV/Show/S01E01.mkv","Type":"Episode","ServerId":"abc123","Id":"item123"},'
    '"Server":{"Name":"Test Emby Server","Id":"server123"}}'
)


class TestEmbyWebhook:
    def test_library_new_enqueues_with_path(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post(
            "/emby", data=EMBY_PAYLOAD, content_type="application/x-www-form-urlencoded"
        )
        assert resp.status_code == 200
        task = q.dequeue()
        assert task.file_path == "/media/TV/Show/S01E01.mkv"

    def test_notification_test(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post(
            "/emby",
            data='data={"Event":"system.notificationtest","Server":{"Name":"x"}}',
            content_type="application/x-www-form-urlencoded",
        )
        assert resp.status_code == 200
        assert b"Notification test received successfully!" in resp.data
        assert q.size() == 0

    def test_empty_data_accepted(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post("/emby", data={})
        assert resp.status_code == 200
        assert q.size() == 0

    def test_missing_path(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post(
            "/emby",
            data='data={"Event":"library.new","Item":{"Name":"x"}}',
            content_type="application/x-www-form-urlencoded",
        )
        assert resp.status_code == 400
        assert b"Missing file path" in resp.data


# -- Tautulli ---------------------------------------------------------------------


class TestTautulliWebhook:
    def test_added_enqueues(self, ctx):
        server, q, _ = ctx
        data = "event=added&file=/media/TV/Show/S01E01.mkv&title=Test%20Episode"
        resp = server.app.test_client().post(
            "/tautulli",
            data=data,
            headers={"source": "Tautulli", "Content-Type": "application/x-www-form-urlencoded"},
        )
        assert resp.status_code == 200
        assert q.dequeue().file_path == "/media/TV/Show/S01E01.mkv"

    def test_played_enqueues(self, ctx):
        server, q, _ = ctx
        data = "event=played&file=/media/Movies/Movie.mkv"
        resp = server.app.test_client().post(
            "/tautulli",
            data=data,
            headers={"source": "Tautulli", "Content-Type": "application/x-www-form-urlencoded"},
        )
        assert resp.status_code == 200
        assert q.size() == 1

    def test_wrong_source_header(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post(
            "/tautulli",
            data="event=added&file=/x.mkv",
            headers={"source": "Other", "Content-Type": "application/x-www-form-urlencoded"},
        )
        assert resp.status_code == 400
        assert q.size() == 0

    def test_missing_file(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post(
            "/tautulli",
            data="event=added",
            headers={"source": "Tautulli", "Content-Type": "application/x-www-form-urlencoded"},
        )
        assert resp.status_code == 400
        assert b"Missing file field" in resp.data


# -- status/health ------------------------------------------------------------------


class TestStatusEndpoints:
    def test_status(self, ctx):
        server, _, _ = ctx
        resp = server.app.test_client().get("/status")
        assert resp.status_code == 200
        assert resp.json["status"] == "operational"

    def test_health_and_ready(self, ctx):
        server, _, _ = ctx
        assert server.app.test_client().get("/health").json["status"] == "alive"
        assert server.app.test_client().get("/ready").json["status"] == "ready"

    def test_queue_status(self, ctx):
        server, q, _ = ctx
        q.enqueue(Task(file_path="/a.mkv"))
        data = server.app.test_client().get("/queue/status").json
        assert data["queue_size"] == 1
        assert data["is_idle"] is False


# -- ASR segment formatting ----------------------------------------------------------


class TestFormatSegments:
    SEGS = [
        {"start": 0.0, "end": 2.5, "text": " Hello world "},
        {"start": 3.0, "end": 5.25, "text": "Second line"},
    ]

    def test_srt(self):
        out = format_segments(self.SEGS, "srt")
        assert "1\n00:00:00,000 --> 00:00:02,500\nHello world" in out
        assert "2\n00:00:03,000 --> 00:00:05,250\nSecond line" in out

    def test_vtt(self):
        out = format_segments(self.SEGS, "vtt")
        assert out.startswith("WEBVTT")
        assert "00:00:00.000 --> 00:00:02.500" in out

    def test_json(self):
        out = format_segments(self.SEGS, "json")
        parsed = json.loads(out)
        assert parsed[0]["text"] == " Hello world "

    def test_txt_and_tsv_and_lrc(self):
        assert "Hello world\nSecond line" == format_segments(self.SEGS, "txt")
        assert "0.000\t2.500\tHello world" in format_segments(self.SEGS, "tsv")
        assert "[00:00.00]Hello world" in format_segments(self.SEGS, "lrc")


# -- Review-fix regressions (PR #5 skeptical review) ---------------------------


class TestParityFixes:
    def test_non_dict_plex_payload_rejected(self, ctx):
        server, q, _ = ctx
        headers = {"User-Agent": "PlexMediaServer/1.0"}
        for bad in ("[]", "null", '"s"', "42"):
            resp = server.app.test_client().post(
                "/plex", data={"payload": bad}, headers=headers
            )
            assert resp.status_code == 400, f"payload={bad}"
            assert b"Invalid JSON payload" in resp.data
        assert q.size() == 0

    def test_non_dict_emby_data_rejected(self, ctx):
        server, q, _ = ctx
        for bad in ("3", '"s"', "[]"):
            resp = server.app.test_client().post(
                "/emby", data=f"data={bad}", content_type="application/x-www-form-urlencoded"
            )
            assert resp.status_code == 400, f"data={bad}"
        assert q.size() == 0

    def test_scalar_metadata_rejected(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post(
            "/plex",
            data={"payload": '{"event":"library.new","Metadata":"oops"}'},
            headers={"User-Agent": "PlexMediaServer/1.0"},
        )
        assert resp.status_code == 400
        assert b"Invalid JSON payload" in resp.data

    def test_int_rating_key_rejected_like_go(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post(
            "/plex",
            data={"payload": '{"event":"library.new","Metadata":{"ratingKey":12345}}'},
            headers={"User-Agent": "PlexMediaServer/1.0"},
        )
        assert resp.status_code == 400
        assert b"Invalid JSON payload" in resp.data

    def test_webhook_tautulli_route_registered(self, ctx):
        server, q, _ = ctx
        resp = server.app.test_client().post(
            "/webhook/tautulli",
            data="event=added&file=/media/x.mkv",
            headers={"source": "Tautulli", "Content-Type": "application/x-www-form-urlencoded"},
        )
        assert resp.status_code == 200
        assert q.size() == 1

    def test_no_enabled_gate_parity(self, ctx):
        """Go has no *_ENABLED gate on webhook ingestion — tasks enqueue and
        path resolution fails at dispatch. Ports must not 400."""
        server, q, _ = ctx
        server.config.dispatcher.plex_enabled = False
        server.config.dispatcher.jellyfin_enabled = False
        data, headers = plex_form("library.new", "1")
        assert server.app.test_client().post("/plex", data=data, headers=headers).status_code == 200
        assert (
            server.app.test_client().post("/jellyfin", data=JELLYFIN_ADDED, headers=JF_HEADERS).status_code
            == 200
        )
        assert q.size() == 2

    def test_go_error_strings_exact(self, ctx):
        server, _, _ = ctx
        client = server.app.test_client()
        resp = client.post("/plex", data={}, headers={"User-Agent": "curl"})
        assert resp.json["error"] == (
            "This doesn't appear to be a properly configured Plex webhook, "
            "please review the instructions again"
        )
        resp = client.post(
            "/jellyfin", data="NotificationType=ItemAdded", headers={"User-Agent": "curl"}
        )
        assert "please review the instructions again" in resp.json["error"]
        resp = client.post(
            "/tautulli",
            data="event=added&file=/x.mkv",
            headers={"source": "Other", "Content-Type": "application/x-www-form-urlencoded"},
        )
        assert "please review the instructions again" in resp.json["error"]

    def test_probe_aliases(self, ctx):
        server, _, _ = ctx
        client = server.app.test_client()
        assert client.get("/healthz").json["status"] == "alive"
        assert client.get("/readyz").json["status"] == "ready"
        assert client.get("/live").json["status"] == "alive"
        assert "uptime_seconds" in client.get("/livez").json

    def test_queue_processing_and_history_endpoints(self, ctx):
        server, q, _ = ctx
        q.enqueue(Task(file_path="/a.mkv"))
        client = server.app.test_client()
        assert client.get("/queue/processing").json["processing"] == 0
        t = q.dequeue()
        assert client.get("/queue/processing").json["processing"] == 1
        q.mark_done(t)
        history = client.get("/queue/history").json["history"]
        assert history[-1]["status"] == "done"
        task_id = history[-1]["task"]["id"]
        assert client.get(f"/tasks/{task_id}").json["status"] == "done"
        assert client.get("/tasks/does-not-exist").status_code == 404
