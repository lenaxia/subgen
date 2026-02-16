package formats_test

import (
	"bytes"
	"fmt"
	"log"

	"github.com/mccloud/subgen/orchestrator/pkg/formats"
)

// ExampleNewWriter demonstrates how to create and use different format writers
func ExampleNewWriter() {
	// Sample subtitle segments
	segments := []formats.Segment{
		{Start: 0.0, End: 3.2, Text: "Hello, this is a test subtitle."},
		{Start: 3.4, End: 6.8, Text: "This is the second line of text."},
		{Start: 7.0, End: 10.5, Text: "The audio continues with more dialogue."},
	}

	metadata := formats.Metadata{
		Language: "en",
		Duration: 10.5,
	}

	// Create a VTT writer
	vttWriter, err := formats.NewWriter("vtt")
	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	if err := vttWriter.Write(&buf, segments, metadata); err != nil {
		log.Fatal(err)
	}

	fmt.Println("VTT Output:")
	fmt.Println(buf.String())
}

// ExampleVTTWriter demonstrates VTT format output
func ExampleVTTWriter() {
	writer := &formats.VTTWriter{}

	segments := []formats.Segment{
		{Start: 0.0, End: 2.5, Text: "First subtitle line"},
		{Start: 3.0, End: 5.5, Text: "Second subtitle line"},
	}

	metadata := formats.Metadata{
		Language: "en",
		Duration: 6.0,
	}

	var buf bytes.Buffer
	if err := writer.Write(&buf, segments, metadata); err != nil {
		log.Fatal(err)
	}

	fmt.Println(buf.String())
	// Output:
	// WEBVTT
	//
	// 00:00:00.000 --> 00:00:02.500
	// First subtitle line
	//
	// 00:00:03.000 --> 00:00:05.500
	// Second subtitle line
}

// ExampleTXTWriter demonstrates plain text format output
func ExampleTXTWriter() {
	writer := &formats.TXTWriter{}

	segments := []formats.Segment{
		{Start: 0.0, End: 2.5, Text: "First subtitle line"},
		{Start: 3.0, End: 5.5, Text: "Second subtitle line"},
	}

	metadata := formats.Metadata{
		Language: "en",
		Duration: 6.0,
	}

	var buf bytes.Buffer
	if err := writer.Write(&buf, segments, metadata); err != nil {
		log.Fatal(err)
	}

	fmt.Print(buf.String())
	// Output:
	// First subtitle line
	// Second subtitle line
}

// ExampleTSVWriter demonstrates TSV format output
func ExampleTSVWriter() {
	writer := &formats.TSVWriter{}

	segments := []formats.Segment{
		{Start: 0.0, End: 2.5, Text: "First subtitle line"},
		{Start: 3.0, End: 5.5, Text: "Second subtitle line"},
	}

	metadata := formats.Metadata{
		Language: "en",
		Duration: 6.0,
	}

	var buf bytes.Buffer
	if err := writer.Write(&buf, segments, metadata); err != nil {
		log.Fatal(err)
	}

	fmt.Print(buf.String())
	// Output:
	// start	end	text
	// 0.000	2.500	First subtitle line
	// 3.000	5.500	Second subtitle line
}

// ExampleJSONWriter demonstrates JSON format output
func ExampleJSONWriter() {
	writer := &formats.JSONWriter{}

	segments := []formats.Segment{
		{Start: 0.0, End: 2.5, Text: "First subtitle line"},
	}

	metadata := formats.Metadata{
		Language: "en",
		Duration: 3.0,
	}

	var buf bytes.Buffer
	if err := writer.Write(&buf, segments, metadata); err != nil {
		log.Fatal(err)
	}

	fmt.Print(buf.String())
	// Output:
	// {
	//   "language": "en",
	//   "duration": 3,
	//   "segments": [
	//     {
	//       "start": 0,
	//       "end": 2.5,
	//       "text": "First subtitle line"
	//     }
	//   ]
	// }
}
