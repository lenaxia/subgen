#!/usr/bin/env python3
"""Test DetectLanguage RPC method."""

import sys
import grpc

# Add worker pb to path
sys.path.insert(0, "/home/mikekao/personal/subgen/worker")

from worker.pb import transcription_pb2, transcription_pb2_grpc


def test_detect_language():
    """Test language detection with a file path."""

    # Connect to worker
    channel = grpc.insecure_channel("localhost:50051")
    stub = transcription_pb2_grpc.TranscriptionServiceStub(channel)

    # Create request
    request = transcription_pb2.DetectLanguageRequest(
        file_path="/output/speech_sample.wav", sample_length=10, sample_offset=0
    )

    print("Sending DetectLanguage request...")
    print(f"  File: /output/speech_sample.wav")
    print(f"  Sample length: 10 seconds")

    try:
        response = stub.DetectLanguage(request, timeout=30)

        if response.success:
            print("\n✅ Language Detection Success!")
            print(f"   Language Code: {response.language_code}")
            print(f"   Language Name: {response.language_name}")
            print(f"   Confidence: {response.confidence:.2f}")
        else:
            print(f"\n❌ Language Detection Failed: {response.error_message}")

    except grpc.RpcError as e:
        print(f"\n❌ gRPC Error: {e.code()} - {e.details()}")
    except Exception as e:
        print(f"\n❌ Error: {e}")
    finally:
        channel.close()


if __name__ == "__main__":
    test_detect_language()
