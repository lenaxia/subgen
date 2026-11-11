"""Media path mapping — port of orchestrator/internal/util/path_mapper.go.

Maps container-internal media paths to host/library paths using ordered
prefix pairs (USE_PATH_MAPPING / PATH_MAPPING_FROM / PATH_MAPPING_TO).
"""

import logging
import os
from typing import List, Tuple

logger = logging.getLogger(__name__)


class PathMapper:
    def __init__(self, enabled: bool, from_list: str, to_list: str):
        self.enabled = enabled
        self.pairs: List[Tuple[str, str]] = []
        if not enabled:
            return
        froms = [p.strip().rstrip("/") for p in from_list.split(",") if p.strip()]
        tos = [p.strip().rstrip("/") for p in to_list.split(",") if p.strip()]
        if len(froms) != len(tos):
            raise ValueError(
                f"PATH_MAPPING_FROM/TO count mismatch: {len(froms)} vs {len(tos)}"
            )
        self.pairs = list(zip(froms, tos))
        if not self.pairs:
            logger.warning("USE_PATH_MAPPING=true but no mapping pairs configured")

    def map(self, path: str) -> str:
        """Return the mapped path. Unmatched paths are returned unchanged.

        Raises FileNotFoundError when a mapping matched but the target does
        not exist on this filesystem (mirrors the Go os.Stat validation) —
        in single-process mode the dispatcher shares the worker's mounts, so
        a mapped path must exist.
        """
        if not self.enabled or not path:
            return path
        for src, dst in self.pairs:
            if path.startswith(src):
                mapped = dst + path[len(src):]
                if not os.path.exists(mapped):
                    raise FileNotFoundError(
                        f"Mapped path does not exist: {path} -> {mapped}"
                    )
                logger.debug("Path mapped: %s -> %s", path, mapped)
                return mapped
        return path
