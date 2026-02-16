# Subtitle Format Writers

This package provides writers for multiple subtitle and transcript formats.

## Supported Formats

- **VTT** (WebVTT) - Web Video Text Tracks format for HTML5 video
- **TXT** - Plain text transcript without timestamps
- **TSV** - Tab-separated values with start/end timestamps and text
- **JSON** - Structured JSON with language, duration, and segments

## Usage

### Basic Usage

```go
import "github.com/mccloud/subgen/orchestrator/pkg/formats"

// Create segments
segments := []formats.Segment{
    {Start: 0.0, End: 3.2, Text: "Hello, this is a test subtitle."},
    {Start: 3.4, End: 6.8, Text: "This is the second line of text."},
}

// Create metadata
metadata := formats.Metadata{
    Language: "en",
    Duration: 10.5,
}

// Create writer using factory
writer, err := formats.NewWriter("vtt")
if err != nil {
    log.Fatal(err)
}

// Write to buffer or file
var buf bytes.Buffer
if err := writer.Write(&buf, segments, metadata); err != nil {
    log.Fatal(err)
}

fmt.Println(buf.String())
```

### Format-Specific Examples

#### VTT (WebVTT)

```go
writer := &formats.VTTWriter{}
writer.Write(output, segments, metadata)
```

Output:
```
WEBVTT

00:00:00.000 --> 00:00:03.200
Hello, this is a test subtitle.

00:00:03.400 --> 00:00:06.800
This is the second line of text.
```

#### TXT (Plain Text)

```go
writer := &formats.TXTWriter{}
writer.Write(output, segments, metadata)
```

Output:
```
Hello, this is a test subtitle.
This is the second line of text.
```

#### TSV (Tab-Separated Values)

```go
writer := &formats.TSVWriter{}
writer.Write(output, segments, metadata)
```

Output:
```
start	end	text
0.000	3.200	Hello, this is a test subtitle.
3.400	6.800	This is the second line of text.
```

#### JSON (Structured Data)

```go
writer := &formats.JSONWriter{}
writer.Write(output, segments, metadata)
```

Output:
```json
{
  "language": "en",
  "duration": 10.5,
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

## Interface

All writers implement the `Writer` interface:

```go
type Writer interface {
    Write(w io.Writer, segments []Segment, metadata Metadata) error
}
```

### Types

```go
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
```

## Factory Function

The `NewWriter` function creates a writer based on format string:

```go
func NewWriter(format string) (Writer, error)
```

Supported format strings (case-insensitive):
- `"vtt"` or `"VTT"` - WebVTT format
- `"txt"` or `"TXT"` - Plain text format
- `"tsv"` or `"TSV"` - Tab-separated values format
- `"json"` or `"JSON"` - JSON format

Returns error if format is not supported.

## Error Handling

All writers return errors for:
- Nil writer parameter
- I/O errors during writing

Empty segment lists are handled gracefully - formats that require headers/structure will still produce valid output.

## Integration Points

### Future Use Cases

1. **Worker Integration**: The Python worker will use these writers to output subtitles in the requested format
2. **API Endpoints**: Future API endpoints will accept format parameters (e.g., `/asr?format=vtt`)
3. **Configuration**: The `SUBTITLE_FORMAT` environment variable will determine the default output format

### Example Integration

```go
// In worker or API handler
format := os.Getenv("SUBTITLE_FORMAT") // "vtt", "txt", "tsv", "json"
writer, err := formats.NewWriter(format)
if err != nil {
    // Fall back to default format
    writer = &formats.VTTWriter{}
}

// Write subtitles
file, _ := os.Create("output.vtt")
defer file.Close()
writer.Write(file, segments, metadata)
```

## Testing

Run tests:
```bash
cd orchestrator
go test ./pkg/formats/... -v
```

Run with coverage:
```bash
go test ./pkg/formats/... -cover
```

View coverage report:
```bash
go test ./pkg/formats/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Implementation Notes

### VTT Writer
- Follows WebVTT specification
- Timestamp format: `HH:MM:SS.mmm`
- Preserves multiline text and HTML-like tags
- Empty segments produce valid VTT with header only

### TXT Writer
- No timestamps included
- One line per segment
- Preserves multiline text within segments
- Empty segments produce empty lines

### TSV Writer
- Header row: `start\tend\ttext`
- Timestamps with 3 decimal places
- Tabs and newlines in text replaced with spaces
- Valid TSV structure maintained

### JSON Writer
- Pretty-printed with 2-space indentation
- All text preserved with proper JSON escaping
- Floating-point precision maintained
- Valid JSON even with empty segments

## References

- [WebVTT Specification](https://www.w3.org/TR/webvtt1/)
- [EPIC_08 README](../../../docs/BACKLOG/EPIC_08/README.md)
- [Story 01: Multiple Output Formats](../../../docs/BACKLOG/EPIC_08/stories/STORY_01_output_formats.md)
