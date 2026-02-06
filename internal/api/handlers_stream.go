package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

	// All formats playable: native formats direct play, MKV/AVI remuxed on-the-fly
	needsRemux := stream.NeedsRemux(media.FilePath)
	directPlayable := true

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
		// Start new session
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
			// HLS-based remux (kept for future use / fallback)
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
			// Standard HLS transcode
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
		// Poll briefly for the playlist to appear
		for i := 0; i < 20; i++ {
			if _, err := os.Stat(playlistPath); err == nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
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

	// Parse optional seek position (in seconds)
	startSeconds := 0.0
	if startParam := r.URL.Query().Get("start"); startParam != "" {
		if parsed, err := strconv.ParseFloat(startParam, 64); err == nil && parsed > 0 {
			startSeconds = parsed
		}
	}

	// If the file needs remuxing (MKV, AVI, etc.) remux on-the-fly to MP4
	if stream.NeedsRemux(media.FilePath) {
		audioCodec := ""
		if media.AudioCodec != nil {
			audioCodec = *media.AudioCodec
		}
		if err := stream.ServeRemuxed(w, r, s.config.FFmpeg.FFmpegPath, media.FilePath, audioCodec, startSeconds); err != nil {
			s.respondError(w, http.StatusInternalServerError, "remux failed: "+err.Error())
		}
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
	standards := []int{360, 480, 720, 1080, 2160}
	for _, s := range standards {
		if height >= s-s*15/100 && height <= s+s*15/100 {
			return s
		}
	}
	if height > 1080 && height < 2160 {
		return 1080
	}
	return height
}
