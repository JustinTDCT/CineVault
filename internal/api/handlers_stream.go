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
		height = *media.Height
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

func (s *Server) handleStreamSegment(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("mediaId")
	quality := r.PathValue("quality")
	segmentFile := r.PathValue("segment")

	// Find or start transcode session
	sessionKey := fmt.Sprintf("%s-%s", mediaID, quality)
	session := s.transcoder.GetSession(sessionKey)

	if session == nil {
		// Start new transcode
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
		sess, err := s.transcoder.StartTranscode(mediaID, userID, media.FilePath, quality)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, "transcode failed: "+err.Error())
			return
		}
		session = sess
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
		http.ServeFile(w, r, playlistPath)
		return
	}

	// Serve segment file
	segPath := filepath.Join(session.OutputDir, segmentFile)
	if _, err := os.Stat(segPath); err != nil {
		s.respondError(w, http.StatusNotFound, "segment not ready")
		return
	}

	w.Header().Set("Content-Type", "video/mp2t")
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

	if err := stream.ServeDirectFile(w, r, media.FilePath); err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
	}
}

