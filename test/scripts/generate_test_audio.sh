#!/bin/bash
# Generate synthetic test audio files using ffmpeg
# Usage: ./generate_test_audio.sh

set -e

TESTDATA_DIR="../testdata"
mkdir -p "$TESTDATA_DIR"

echo "Generating test audio files..."

# 1. Short audio (30 seconds, 440Hz sine wave)
echo "  - short_audio.mp3 (30s)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" \
  -ac 1 -ar 16000 -ab 64k \
  "$TESTDATA_DIR/short_audio.mp3" -y 2>/dev/null

# 2. Long audio (5 minutes, 440Hz + 880Hz mix)
echo "  - long_audio.mp3 (5min)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=300" \
  -f lavfi -i "sine=frequency=880:duration=300" \
  -filter_complex "[0:a][1:a]amix=inputs=2:duration=first" \
  -ac 1 -ar 16000 -ab 64k \
  "$TESTDATA_DIR/long_audio.mp3" -y 2>/dev/null

# 3. Spanish audio (silence, for testing - Whisper will detect as 'unknown')
echo "  - spanish_audio.mp3 (30s silence)"
ffmpeg -f lavfi -i "anullsrc=r=16000:cl=mono:d=30" \
  -ab 64k \
  "$TESTDATA_DIR/spanish_audio.mp3" -y 2>/dev/null

# 4. Video with audio (1 minute, color bars + tone)
echo "  - video.mkv (1min)"
ffmpeg -f lavfi -i "testsrc=duration=60:size=640x480:rate=30" \
  -f lavfi -i "sine=frequency=440:duration=60" \
  -c:v libx264 -c:a aac -ab 128k \
  "$TESTDATA_DIR/video.mkv" -y 2>/dev/null

# 5. Corrupt audio (invalid data)
echo "  - corrupt_audio.mp3 (invalid)"
echo "This is not valid audio data" > "$TESTDATA_DIR/corrupt_audio.mp3"

# 6. Audio-only file for LRC testing
echo "  - audio_only.m4a (30s)"
ffmpeg -f lavfi -i "sine=frequency=440:duration=30" \
  -ac 1 -ar 16000 -c:a aac -ab 64k \
  "$TESTDATA_DIR/audio_only.m4a" -y 2>/dev/null

echo ""
echo "Test audio files generated successfully!"
ls -lh "$TESTDATA_DIR"
