"""In-process task queue for the single-process dispatcher.

Ported from orchestrator/internal/queue with three deliberate improvements:
- requeue-not-delete: a failed task goes back on the queue with an attempt
  counter (the Go CleanupStaleTasks path dropped tasks forever);
- persistence: queue state snapshots to JSON so an orchestrator restart no
  longer loses queued work (audio_content is excluded — /asr uploads must be
  re-posted);
- dedup key normalization: file paths are os.path.normpath'ed so the same
  file via different separators does not enqueue twice.
"""

import hashlib
import heapq
import json
import logging
import os
import threading
import time
from collections import deque
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)


class DuplicateTaskError(Exception):
    """Raised when a task with the same identity is already queued or processing."""


class QueueFullError(Exception):
    """Raised when the queue has reached its maximum size."""


# Lower value = dequeued first. detect_language and asr are interactive
# (a human or Bazarr is waiting), transcribe is background batch.
PRIORITY_DETECT_LANGUAGE = 0
PRIORITY_ASR = 1
PRIORITY_TRANSCRIBE = 2

TYPE_PRIORITY = {
    "detect_language": PRIORITY_DETECT_LANGUAGE,
    "asr": PRIORITY_ASR,
    "transcribe": PRIORITY_TRANSCRIBE,
}


def _norm_path(path: str) -> str:
    return os.path.normpath(path) if path else path


def compute_task_id(
    file_path: str = "",
    plex_server: str = "",
    plex_item_id: str = "",
    jellyfin_server: str = "",
    jellyfin_item_id: str = "",
    audio_content: Optional[bytes] = None,
) -> str:
    """Stable identity for dedup, mirroring the Go Task.ID derivation.

    Priority: file path > plex server:item > jellyfin server:item > audio hash.
    """
    if file_path:
        id_string = _norm_path(file_path)
    elif plex_item_id:
        id_string = f"plex:{plex_server}:{plex_item_id}"
    elif jellyfin_item_id:
        id_string = f"jellyfin:{jellyfin_server}:{jellyfin_item_id}"
    elif audio_content is not None:
        id_string = f"asr:content:{hashlib.sha256(audio_content).hexdigest()}"
    else:
        id_string = ""
    return hashlib.sha256(id_string.encode("utf-8")).hexdigest()


@dataclass
class Task:
    """One unit of dispatcher work."""

    task_type: str = "transcribe"  # transcribe | asr | detect_language
    file_path: str = ""  # resolved media path ("" for plex/jellyfin until dispatch)
    transcription_type: str = "transcribe"  # transcribe | translate
    force_language: str = ""

    # Media-server resolution metadata (Plex/Jellyfin tasks start without a path)
    plex_item_id: str = ""
    plex_server: str = ""
    plex_token: str = ""
    jellyfin_item_id: str = ""
    jellyfin_server: str = ""
    jellyfin_token: str = ""

    # ASR (/asr endpoint) upload
    audio_content: Optional[bytes] = None
    asr_options: Dict[str, str] = field(default_factory=dict)
    result_queue: Optional[Any] = None  # queue.Queue for blocking /asr callers

    id: str = ""
    queued_at: float = field(default_factory=time.time)
    started_at: float = 0.0
    attempts: int = 0
    last_error: str = ""

    def __post_init__(self) -> None:
        if not self.id:
            self.id = compute_task_id(
                file_path=self.file_path,
                plex_server=self.plex_server,
                plex_item_id=self.plex_item_id,
                jellyfin_server=self.jellyfin_server,
                jellyfin_item_id=self.jellyfin_item_id,
                audio_content=self.audio_content,
            )
        if self.task_type not in TYPE_PRIORITY:
            self.task_type = "transcribe"

    @property
    def priority(self) -> int:
        return TYPE_PRIORITY[self.task_type]

    def to_persisted(self) -> Dict[str, Any]:
        """JSON-safe snapshot (audio_content excluded: too large to persist)."""
        return {
            "id": self.id,
            "task_type": self.task_type,
            "file_path": self.file_path,
            "transcription_type": self.transcription_type,
            "force_language": self.force_language,
            "plex_item_id": self.plex_item_id,
            "plex_server": self.plex_server,
            "plex_token": self.plex_token,
            "jellyfin_item_id": self.jellyfin_item_id,
            "jellyfin_server": self.jellyfin_server,
            "jellyfin_token": self.jellyfin_token,
            "asr_options": self.asr_options,
            "queued_at": self.queued_at,
            "started_at": self.started_at,
            "attempts": self.attempts,
            "last_error": self.last_error,
        }

    @classmethod
    def from_persisted(cls, data: Dict[str, Any]) -> "Task":
        task = cls(
            task_type=data.get("task_type", "transcribe"),
            file_path=data.get("file_path", ""),
            transcription_type=data.get("transcription_type", "transcribe"),
            force_language=data.get("force_language", ""),
            plex_item_id=data.get("plex_item_id", ""),
            plex_server=data.get("plex_server", ""),
            plex_token=data.get("plex_token", ""),
            jellyfin_item_id=data.get("jellyfin_item_id", ""),
            jellyfin_server=data.get("jellyfin_server", ""),
            jellyfin_token=data.get("jellyfin_token", ""),
            asr_options=data.get("asr_options", {}),
            queued_at=data.get("queued_at", time.time()),
            started_at=data.get("started_at", 0.0),
            attempts=data.get("attempts", 0),
            last_error=data.get("last_error", ""),
        )
        task.id = data.get("id") or task.id
        return task


