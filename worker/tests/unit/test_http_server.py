"""
Unit tests for HTTP health check server.

Following TDD workflow:
1. Write tests FIRST (these tests should FAIL initially)
2. Run tests to confirm failures
3. Implement features to make tests pass
4. Run tests again to confirm success
"""

import pytest
import time
import os
from unittest.mock import Mock, patch, MagicMock
from flask import Flask


# Import the module we're testing
# Note: These imports will FAIL initially (that's correct for TDD!)
try:
    from src.http_server import app, init_health_server, _service

    IMPORTS_WORK = True
except ImportError:
    IMPORTS_WORK = False
    pytest.skip("HTTP server module not yet implemented", allow_module_level=True)


@pytest.fixture
def mock_service():
    """Create a mock TranscriptionService for testing"""
    service = Mock()
    service.stats = {
        "memory_mb": 1024,
        "jobs_active": 2,
        "jobs_processed": 100,
        "jobs_failed": 5,
        "consecutive_errors": 0,
        "last_job_timestamp": int(time.time()),
    }
    service.start_time = time.time() - 3600  # 1 hour uptime

    # Mock config
    service.config = Mock()
    service.config.system = Mock()
    service.config.system.memory_threshold_mb = 8000
    service.config.whisper = Mock()
    service.config.whisper.model_name = "large-v3"
    service.config.whisper.model_path = "/tmp/models"
    service.config.version = "1.0.0"

    # Mock model manager
    service.model_manager = Mock()
    service.model_manager.is_loaded = Mock(return_value=True)

    return service


@pytest.fixture
def client(mock_service):
    """Create Flask test client"""
    app.config["TESTING"] = True
    init_health_server(mock_service)
    with app.test_client() as client:
        yield client


class TestHealthEndpoint:
    """Test /health endpoint (liveness probe)"""

    def test_health_returns_200(self, client):
        """Happy path: Health endpoint should return 200"""
        response = client.get("/health")
        assert response.status_code == 200

    def test_health_returns_json(self, client):
        """Happy path: Health endpoint should return JSON"""
        response = client.get("/health")
        assert response.content_type == "application/json"

    def test_health_contains_status(self, client):
        """Happy path: Health response should contain status field"""
        response = client.get("/health")
        data = response.get_json()
        assert "status" in data
        assert data["status"] == "alive"

    def test_health_contains_timestamp(self, client):
        """Happy path: Health response should contain timestamp"""
        response = client.get("/health")
        data = response.get_json()
        assert "timestamp" in data
        assert isinstance(data["timestamp"], int)
        assert data["timestamp"] > 0

    def test_health_timestamp_is_recent(self, client):
        """Happy path: Timestamp should be within last 5 seconds"""
        response = client.get("/health")
        data = response.get_json()
        now = int(time.time())
        assert abs(data["timestamp"] - now) < 5

    def test_health_only_accepts_get(self, client):
        """Unhappy path: POST should not be allowed"""
        response = client.post("/health")
        assert response.status_code == 405  # Method Not Allowed

    def test_health_rejects_put(self, client):
        """Unhappy path: PUT should not be allowed"""
        response = client.put("/health")
        assert response.status_code == 405

    def test_health_rejects_delete(self, client):
        """Unhappy path: DELETE should not be allowed"""
        response = client.delete("/health")
        assert response.status_code == 405


class TestReadyEndpoint:
    """Test /ready endpoint (readiness probe)"""

    def test_ready_returns_200_when_healthy(self, client, mock_service):
        """Happy path: Ready endpoint should return 200 when service is healthy"""
        response = client.get("/ready")
        assert response.status_code == 200

    def test_ready_returns_json(self, client):
        """Happy path: Ready endpoint should return JSON"""
        response = client.get("/ready")
        assert response.content_type == "application/json"

    def test_ready_contains_status_ready(self, client):
        """Happy path: Ready response should contain status=ready"""
        response = client.get("/ready")
        data = response.get_json()
        assert data["status"] == "ready"

    def test_ready_includes_memory_info(self, client):
        """Happy path: Ready response should include memory information"""
        response = client.get("/ready")
        data = response.get_json()
        assert "memory_mb" in data
        assert isinstance(data["memory_mb"], (int, float))

    def test_ready_includes_job_info(self, client):
        """Happy path: Ready response should include job information"""
        response = client.get("/ready")
        data = response.get_json()
        assert "jobs_active" in data
        assert "model_loaded" in data
        assert "uptime_seconds" in data

    def test_ready_returns_503_when_service_not_initialized(self, client):
        """Unhappy path: Should return 503 when service is None"""
        # Temporarily set service to None
        import src.http_server as hs

        original = hs._service
        hs._service = None

        response = client.get("/ready")
        assert response.status_code == 503
        data = response.get_json()
        assert data["status"] == "not_ready"
        assert data["reason"] == "service_not_initialized"

        # Restore
        hs._service = original

    def test_ready_returns_503_when_memory_exceeded(self, client, mock_service):
        """Unhappy path: Should return 503 when memory threshold exceeded"""
        # Set memory above threshold
        mock_service.stats["memory_mb"] = 9000  # Above 8000 threshold

        response = client.get("/ready")
        assert response.status_code == 503
        data = response.get_json()
        assert data["status"] == "not_ready"
        assert data["reason"] == "memory_threshold_exceeded"
        assert "memory_mb" in data
        assert "threshold_mb" in data

    def test_ready_returns_503_when_too_many_errors(self, client, mock_service):
        """Unhappy path: Should return 503 when consecutive errors > 3"""
        mock_service.stats["consecutive_errors"] = 5

        response = client.get("/ready")
        assert response.status_code == 503
        data = response.get_json()
        assert data["status"] == "not_ready"
        assert data["reason"] == "too_many_consecutive_errors"
        assert data["consecutive_errors"] == 5

    @patch("os.path.exists")
    @patch("os.statvfs")
    def test_ready_returns_503_when_disk_space_low(
        self, mock_statvfs, mock_exists, client, mock_service
    ):
        """Unhappy path: Should return 503 when disk space < 500MB"""
        mock_exists.return_value = True

        # Mock statvfs to return low disk space (100MB)
        mock_stat = Mock()
        mock_stat.f_bavail = 100 * 1024  # blocks
        mock_stat.f_frsize = 1024  # block size = 1KB
        mock_statvfs.return_value = mock_stat

        response = client.get("/ready")
        assert response.status_code == 503
        data = response.get_json()
        assert data["status"] == "not_ready"
        assert data["reason"] == "insufficient_disk_space"
        assert "free_mb" in data
        assert "required_mb" in data

    def test_ready_only_accepts_get(self, client):
        """Unhappy path: POST should not be allowed"""
        response = client.post("/ready")
        assert response.status_code == 405


