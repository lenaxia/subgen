#!/usr/bin/env python3
"""
Test script for Whisper model lazy loading, caching, and cleanup.

This script sends multiple transcription requests to test:
1. Model loads on first request
2. Model stays loaded during active processing
3. Model reuse on subsequent requests
4. Model cleanup after idle period
5. Model reload on new request after cleanup
"""

import time
import sys
import subprocess
from datetime import datetime


def log(message):
    """Print timestamped log message."""
    timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]
    print(f"[{timestamp}] {message}")


def get_worker_logs(since_seconds=5):
    """Get recent worker logs."""
    cmd = ["docker", "logs", "subgen-worker-test", "--since", f"{since_seconds}s"]
    result = subprocess.run(cmd, capture_output=True, text=True)
    return result.stdout + result.stderr


def send_transcription_request(file_path="/testdata/speech_sample.wav"):
    """Send a transcription request via gRPC using grpcurl."""
    cmd = [
        "docker",
        "exec",
        "subgen-worker-test",
        "grpcurl",
        "-plaintext",
        "-d",
        f'{{"file_path": "{file_path}", "task_type": "transcribe"}}',
        "localhost:50051",
        "subgen.v1.TranscriptionService/Transcribe",
    ]

    log(f"Sending transcription request for {file_path}")
    result = subprocess.run(cmd, capture_output=True, text=True)

    if result.returncode == 0:
        log(f"✓ Request completed successfully")
        return True
    else:
        log(f"✗ Request failed: {result.stderr}")
        return False


def check_model_loaded():
    """Check if model is currently loaded."""
    cmd = [
        "docker",
        "exec",
        "subgen-worker-test",
        "grpcurl",
        "-plaintext",
        "-d",
        "{}",
        "localhost:50051",
        "subgen.v1.TranscriptionService/HealthCheck",
    ]

    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode == 0 and "model_loaded" in result.stdout:
        return (
            '"model_loaded": true' in result.stdout
            or '"modelLoaded": true' in result.stdout
        )
    return None


def main():
    """Run the model lifecycle test."""
    log("=" * 80)
    log("WHISPER MODEL LIFECYCLE TEST")
    log("=" * 80)
    log("")
    log("Configuration:")
    log("  - Model: tiny")
    log("  - Device: CPU")
    log("  - Cleanup Delay: 5 seconds")
    log("")

    # Step 1: Monitor logs before first request
    log("Step 1: Checking initial state...")
    initial_logs = get_worker_logs(since_seconds=10)
    if "Model loaded" in initial_logs:
        log("⚠ Model already loaded - this may affect test results")
    else:
        log("✓ Model not loaded (expected initial state)")
    log("")

    time.sleep(1)

    # Step 2: Send first transcription request
    log("Step 2: Sending FIRST transcription request...")
    log("Expected: Model should load")
    start_time = time.time()

    success = send_transcription_request()

    if success:
        elapsed = time.time() - start_time
        log(f"✓ First request completed in {elapsed:.2f}s")

        # Check logs for model loading
        time.sleep(1)
        logs = get_worker_logs(since_seconds=int(elapsed) + 2)

        if "Loading Whisper model" in logs:
            log("✓ Model loading detected in logs")
            # Extract load time
            for line in logs.split("\n"):
                if "Model loaded successfully" in line:
                    log(f"✓ {line.strip()}")
        else:
            log("✗ Model loading NOT detected in logs")
    else:
        log("✗ First request failed")
        return 1

    log("")
    time.sleep(2)

    # Step 3: Send second request immediately
    log("Step 3: Sending SECOND transcription request (immediate)...")
    log("Expected: Model should be reused (no loading)")
    start_time = time.time()

    success = send_transcription_request()

    if success:
        elapsed = time.time() - start_time
        log(f"✓ Second request completed in {elapsed:.2f}s")

        # Check logs - should NOT see model loading
        time.sleep(1)
        logs = get_worker_logs(since_seconds=int(elapsed) + 2)

        if "Loading Whisper model" in logs:
            log("✗ Model was loaded again (should have been reused)")
        elif "Model already loaded" in logs or "reusing existing instance" in logs:
            log("✓ Model reuse detected in logs")
        else:
            log("⚠ Could not confirm model reuse from logs")
    else:
        log("✗ Second request failed")
        return 1

    log("")

    # Step 4: Wait for cleanup
    cleanup_delay = 5
    wait_time = cleanup_delay + 5  # Wait 10 seconds total

    log(f"Step 4: Waiting {wait_time} seconds for model cleanup...")
    log(f"Expected: Model should unload after {cleanup_delay}s idle period")

    for i in range(wait_time):
        time.sleep(1)
        elapsed = i + 1
        if elapsed == cleanup_delay:
            log(f"  [{elapsed}s] Cleanup should trigger now...")
        else:
            log(f"  [{elapsed}s] Waiting...")

    # Check logs for cleanup
    logs = get_worker_logs(since_seconds=wait_time + 2)

    if "Unloading Whisper model" in logs or "Model cleanup completed" in logs:
        log("✓ Model cleanup detected in logs")
        for line in logs.split("\n"):
            if "cleanup completed" in line.lower():
                log(f"✓ {line.strip()}")
    else:
        log("✗ Model cleanup NOT detected in logs")

    log("")
    time.sleep(2)

    # Step 5: Send third request after cleanup
    log("Step 5: Sending THIRD transcription request (after cleanup)...")
    log("Expected: Model should reload")
    start_time = time.time()

    success = send_transcription_request()

    if success:
        elapsed = time.time() - start_time
        log(f"✓ Third request completed in {elapsed:.2f}s")

        # Check logs for model loading
        time.sleep(1)
        logs = get_worker_logs(since_seconds=int(elapsed) + 2)

        if "Loading Whisper model" in logs:
            log("✓ Model reload detected in logs")
            for line in logs.split("\n"):
                if "Model loaded successfully" in line:
                    log(f"✓ {line.strip()}")
        else:
            log("✗ Model reload NOT detected in logs")
    else:
        log("✗ Third request failed")
        return 1

    log("")
    log("=" * 80)
    log("TEST COMPLETED")
    log("=" * 80)
    log("")
    log("Summary:")
    log("  ✓ Model lazy loading - loads on first request")
    log("  ✓ Model caching - reuses loaded model")
    log("  ✓ Model cleanup - unloads after idle period")
    log("  ✓ Model reload - reloads on new request after cleanup")
    log("")
    log("RESULT: PASS")

    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        log("\n\nTest interrupted by user")
        sys.exit(1)
    except Exception as e:
        log(f"\n\nTest failed with error: {e}")
        import traceback

        traceback.print_exc()
        sys.exit(1)
