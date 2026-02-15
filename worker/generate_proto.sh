#!/bin/bash
# Generate Python gRPC code from protobuf definition
#
# Requirements:
#   pip install grpcio-tools
#
# Usage:
#   ./generate_proto.sh

set -e

# Change to project root
cd "$(dirname "$0")/.."

echo "Generating Python gRPC code from api/transcription.proto..."

python3 -m grpc_tools.protoc \
    -I./api \
    --python_out=./worker/pb \
    --grpc_python_out=./worker/pb \
    --pyi_out=./worker/pb \
    ./api/transcription.proto

echo "Generated files:"
ls -lh ./worker/pb/

echo "✅ Protobuf generation complete!"