class TestMetricsEndpoint:
    """Test /metrics endpoint"""

    @patch("psutil.Process")
    def test_metrics_returns_200(self, mock_process_class, client):
        """Happy path: Metrics endpoint should return 200"""
        # Mock psutil.Process
        mock_process = Mock()
        mock_process.cpu_percent.return_value = 25.5
        mock_process.memory_info.return_value = Mock(rss=1024 * 1024 * 1024)  # 1GB
        mock_process_class.return_value = mock_process

        response = client.get("/metrics")
        assert response.status_code == 200

    @patch("psutil.Process")
    def test_metrics_returns_json(self, mock_process_class, client):
        """Happy path: Metrics endpoint should return JSON"""
        mock_process = Mock()
        mock_process.cpu_percent.return_value = 25.5
        mock_process.memory_info.return_value = Mock(rss=1024 * 1024 * 1024)
        mock_process_class.return_value = mock_process

        response = client.get("/metrics")
        assert response.content_type == "application/json"

    @patch("psutil.Process")
    def test_metrics_includes_resource_usage(self, mock_process_class, client):
        """Happy path: Metrics should include CPU and memory"""
        mock_process = Mock()
        mock_process.cpu_percent.return_value = 25.5
        mock_process.memory_info.return_value = Mock(rss=1024 * 1024 * 1024)
        mock_process_class.return_value = mock_process

        response = client.get("/metrics")
        data = response.get_json()

        assert "memory_mb" in data
        assert "cpu_percent" in data
        assert "disk_available_mb" in data
        assert data["cpu_percent"] == 25.5

    @patch("psutil.Process")
    def test_metrics_includes_model_state(self, mock_process_class, client):
        """Happy path: Metrics should include model information"""
        mock_process = Mock()
        mock_process.cpu_percent.return_value = 25.5
        mock_process.memory_info.return_value = Mock(rss=1024 * 1024 * 1024)
        mock_process_class.return_value = mock_process

        response = client.get("/metrics")
        data = response.get_json()

        assert "model_loaded" in data
        assert "model_name" in data
        assert data["model_name"] == "large-v3"

    @patch("psutil.Process")
    def test_metrics_includes_job_statistics(self, mock_process_class, client):
        """Happy path: Metrics should include job stats"""
        mock_process = Mock()
        mock_process.cpu_percent.return_value = 25.5
        mock_process.memory_info.return_value = Mock(rss=1024 * 1024 * 1024)
        mock_process_class.return_value = mock_process

        response = client.get("/metrics")
        data = response.get_json()

        assert "jobs_processed" in data
        assert "jobs_active" in data
        assert "jobs_failed" in data
        assert "consecutive_errors" in data
        assert "last_job_timestamp" in data

    def test_metrics_returns_503_when_service_not_initialized(self, client):
        """Unhappy path: Should return 503 when service is None"""
        import src.http_server as hs

        original = hs._service
        hs._service = None

        response = client.get("/metrics")
        assert response.status_code == 503
        data = response.get_json()
        assert "error" in data
        assert data["error"] == "service_not_initialized"

        hs._service = original

    def test_metrics_only_accepts_get(self, client):
        """Unhappy path: POST should not be allowed"""
        response = client.post("/metrics")
        assert response.status_code == 405


class TestInitHealthServer:
    """Test init_health_server function"""

    def test_init_sets_service_reference(self, mock_service):
        """Happy path: init_health_server should set global service reference"""
        init_health_server(mock_service)
        import src.http_server as hs

        assert hs._service is not None
        assert hs._service == mock_service

    def test_init_accepts_none_service(self):
        """Edge case: Should handle None service gracefully"""
        init_health_server(None)
        import src.http_server as hs

        assert hs._service is None
