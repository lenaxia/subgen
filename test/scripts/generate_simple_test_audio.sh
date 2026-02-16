#!/bin/bash
# Generate simple test audio files for integration tests
set -e

TESTDATA_DIR="../testdata"
mkdir -p "$TESTDATA_DIR"

echo "Generating simple test audio files..."

# Create short WAV file (30 seconds, 440Hz sine)
echo "  - short_audio.mp3"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" \
  -ac 1 -ar 16000 -ab 64k \
  "$TESTDATA_DIR/short_audio.mp3" -y 2>&1 | grep -E "(Duration|Output)" || true

# Create corrupt audio
echo "  - corrupt_audio.mp3"
echo "Not a valid MP3 file" > "$TESTDATA_DIR/corrupt_audio.mp3"

# Create video file (short)
echo "  - video.mkv"
ffmpeg -f lavfi -i "testsrc=duration=10:size=320x240:rate=10" \
  -f lavfi -i "sine=frequency=440:duration=10" \
  -c:v libx264 -preset ultrafast -c:a aac \
  "$TESTDATA_DIR/video.mkv" -y 2>&1 | grep -E "(Duration|Output)" || true

echo ""
echo "Done! Generated files:"
ls -lh "$TESTDATA_DIR" | grep -E "\.mp3|\.mkv"
