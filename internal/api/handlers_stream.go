package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/stream"
	"github.com/google/uuid"
)

// ──────────────────── Stream Handlers ────────────────────

func (s *Server) handleStreamMaster(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	// Determine available qualities based on source resolution
	var qualities []string
	height := 0
	if media.Height != nil {
		height = normalizeResolution(*media.Height)
	}
	for _, q := range []string{"360p", "480p", "720p", "1080p", "4K"} {
		if stream.Qualities[q].Height <= height || height == 0 {
			qualities = append(qualities, q)
		}
	}
	if len(qualities) == 0 {
		qualities = []string{"720p"}
	}

	playlist := s.transcoder.GenerateMasterPlaylist(mediaID.String(), media.FilePath, qualities)

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(playlist))
}

// handleStreamInfo returns media stream info (resolution, codec, available qualities) as JSON
func (s *Server) handleStreamInfo(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	height := 0
	if media.Height != nil {
		height = *media.Height
	}
	width := 0
	if media.Width != nil {
		width = *media.Width
	}

	// Normalize height to standard resolution label
	standardHeight := normalizeResolution(height)

	// Determine available transcode qualities
	var transcodeQualities []string
	for _, q := range []string{"360p", "480p", "720p", "1080p", "4K"} {
		if stream.Qualities[q].Height <= standardHeight || height == 0 {
			transcodeQualities = append(transcodeQualities, q)
		}
	}

	needsRemux := stream.NeedsRemux(media.FilePath)
	// Native formats (MP4/WebM) can direct play; MKV/AVI/etc. use HLS-based remux
	directPlayable := !needsRemux

	codec := ""
	if media.Codec != nil {
		codec = *media.Codec
	}
	container := ""
	if media.Container != nil {
		container = *media.Container
	}

	nativeRes := ""
	if standardHeight > 0 {
		nativeRes = fmt.Sprintf("%dp", standardHeight)
	}

	audioCodec := ""
	if media.AudioCodec != nil {
		audioCodec = *media.AudioCodec
	}

	duration := 0
	if media.DurationSeconds != nil {
		duration = *media.DurationSeconds
	}

	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"media_id":            mediaID.String(),
			"width":               width,
			"height":              height,
			"native_resolution":   nativeRes,
			"codec":               codec,
			"audio_codec":         audioCodec,
			"container":           container,
			"direct_playable":     directPlayable,
			"needs_remux":         needsRemux,
			"duration_seconds":    duration,
			"transcode_qualities": transcodeQualities,
		},
	})
}

func (s *Server) handleStreamSegment(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("mediaId")
	quality := r.PathValue("quality")
	segmentFile := r.PathValue("segment")

	// Find or start session
	sessionKey := fmt.Sprintf("%s-%s", mediaID, quality)
	session := s.transcoder.GetSession(sessionKey)

	if session == nil {
		// Start new session (transcode or remux)
		mid, err := uuid.Parse(mediaID)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, "invalid media id")
			return
		}
		media, err := s.mediaRepo.GetByID(mid)
		if err != nil {
			s.respondError(w, http.StatusNotFound, "media not found")
			return
		}

		userID := s.getUserID(r).String()

		if quality == "remux" {
			// HLS-based remux: video copied, audio transcoded if needed
			audioCodec := ""
			if media.AudioCodec != nil {
				audioCodec = *media.AudioCodec
			}
			sess, err := s.transcoder.StartRemux(mediaID, userID, media.FilePath, audioCodec)
			if err != nil {
				s.respondError(w, http.StatusInternalServerError, "remux failed: "+err.Error())
				return
			}
			session = sess
		} else {
			// Standard HLS transcode (re-encode to target quality)
			sess, err := s.transcoder.StartTranscode(mediaID, userID, media.FilePath, quality)
			if err != nil {
				s.respondError(w, http.StatusInternalServerError, "transcode failed: "+err.Error())
				return
			}
			session = sess
		}
	}

	// Check if requesting the m3u8 playlist
	if strings.HasSuffix(segmentFile, ".m3u8") {
		playlistPath := filepath.Join(session.OutputDir, "stream.m3u8")
		// Wait briefly for file to appear
		if _, err := os.Stat(playlistPath); err != nil {
			s.respondError(w, http.StatusAccepted, "transcoding in progress")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.ServeFile(w, r, playlistPath)
		return
	}

	// Serve segment file
	segPath := filepath.Join(session.OutputDir, segmentFile)
	if _, err := os.Stat(segPath); err != nil {
		s.respondError(w, http.StatusNotFound, "segment not ready")
		return
	}

	// Use correct MIME type based on segment format
	if strings.HasSuffix(segmentFile, ".mp4") {
		w.Header().Set("Content-Type", "video/mp4")
	} else {
		w.Header().Set("Content-Type", "video/mp2t")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeFile(w, r, segPath)
}

func (s *Server) handleStreamDirect(w http.ResponseWriter, r *http.Request) {
	mediaID, err := uuid.Parse(r.PathValue("mediaId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media id")
		return
	}

	media, err := s.mediaRepo.GetByID(mediaID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media not found")
		return
	}

	// Native browser format (MP4/WebM) - serve directly with range support
	if err := stream.ServeDirectFile(w, r, media.FilePath); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
	}
}

// normalizeResolution maps actual pixel heights to standard resolution labels
func normalizeResolution(height int) int {
	if height <= 0 {
		return 0
	}
	// Map to nearest standard resolution (with tolerance)
	standards := []int{360, 480, 720, 1080, 2160}
	for _, s := range standards {
		// Within 15% tolerance of standard
		if height >= s-s*15/100 && height <= s+s*15/100 {
			return s
		}
	}
	// If above 1080 but below 4K, call it 1080p
	if height > 1080 && height < 2160 {
		return 1080
	}
	// Return as-is for non-standard
	return height
}
