# Whisper Model Lifecycle Test Plan

## Test Steps
1. [ ] Monitor worker logs before first request
2. [ ] Send first transcription request and watch for model loading
3. [ ] Send immediate second request - verify model reuse
4. [ ] Wait 10 seconds after queue empty - watch for cleanup
5. [ ] Send third request - verify model reload
6. [ ] Verify VRAM/memory cleanup events in logs
7. [ ] Check that tiny model is being used
8. [ ] Compile results and evidence

## Expected Behavior
- Model loads on first request
- Model stays loaded during active processing
- Model unloads after MODEL_CLEANUP_DELAY (5s) idle period
- Memory is released during cleanup
- Subsequent requests reload the model

## Test Environment
- MODEL_CLEANUP_DELAY=5 seconds
- Worker running with proper configuration
