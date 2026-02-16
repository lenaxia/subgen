# Story 01: Multiple Output Formats

**Epic**: EPIC_08  
**Status**: Not Started  
**Effort**: 8-10 hours  
**Priority**: MEDIUM  
**Assignee**: Delegation Agent

---

## User Story

As a Subgen user,
I want to generate subtitles in multiple formats (VTT, TXT, TSV, JSON),
So that I can use subtitles with different players and applications that require specific formats.

---

## Acceptance Criteria

- [ ] **VTT format** - WebVTT format for web players (HTML5 video)
- [ ] **TXT format** - Plain text transcript without timestamps
- [ ] **TSV format** - Tab-separated values with start, end, and text columns
- [ ] **JSON format** - Structured data with language, duration, and segments
- [ ] Writer interface defines clear contract with Write method
- [ ] Factory method to create appropriate writer based on format string
- [ ] All writers handle empty segment lists gracefully
- [ ] All writers handle invalid input with proper error messages
- [ ] Unit tests for each format (happy and unhappy paths)
- [ ] Type checking passes (no type errors)
- [ ] Integration points documented
- [ ] Work log created

---

## Technical Design

### Approach

Create a `Writer` interface in the `orchestrator/pkg/formats` package that defines a common contract for all subtitle format writers. Implement concrete writers for VTT, TXT, TSV, and JSON formats.

### Writer Interface

```go
package formats

import "io"

// Segment represents a single subtitle segment with timing and text
type Segment struct {
    Start float64 // Start time in seconds
    End   float64 // End time in seconds
    Text  string  // Subtitle text
}

// Metadata contains information about the transcription
type Metadata struct {
    Language string  // ISO language code (e.g., "en")
    Duration float64 // Total duration in seconds
}

// Writer defines the interface for subtitle format writers
type Writer interface {
    // Write writes segments to the provided writer in the specific format
    Write(w io.Writer, segments []Segment, metadata Metadata) error
}
```

### Files to Create

1. **orchestrator/pkg/formats/writer.go**
   - Writer interface definition
   - Segment and Metadata structs
   - Factory function: `NewWriter(format string) (Writer, error)`

2. **orchestrator/pkg/formats/vtt_writer.go**
   - VTTWriter struct implementing Writer interface
   - WebVTT format output
   - Format: Header "WEBVTT\n\n" followed by cues

3. **orchestrator/pkg/formats/txt_writer.go**
   - TXTWriter struct implementing Writer interface
   - Plain text output (no timestamps)
   - One line per segment

4. **orchestrator/pkg/formats/tsv_writer.go**
   - TSVWriter struct implementing Writer interface
   - Tab-separated values output
   - Header row: "start\tend\ttext"

5. **orchestrator/pkg/formats/json_writer.go**
   - JSONWriter struct implementing Writer interface
   - Structured JSON output with language, duration, segments

6. **orchestrator/pkg/formats/writer_test.go**
   - Tests for factory function
   - Integration tests

7. **orchestrator/pkg/formats/vtt_writer_test.go**
   - Unit tests for VTT writer

8. **orchestrator/pkg/formats/txt_writer_test.go**
   - Unit tests for TXT writer

9. **orchestrator/pkg/formats/tsv_writer_test.go**
   - Unit tests for TSV writer

10. **orchestrator/pkg/formats/json_writer_test.go**
    - Unit tests for JSON writer

### Format Specifications

#### VTT (WebVTT)
```vtt
WEBVTT

00:00:00.000 --> 00:00:03.200
Hello, this is a test subtitle.

00:00:03.400 --> 00:00:06.800
This is the second line of text.
```

#### TXT (Plain Text)
```txt
Hello, this is a test subtitle.
This is the second line of text.
The audio continues with more dialogue.
```

#### TSV (Tab-Separated Values)
```tsv
start	end	text
0.000	3.200	Hello, this is a test subtitle.
3.400	6.800	This is the second line of text.
```

#### JSON (Structured Data)
```json
{
  "language": "en",
  "duration": 120.5,
  "segments": [
    {
      "start": 0.0,
      "end": 3.2,
      "text": "Hello, this is a test subtitle."
    },
    {
      "start": 3.4,
      "end": 6.8,
      "text": "This is the second line of text."
    }
  ]
}
```

### Integration Points

- **Future use by worker**: When the worker generates subtitles, it will use these writers to output in the requested format
- **API endpoints**: Future stories will add format query parameters to endpoints (e.g., `/asr?format=vtt`)
- **Configuration**: SUBTITLE_FORMAT environment variable will determine default format

---

## Testing Strategy

### Unit Tests

**VTT Writer:**
- Happy path: Normal segments with timestamps
- Empty segments list
- Missing timestamps (zero values)
- Special characters in text
- Long text segments
- Multiline text in segments

**TXT Writer:**
- Happy path: Normal segments
- Empty segments list
- Special characters in text
- Empty text in segments

**TSV Writer:**
- Happy path: Normal segments with tab separation
- Empty segments list
- Text containing tabs (should be escaped/replaced)
- Text containing newlines (should be escaped/replaced)
- Floating point precision for timestamps

**JSON Writer:**
- Happy path: Valid JSON structure
- Empty segments list
- Missing metadata fields
- Special characters in text (JSON escaping)
- Invalid UTF-8 sequences

**Factory Function:**
- Valid format strings: "vtt", "txt", "tsv", "json"
- Case insensitivity: "VTT", "Vtt", etc.
- Invalid format strings
- Empty format string

### Integration Tests

- Create all 4 formats from the same segment data
- Verify each output is valid for its format
- Compare output sizes and content

### Test Data

```go
var testSegments = []Segment{
    {Start: 0.0, End: 3.2, Text: "Hello, this is a test subtitle."},
    {Start: 3.4, End: 6.8, Text: "This is the second line of text."},
    {Start: 7.0, End: 10.5, Text: "The audio continues with more dialogue."},
}

var testMetadata = Metadata{
    Language: "en",
    Duration: 10.5,
}
```

---

## Definition of Done

- [ ] Writer interface created in orchestrator/pkg/formats/writer.go
- [ ] VTT writer implemented and tested
- [ ] TXT writer implemented and tested
- [ ] TSV writer implemented and tested
- [ ] JSON writer implemented and tested
- [ ] Factory function implemented and tested
- [ ] All unit tests written FIRST (TDD)
- [ ] All unit tests passing
- [ ] Test coverage > 90% for formats package
- [ ] Go type checking passes (`go build ./...`)
- [ ] Code follows Go best practices (golint/staticcheck clean)
- [ ] Integration points documented in code comments
- [ ] Work log created in docs/WORKLOGS/

---

## Commands for Validation

```bash
# Build package
cd orchestrator
go build ./pkg/formats/...

# Run all tests
go test ./pkg/formats/... -v

# Run tests with coverage
go test ./pkg/formats/... -cover -coverprofile=coverage.out
go tool cover -func=coverage.out

# Type checking
go vet ./pkg/formats/...

# Linting (if staticcheck is installed)
staticcheck ./pkg/formats/...

# Format check
go fmt ./pkg/formats/...
```

---

## References

- Epic README: docs/BACKLOG/EPIC_08/README.md
- README-LLM.md: Project documentation and workflow guide
- WebVTT Spec: https://www.w3.org/TR/webvtt1/
- Go encoding/json: Standard library for JSON output
- Go text/tabwriter: Standard library for TSV formatting

---

**Story Created:** 2026-02-16  
**Last Updated:** 2026-02-16
