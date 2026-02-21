#!/bin/bash
# Pre-commit hook to check for consistent dependency versions across all packages
# This ensures protobuf, gRPC, and other critical dependencies use the same versions

set -e

echo "🔍 Checking dependency version consistency..."

# Define critical packages to check
CRITICAL_PACKAGES=(
    "grpcio"
    "grpcio-tools"
    "protobuf"
    "google.golang.org/protobuf"
    "google.golang.org/grpc"
)

# Packages that are related but not required to have same version
RELATED_PACKAGES=(
    "grpcio-testing"
)

# Find all requirement files
REQUIREMENT_FILES=(
    "worker/requirements.txt"
    "worker/requirements-dev.txt"
)

# Find all go.mod files
GOMOD_FILES=(
    "orchestrator/go.mod"
)

# Function to extract version from requirement line
extract_python_version() {
    local line="$1"
    local pkg="$2"
    
    # Remove comments and trim
    line=$(echo "$line" | sed 's/#.*//' | xargs)
    
                if [[ "$line" == "$pkg=="* || "$line" == "$pkg>="* || "$line" == "$pkg~="* || "$line" == "$pkg<="* || "$line" == "$pkg>"* || "$line" == "$pkg<"* ]]; then
                    # Extract version after package name and operator
                    local version=$(echo "$line" | sed "s/^$pkg[=<>~]*//" | xargs)
                    echo "$version"
                elif [[ "$line" == "$pkg" ]]; then
                    # No version specified
                    echo "any"
                fi
}

# Function to extract version from go.mod
extract_go_version() {
    local file="$1"
    local pkg="$2"
    
    # Look for the package in go.mod
    local line=$(grep -E "^[[:space:]]*$pkg[[:space:]]+" "$file" || true)
    if [ -n "$line" ]; then
        # Extract version (everything after package name)
        local version=$(echo "$line" | awk '{print $2}')
        echo "$version"
    fi
}

# Check Python requirements
echo "📦 Checking Python requirements..."
for pkg in "${CRITICAL_PACKAGES[@]}"; do
    # Skip Go packages for Python check
    if [[ "$pkg" == google.golang.org/* ]]; then
        continue
    fi
    
    versions=()
    for req_file in "${REQUIREMENT_FILES[@]}"; do
        if [ -f "$req_file" ]; then
            # Look for package in requirement file
            while IFS= read -r line; do
                version=$(extract_python_version "$line" "$pkg")
                if [ -n "$version" ]; then
                    versions+=("$req_file: $pkg$version")
                fi
            done < "$req_file"
        fi
    done
    
    # Check if we found multiple versions
    if [ ${#versions[@]} -gt 1 ]; then
        unique_versions=$(printf "%s\n" "${versions[@]}" | awk -F': ' '{print $2}' | sort -u | wc -l)
        if [ "$unique_versions" -gt 1 ]; then
            echo "❌ Inconsistent versions found for $pkg:"
            printf "  %s\n" "${versions[@]}"
            exit 1
        fi
    fi
done

# Check Go modules
echo "⚙️  Checking Go modules..."
for pkg in "${CRITICAL_PACKAGES[@]}"; do
    # Skip Python packages for Go check
    if [[ "$pkg" != google.golang.org/* ]]; then
        continue
    fi
    
    versions=()
    for gomod in "${GOMOD_FILES[@]}"; do
        if [ -f "$gomod" ]; then
            version=$(extract_go_version "$gomod" "$pkg")
            if [ -n "$version" ]; then
                versions+=("$gomod: $pkg $version")
            fi
        fi
    done
    
    # Check if we found multiple versions
    if [ ${#versions[@]} -gt 1 ]; then
        unique_versions=$(printf "%s\n" "${versions[@]}" | awk -F': ' '{print $2}' | sort -u | wc -l)
        if [ "$unique_versions" -gt 1 ]; then
            echo "❌ Inconsistent versions found for $pkg:"
            printf "  %s\n" "${versions[@]}"
            exit 1
        fi
    fi
done

# Check protobuf compiler version consistency
echo "🔧 Checking protobuf compiler consistency..."
if command -v protoc &> /dev/null; then
    PROTOC_VERSION=$(protoc --version | awk '{print $2}')
    echo "  protoc version: $PROTOC_VERSION"
    
    # Check if this matches expected version
    # Note: This is informational only, as protoc version doesn't need to match library versions exactly
else
    echo "⚠️  protoc not found in PATH"
fi

# Check gRPC version alignment between Python and Go
echo "📊 Checking gRPC version alignment..."
PYTHON_GRPC_VERSION=""
GO_GRPC_VERSION=""

# Get Python gRPC version
if [ -f "worker/requirements.txt" ]; then
    PYTHON_GRPC_VERSION=$(grep "^grpcio==" worker/requirements.txt | sed 's/grpcio==//' || true)
    if [ -z "$PYTHON_GRPC_VERSION" ]; then
        PYTHON_GRPC_VERSION=$(grep "^grpcio>=" worker/requirements.txt | sed 's/grpcio>=//' || true)
    fi
fi

# Get Go gRPC version
if [ -f "orchestrator/go.mod" ]; then
    GO_GRPC_VERSION=$(grep "google.golang.org/grpc" orchestrator/go.mod | awk '{print $2}')
fi

if [ -n "$PYTHON_GRPC_VERSION" ] && [ -n "$GO_GRPC_VERSION" ]; then
    # Extract major.minor versions for comparison
    PY_MAJOR_MINOR=$(echo "$PYTHON_GRPC_VERSION" | cut -d. -f1-2)
    GO_MAJOR_MINOR=$(echo "$GO_GRPC_VERSION" | cut -d. -f1-2)
    
    if [ "$PY_MAJOR_MINOR" != "$GO_MAJOR_MINOR" ]; then
        echo "⚠️  gRPC major.minor version mismatch:"
        echo "  Python: $PYTHON_GRPC_VERSION"
        echo "  Go: $GO_GRPC_VERSION"
        echo "  Consider aligning to same major.minor version"
    else
        echo "✅ gRPC versions aligned (major.minor): $PY_MAJOR_MINOR"
    fi
fi

echo "✅ All version checks passed!"
echo ""
echo "📋 Summary:"
echo "  Python gRPC: ${PYTHON_GRPC_VERSION:-Not found}"
echo "  Go gRPC: ${GO_GRPC_VERSION:-Not found}"
echo "  protoc: ${PROTOC_VERSION:-Not found}"