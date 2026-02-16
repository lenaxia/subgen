package skip

// FFProbeOutput represents the top-level FFprobe JSON response
type FFProbeOutput struct {
	Streams []FFProbeStream `json:"streams"`
}

// FFProbeStream represents a single stream in the FFprobe output
type FFProbeStream struct {
	Index     int               `json:"index"`
	CodecType string            `json:"codec_type"`
	CodecName string            `json:"codec_name"`
	Tags      FFProbeStreamTags `json:"tags,omitempty"`
}

// FFProbeStreamTags represents stream metadata tags
type FFProbeStreamTags struct {
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
}

// SubtitleTrack represents a parsed subtitle track with all relevant metadata
type SubtitleTrack struct {
	Index    int    // Stream index in the container
	Language string // ISO 639-2 language code (e.g., "eng", "jpn")
	Title    string // Subtitle title/description (e.g., "English (Full)", "Signs/Songs")
	Codec    string // Codec name (e.g., "subrip", "ass", "hdmv_pgs_subtitle")
}
