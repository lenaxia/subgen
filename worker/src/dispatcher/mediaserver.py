"""Plex and Jellyfin API clients — ports of internal/mediaserver/{plex,jellyfin}.go.

Used to (a) resolve a webhook item id to a media file path at dispatch time
and (b) ask the server to refresh metadata after a successful transcription.
"""

import logging
import xml.etree.ElementTree as ET
from typing import Optional, Tuple

import requests

logger = logging.getLogger(__name__)

DEFAULT_TIMEOUT = 30


class MediaServerError(Exception):
    pass


class PlexClient:
    def __init__(self, server: str, token: str):
        self.server = server.rstrip("/")
        self.token = token
        self._session = requests.Session()

    def _headers(self) -> dict:
        return {"X-Plex-Token": self.token, "Accept": "application/xml"}

    def get_file_path(self, item_id: str) -> str:
        """Resolve a ratingKey to its media file path.

        GET {server}/library/metadata/{id} -> MediaContainer/Video/Media/Part@file
        """
        url = f"{self.server}/library/metadata/{item_id}"
        try:
            resp = self._session.get(url, headers=self._headers(), timeout=DEFAULT_TIMEOUT)
            resp.raise_for_status()
        except requests.RequestException as e:
            raise MediaServerError(f"Plex metadata fetch failed for item {item_id}: {e}") from e

        try:
            root = ET.fromstring(resp.content)
            part = root.find("./Video/Media/Part")
            if part is None:
                raise MediaServerError(f"Plex item {item_id}: no Media/Part element")
            file_path = part.get("file", "")
            if not file_path:
                raise MediaServerError(f"Plex item {item_id}: empty file attribute")
            return file_path
        except ET.ParseError as e:
            raise MediaServerError(f"Plex item {item_id}: invalid XML response: {e}") from e

    def refresh_metadata(self, item_id: str) -> bool:
        """PUT /library/metadata/{id}/refresh — best effort, warnings only."""
        url = f"{self.server}/library/metadata/{item_id}/refresh"
        try:
            resp = self._session.put(url, headers=self._headers(), timeout=DEFAULT_TIMEOUT)
            if resp.status_code != 200:
                logger.warning("Plex refresh for %s returned %d", item_id, resp.status_code)
                return False
            return True
        except requests.RequestException as e:
            logger.warning("Plex refresh for %s failed: %s", item_id, e)
            return False


class JellyfinClient:
    def __init__(self, server: str, token: str):
        self.server = server.rstrip("/")
        self.token = token
        self._session = requests.Session()
        self._admin_user_id: Optional[str] = None

    def _headers(self) -> dict:
        return {"Authorization": f"MediaBrowser Token={self.token}"}

    def _get_admin_user_id(self) -> str:
        """First administrator user, cached (port of jellyfin.go:129-187)."""
        if self._admin_user_id:
            return self._admin_user_id
        try:
            resp = self._session.get(
                f"{self.server}/Users", headers=self._headers(), timeout=DEFAULT_TIMEOUT
            )
            resp.raise_for_status()
        except requests.RequestException as e:
            raise MediaServerError(f"Jellyfin user listing failed: {e}") from e
        for user in resp.json():
            if user.get("Policy", {}).get("IsAdministrator"):
                self._admin_user_id = user["Id"]
                return self._admin_user_id
        raise MediaServerError("Jellyfin: no administrator user found")

    def get_file_path(self, item_id: str) -> str:
        """Resolve an item id to its file path via an admin user's item view."""
        user_id = self._get_admin_user_id()
        url = f"{self.server}/Users/{user_id}/Items/{item_id}"
        try:
            resp = self._session.get(url, headers=self._headers(), timeout=DEFAULT_TIMEOUT)
            resp.raise_for_status()
        except requests.RequestException as e:
            raise MediaServerError(f"Jellyfin item fetch failed for {item_id}: {e}") from e
        path = resp.json().get("Path", "")
        if not path:
            raise MediaServerError(f"Jellyfin item {item_id}: empty Path")
        return path

    def refresh_metadata(self, item_id: str) -> bool:
        """POST /Items/{id}/Refresh — expects 204, best effort."""
        url = f"{self.server}/Items/{item_id}/Refresh"
        try:
            resp = self._session.post(
                url,
                headers=self._headers(),
                params={"MetadataRefreshMode": "FullRefresh"},
                timeout=DEFAULT_TIMEOUT,
            )
            if resp.status_code != 204:
                logger.warning("Jellyfin refresh for %s returned %d", item_id, resp.status_code)
                return False
            return True
        except requests.RequestException as e:
            logger.warning("Jellyfin refresh for %s failed: %s", item_id, e)
            return False
