#!/usr/bin/env python3
"""
Simple gRPC test client to send transcription requests.
This script is meant to be run inside the worker container.
"""

import sys
import grpc

# Add pb to path
sys.path.insert(0, "/app")

from pb import transcription_pb2
from pb import transcription_pb2_grpc


def send_transcription_request(file_path="/testdata/speech_sample.wav"):
    """Send a transcription request."""
    try:
        # Create channel
        channel = grpc.insecure_channel("localhost:50051")
        stub = transcription_pb2_grpc.TranscriptionServiceStub(channel)

        # Create request
        request = transcription_pb2.TranscribeRequest(
            file_path=file_path,
            task_type="transcribe",
            force_language="",
            options=transcription_pb2.TranscribeOptions(
                whisper_model="tiny",
                whisper_threads=4,
                lrc_for_audio=False,
            ),
        )

        # Send request
        response = stub.Transcribe(request, timeout=60)

        # Print result
        print(f"SUCCESS: {response.success}")
        print(f"LANGUAGE: {response.detected_language}")
        if response.subtitle_path:
            print(f"OUTPUT: {response.subtitle_path}")
        if response.error:
            print(f"ERROR: {response.error}")

        return response.success

    except Exception as e:
        print(f"EXCEPTION: {e}")
        return False
    finally:
        channel.close()


def check_health():
    """Check worker health."""
    try:
        channel = grpc.insecure_channel("localhost:50051")
        stub = transcription_pb2_grpc.TranscriptionServiceStub(channel)

        request = transcription_pb2.HealthCheckRequest()
        response = stub.HealthCheck(request, timeout=5)

        print(f"HEALTH: {response.status}")
        print(f"MODEL_LOADED: {response.model_loaded}")
        print(f"MEMORY_MB: {response.memory_mb}")

        return True

    except Exception as e:
        print(f"EXCEPTION: {e}")
        return False
    finally:
        channel.close()


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "health":
        check_health()
    else:
        file_path = sys.argv[1] if len(sys.argv) > 1 else "/testdata/speech_sample.wav"
        send_transcription_request(file_path)