class TaskQueue:
    """Deduplicating priority queue with persistence and bounded size.

    Thread-safe. State transitions:
        enqueue -> (queued) -> dequeue -> (processing) -> mark_done/mark_failed
        mark_failed with attempts remaining -> requeue -> (queued)
    """

    HISTORY_SIZE = 100

    def __init__(
        self,
        max_size: int = 1000,
        persist_path: Optional[str] = None,
        history_size: int = HISTORY_SIZE,
    ):
        self.max_size = max_size
        self.persist_path = persist_path
        self._lock = threading.RLock()
        self._queued: Dict[str, Task] = {}
        self._heap: List[Any] = []  # (priority, queued_at, seq, id)
        self._seq = 0
        self._processing: Dict[str, Task] = {}
        self._history: deque = deque(maxlen=history_size)
        if persist_path:
            self._load()

    # -- core operations ----------------------------------------------------

    def enqueue(self, task: Task) -> Task:
        """Add a task. Raises DuplicateTaskError / QueueFullError."""
        with self._lock:
            if task.id in self._queued or task.id in self._processing:
                raise DuplicateTaskError(task.id)
            if len(self._heap) >= self.max_size:
                raise QueueFullError(
                    f"queue at max size {self.max_size}; task {task.id[:12]} rejected"
                )
            self._queued[task.id] = task
            self._seq += 1
            heapq.heappush(self._heap, (task.priority, task.queued_at, self._seq, task.id))
            logger.debug("Enqueued %s task %s (queue size %d)", task.task_type, task.id[:12], len(self._heap))
            self._persist_locked()
            return task

    def dequeue(self) -> Optional[Task]:
        """Pop the highest-priority task into processing. None when empty."""
        with self._lock:
            while self._heap:
                _, _, _, task_id = heapq.heappop(self._heap)
                task = self._queued.pop(task_id, None)
                if task is None:
                    continue  # stale heap entry (task re-keyed/removed)
                task.started_at = time.time()
                self._processing[task_id] = task
                self._persist_locked()
                return task
            return None

    def requeue(self, task: Task) -> None:
        """Return a processing task to the queue (retry path).

        Keeps the attempt count; a fresh queued_at so it orders behind newer
        work of the same priority. Re-deduped: a re-enqueued duplicate of the
        same identity wins the race and the retry is dropped as duplicate.
        """
        with self._lock:
            if self._processing.pop(task.id, None) is None:
                logger.debug("Requeue ignored: task %s not processing", task.id[:12])
                return
            task.queued_at = time.time()
            self._queued[task.id] = task
            self._seq += 1
            heapq.heappush(self._heap, (task.priority, task.queued_at, self._seq, task.id))
            logger.info(
                "Requeued %s task %s (attempt %d)",
                task.task_type,
                task.id[:12],
                task.attempts,
            )
            self._persist_locked()

    def mark_done(self, task: Task) -> None:
        with self._lock:
            self._processing.pop(task.id, None)
            self._history.append(
                {"task": task.to_persisted(), "status": "done", "finished_at": time.time()}
            )
            self._persist_locked()

    def mark_failed(self, task: Task, error: str) -> None:
        with self._lock:
            self._processing.pop(task.id, None)
            task.last_error = error
            self._history.append(
                {
                    "task": task.to_persisted(),
                    "status": "failed",
                    "error": error,
                    "finished_at": time.time(),
                }
            )
            self._persist_locked()

    def remove_processing(self, task: Task) -> None:
        """Drop a processing task without history (used for poisoned tasks)."""
        with self._lock:
            self._processing.pop(task.id, None)
            self._persist_locked()

    # -- inspection ----------------------------------------------------------

    def size(self) -> int:
        with self._lock:
            return len(self._heap)

    def processing_size(self) -> int:
        with self._lock:
            return len(self._processing)

    def is_idle(self) -> bool:
        with self._lock:
            return not self._heap and not self._processing

    def is_duplicate(self, task_id: str) -> bool:
        with self._lock:
            return task_id in self._queued or task_id in self._processing

    def history(self) -> List[Dict[str, Any]]:
        with self._lock:
            return list(self._history)

    # -- persistence ---------------------------------------------------------

    def _persist_locked(self) -> None:
        if not self.persist_path:
            return
        snapshot = {
            "queued": [t.to_persisted() for t in self._queued.values()],
            "processing": [t.to_persisted() for t in self._processing.values()],
        }
        tmp = self.persist_path + ".tmp"
        try:
            with open(tmp, "w", encoding="utf-8") as f:
                json.dump(snapshot, f)
            os.replace(tmp, self.persist_path)
        except Exception as e:  # persistence must never break the queue
            logger.warning("Queue persistence failed: %s", e)

    def _load(self) -> None:
        try:
            with open(self.persist_path, "r", encoding="utf-8") as f:
                snapshot = json.load(f)
        except FileNotFoundError:
            return
        except Exception as e:
            logger.warning("Queue persistence unreadable, starting empty: %s", e)
            return
        for data in snapshot.get("processing", []):
            # The worker that owned this task is gone (restart while it was
            # mid-job). Restoring it as "processing" would orphan it forever:
            # nothing dequeues from the processing dict, re-enqueueing the
            # same file would be rejected as duplicate, and is_idle() would
            # never go True. Recovery: put it back on the queue — the task's
            # attempts count is preserved, and workers are idempotent
            # (skip-if-subtitle-exists), so a redone job is safe.
            task = Task.from_persisted(data)
            task.started_at = 0.0
            if task.id in self._queued:
                continue
            self._queued[task.id] = task
            self._seq += 1
            heapq.heappush(self._heap, (task.priority, task.queued_at, self._seq, task.id))
            logger.warning(
                "Recovered in-flight task %s from previous run (attempt %d); requeued",
                task.id[:12],
                task.attempts,
            )
        for data in snapshot.get("queued", []):
            task = Task.from_persisted(data)
            if task.id in self._queued or task.id in self._processing:
                continue
            self._queued[task.id] = task
            self._seq += 1
            heapq.heappush(self._heap, (task.priority, task.queued_at, self._seq, task.id))
        if self._queued or self._processing:
            logger.info(
                "Restored queue from %s: %d queued, %d processing",
                self.persist_path,
                len(self._queued),
                len(self._processing),
            )
