#!/bin/bash
# Script to create a test video with multiple audio tracks using FFmpeg in Docker

# Create a simple test video with multiple audio tracks
# 1. Generate a 10-second video with silent audio
# 2. Add 3 audio tracks: English, Spanish, Japanese

docker run --rm -v $(pwd):/work -w /work linuxserver/ffmpeg:latest \
  -f lavfi -i testsrc=duration=10:size=640x480:rate=1 \
  -f lavfi -i sine=frequency=440:duration=10 \
  -f lavfi -i sine=frequency=660:duration=10 \
  -f lavfi -i sine=frequency=880:duration=10 \
  -map 0:v -map 1:a -map 2:a -map 3:a \
  -metadata:s:a:0 language=eng -metadata:s:a:0 title="English" \
  -metadata:s:a:1 language=spa -metadata:s:a:1 title="Spanish" \
  -metadata:s:a:2 language=jpn -metadata:s:a:2 title="Japanese" \
  -c:v libx264 -c:a aac \
  -t 10 \
  multi_audio_test.mkv

echo "Created multi_audio_test.mkv with 3 audio tracks"
