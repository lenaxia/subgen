#!/bin/bash
# Generate Python gRPC code from protobuf definition
#
# Requirements:
#   pip install grpcio-tools==1.78.0 protobuf>=6.31.1,<7.0.0  # Must match requirements.txt
#
# Usage:
#   ./generate_proto.sh

set -e

# Change to project root
cd "$(dirname "$0")/.."

echo "Generating Python gRPC code from api/transcription.proto..."

# Clean previous generated files
rm -rf ./worker/pb/subgen

# Generate with proper package structure based on proto package "subgen.v1"
# This creates subgen/v1/ directory structure
python3 -m grpc_tools.protoc \
    -I./api \
    --python_out=./worker/pb \
    --grpc_python_out=./worker/pb \
    --pyi_out=./worker/pb \
    ./api/transcription.proto

echo "Generated files:"
find ./worker/pb -name "*.py" -o -name "*.pyi" | xargs ls -lh

echo "✅ Protobuf generation complete!"
