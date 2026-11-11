"""Single-process subgen dispatcher entrypoint.

Replaces the Go orchestrator + gRPC worker pair with ONE Python process:
webhook listener (Flask) + in-process queue + in-process transcription via
the existing TranscriptionServicer/TranscriptionEngine.

Usage:
    python dispatcher_main.py

Configuration: the same WorkerSettings environment variables as the gRPC
worker, plus the DispatcherConfig section (PLEX_*, JELLYFIN_*, MONITOR,
TRANSCRIBE_FOLDERS, WEBHOOK_PORT, ...). The gRPC server is NOT started in
this mode.
"""

import logging
import os
import signal
import sys
from pathlib import Path

# Same PYTHONPATH contract as the gRPC worker entrypoint (src/:.:/pb).
WORKER_DIR = Path(__file__).resolve().parent
for p in (str(WORKER_DIR / "src"), str(WORKER_DIR), str(WORKER_DIR / "pb")):
    if p not in sys.path:
        sys.path.insert(0, p)

from config.settings import load_config  # noqa: E402
from dispatcher.pathmap import PathMapper  # noqa: E402
from dispatcher.queue import TaskQueue  # noqa: E402
from dispatcher.runner import Dispatcher  # noqa: E402
from dispatcher.webhooks import WebhookServer  # noqa: E402
from grpc_server.service import TranscriptionServicer  # noqa: E402
from utils.logging import setup_logging  # noqa: E402

logger = logging.getLogger(__name__)

QUEUE_STATE_PATH = os.environ.get("QUEUE_STATE_PATH", "/tmp/subgen-queue-state.json")


def main() -> None:
    config = load_config()
    setup_logging(config.system.debug, config.system.log_format)
    logger.info(
        "Starting subgen dispatcher: port=%d plex=%s jellyfin=%s monitor=%s",
        config.dispatcher.webhook_port,
        config.dispatcher.plex_enabled,
        config.dispatcher.jellyfin_enabled,
        config.dispatcher.monitor_enabled,
    )

    servicer = TranscriptionServicer(config)
    task_queue = TaskQueue(
        max_size=config.dispatcher.queue_max_size,
        persist_path=QUEUE_STATE_PATH,
    )
    dispatcher = Dispatcher(config, servicer, task_queue)
    path_mapper = PathMapper(
        enabled=config.system.use_path_mapping,
        from_list=config.system.path_mapping_from,
        to_list=config.system.path_mapping_to,
    )
    server = WebhookServer(config, task_queue, dispatcher, path_mapper)

    dispatcher.start()

    def shutdown(_sig=None, _frame=None):
        # NOTE: we deliberately do NOT join the worker thread or run normal
        # interpreter teardown here. If a ctranslate2 transcription is
        # mid-flight, joining + interpreter finalization segfaults (exit 139,
        # reproduced under review). Instead: stop scheduling, and hard-exit.
        # Any task still in flight is persisted in the queue state file and
        # recovered (requeued) on next start — at-least-once delivery.
        logger.info("Shutdown signal received; stopping dispatcher")
        dispatcher._stop.set()
        os._exit(0)

    signal.signal(signal.SIGTERM, shutdown)
    signal.signal(signal.SIGINT, shutdown)

    server.run(port=config.dispatcher.webhook_port)


if __name__ == "__main__":
    main()
