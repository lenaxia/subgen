"""
HTTP health check server for Kubernetes and Docker health probes.

Runs alongside gRPC server on separate port (8080) to provide simple HTTP
endpoints for liveness and readiness checks. This is required because K8s
HTTP probes are simpler to configure than gRPC probes.

Endpoints:
- GET /health - Liveness probe (is worker alive?)
- GET /ready - Readiness probe (can worker accept tasks?)
- GET /metrics - Detailed metrics for monitoring
"""

import logging
import time
import psutil
import os
from flask import Flask, jsonify
from typing import TYPE_CHECKING, Optional
from werkzeug.serving import make_server
import threading

if TYPE_CHECKING:
    from grpc_server.service import TranscriptionService

logger = logging.getLogger(__name__)

app = Flask(__name__)
_service: Optional["TranscriptionService"] = None
_server: Optional["ServerThread"] = None


class ServerThread(threading.Thread):
    """Thread wrapper for Flask server to enable graceful shutdown"""

    def __init__(self, host: str, port: int):
        super().__init__(name="HealthServer", daemon=False)
        self.host = host
        self.port = port
        self.server = make_server(host, port, app, threaded=True)
        self.ctx = app.app_context()
        self.ctx.push()

    def run(self):
        logger.info(f"Health server thread started on {self.host}:{self.port}")
        self.server.serve_forever()

    def shutdown(self):
        logger.info("Shutting down HTTP health server...")
        self.server.shutdown()
        logger.info("HTTP health server stopped")


def init_health_server(service: "TranscriptionService"):
    """Initialize health server with reference to gRPC service"""
    global _service
    _service = service
    logger.info("Health server initialized")


@app.route("/health", methods=["GET"])
def health():
    """
    Liveness probe - is the worker alive?

    Returns 200 if process is running.
    Never returns 5xx (otherwise K8s will restart pod).

    Returns:
        200: Worker process is alive
    """
    return jsonify({"status": "alive", "timestamp": int(time.time())}), 200


@app.route("/ready", methods=["GET"])
def ready():
    """
    Readiness probe - can the worker accept new tasks?

    Returns:
        200: Worker is ready to accept tasks
        503: Worker is alive but not ready (don't send traffic)

    Checks:
        - Service is initialized
        - Memory usage below threshold
        - No excessive consecutive errors
        - Disk space available
    """
    if _service is None:
        return jsonify({"status": "not_ready", "reason": "service_not_initialized"}), 503

    # Check memory threshold
    memory_mb = _service.stats.get("memory_mb", 0)
    if memory_mb > _service.config.system.memory_threshold_mb:
        return jsonify(
            {
                "status": "not_ready",
                "reason": "memory_threshold_exceeded",
                "memory_mb": memory_mb,
                "threshold_mb": _service.config.system.memory_threshold_mb,
            }
        ), 503

    # Check consecutive errors
    consecutive_errors = _service.stats.get("consecutive_errors", 0)
    if consecutive_errors > 3:
        return jsonify(
            {
                "status": "not_ready",
                "reason": "too_many_consecutive_errors",
                "consecutive_errors": consecutive_errors,
            }
        ), 503

    # Check disk space (models directory)
    model_path = _service.config.whisper.model_path
    if os.path.exists(model_path):
        stat = os.statvfs(model_path)
        free_mb = (stat.f_bavail * stat.f_frsize) / (1024 * 1024)
        if free_mb < 500:  # Less than 500MB free
            return jsonify(
                {
                    "status": "not_ready",
                    "reason": "insufficient_disk_space",
                    "free_mb": int(free_mb),
                    "required_mb": 500,
                }
            ), 503

    # Worker is ready
    return jsonify(
        {
            "status": "ready",
            "memory_mb": memory_mb,
            "jobs_active": _service.stats.get("jobs_active", 0),
            "model_loaded": _service.model_manager.is_loaded(),
            "uptime_seconds": int(time.time() - _service.start_time),
        }
    ), 200


@app.route("/metrics", methods=["GET"])
def metrics():
    """
    Detailed metrics endpoint for monitoring systems.

    Returns comprehensive worker statistics in JSON format.
    Useful for Prometheus, Grafana, or custom monitoring.

    Returns:
        200: Detailed metrics
        503: Service not initialized
    """
    if _service is None:
        return jsonify({"error": "service_not_initialized"}), 503

    # Get CPU and memory
    process = psutil.Process()
    cpu_percent = process.cpu_percent(interval=0.1)
    memory_mb = int(process.memory_info().rss / (1024 * 1024))

    # Get disk space
    model_path = _service.config.whisper.model_path
    disk_available_mb = 0
    if os.path.exists(model_path):
        stat = os.statvfs(model_path)
        disk_available_mb = int((stat.f_bavail * stat.f_frsize) / (1024 * 1024))

    return jsonify(
        {
            # Resource usage
            "memory_mb": memory_mb,
            "memory_threshold_mb": _service.config.system.memory_threshold_mb,
            "cpu_percent": cpu_percent,
            "disk_available_mb": disk_available_mb,
            # Model state
            "model_loaded": _service.model_manager.is_loaded(),
            "model_name": _service.config.whisper.model_name,
            # Job statistics
            "jobs_processed": _service.stats.get("jobs_processed", 0),
            "jobs_active": _service.stats.get("jobs_active", 0),
            "jobs_failed": _service.stats.get("jobs_failed", 0),
            "consecutive_errors": _service.stats.get("consecutive_errors", 0),
            "last_job_timestamp": _service.stats.get("last_job_timestamp", 0),
            # System info
            "uptime_seconds": int(time.time() - _service.start_time),
            "version": _service.config.version,
            "pid": os.getpid(),
        }
    ), 200


def run_health_server(host: str = "0.0.0.0", port: int = 8080) -> ServerThread:
    """
    Start health check HTTP server in a thread.

    Args:
        host: Host to bind to (default: 0.0.0.0 for all interfaces)
        port: Port to listen on (default: 8080)

    Returns:
        ServerThread instance that can be shut down gracefully
    """
    global _server
    logger.info(f"Starting health server on {host}:{port}")
    try:
        _server = ServerThread(host, port)
        _server.start()
        return _server
    except Exception as e:
        logger.error(f"Health server failed to start: {e}")
        raise


def shutdown_health_server():
    """Shutdown the HTTP health server gracefully"""
    global _server
    if _server is not None:
        _server.shutdown()
        _server.join(timeout=5)
        _server = None
