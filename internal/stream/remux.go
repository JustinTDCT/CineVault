package stream

import (
	"strings"
)

// NeedsRemux returns true if the container isn't browser-native (MP4/WebM)
func NeedsRemux(filePath string) bool {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".m4v") {
		return false
	}
	return true
}

// NeedsAudioTranscode checks if the audio codec needs transcoding to AAC
// Browser-compatible codecs (AAC, MP3, Opus, Vorbis, FLAC) can be copied;
// everything else (DTS, AC3, EAC3, TrueHD, etc.) must be transcoded.
func NeedsAudioTranscode(audioCodec string) bool {
	lower := strings.ToLower(audioCodec)
	switch lower {
	case "aac", "mp3", "opus", "vorbis", "flac":
		return false
	default:
		return true
	}
}
