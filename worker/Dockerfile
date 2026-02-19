# Use Debian Bullseye base for FFmpeg 6.x compatibility with PyAV 11.x
# Note: python:3.11-slim is based on Debian Trixie which has FFmpeg 7.x
# We need Bullseye (FFmpeg 6.x) for compatibility with faster-whisper's PyAV dependency
FROM python:3.11-slim-bullseye

# Install all dependencies (build + runtime) in one stage
RUN apt-get update && apt-get install -y \
    ffmpeg \
    curl \
    gcc \
    g++ \
    pkg-config \
    libavformat-dev \
    libavcodec-dev \
    libavdevice-dev \
    libavutil-dev \
    libavfilter-dev \
    libswscale-dev \
    libswresample-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy and install Python dependencies
COPY worker/requirements.txt .
RUN pip install --no-cache-dir --timeout=1000 -r requirements.txt

# Copy application code, protobuf files, and shared modules
COPY worker/src/ ./src/
COPY worker/pb/ ./pb/
COPY language_code.py ./

# Set Python path - include pb directory for protobuf imports
ENV PYTHONPATH=/app/src:/app:/app/pb

# Health check using HTTP endpoint
HEALTHCHECK --interval=30s --timeout=10s --retries=3 --start-period=60s \
    CMD curl -f http://localhost:8080/health || exit 1

# Expose gRPC port and HTTP health server port
EXPOSE 50051
EXPOSE 8080

# TODO: Run as non-root user after fixing permissions
# RUN useradd -m -u 1000 worker
# USER worker

# Start the worker
CMD ["python", "-m", "src.main"]
