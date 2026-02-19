#!/bin/bash
# Generate Go gRPC code from protobuf definition

set -e

echo "Generating Go gRPC code from api/transcription.proto..."

# Generate to orchestrator/pkg/pb
protoc \
    -I./api \
    --go_out=./orchestrator/pkg/pb \
    --go_opt=paths=source_relative \
    --go-grpc_out=./orchestrator/pkg/pb \
    --go-grpc_opt=paths=source_relative \
    ./api/transcription.proto

# Also generate to orchestrator/pkg/pb/api for backward compatibility
protoc \
    -I./api \
    --go_out=./orchestrator/pkg/pb/api \
    --go_opt=paths=source_relative \
    --go-grpc_out=./orchestrator/pkg/pb/api \
    --go-grpc_opt=paths=source_relative \
    ./api/transcription.proto

echo "Generated files:"
ls -lh ./orchestrator/pkg/pb/
ls -lh ./orchestrator/pkg/pb/api/

echo "✅ Go protobuf generation complete!"