package api

import (
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListLibraryArtists(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	artists, err := s.musicRepo.ListArtistsByLibrary(libraryID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list artists")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: artists})
}

func (s *Server) handleListLibraryAlbums(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	albums, err := s.musicRepo.ListAlbumsByLibrary(libraryID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list albums")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: albums})
}

func (s *Server) handleListArtistAlbums(w http.ResponseWriter, r *http.Request) {
	artistID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid artist ID")
		return
	}

	albums, err := s.musicRepo.ListAlbumsByArtist(artistID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list albums")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: albums})
}

func (s *Server) handleListAlbumTracks(w http.ResponseWriter, r *http.Request) {
	albumID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid album ID")
		return
	}

	tracks, err := s.musicRepo.ListTracksByAlbum(albumID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list tracks")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: tracks})
}

func (s *Server) handleGetArtist(w http.ResponseWriter, r *http.Request) {
	artistID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid artist ID")
		return
	}

	artist, err := s.musicRepo.GetArtistByID(artistID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "artist not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: artist})
}

func (s *Server) handleListMusicGenres(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	genres, err := s.tagRepo.ListGenresByLibrary(libraryID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list genres")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: genres})
}

func (s *Server) handleGetAlbum(w http.ResponseWriter, r *http.Request) {
	albumID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid album ID")
		return
	}

	album, err := s.musicRepo.GetAlbumByID(albumID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "album not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: album})
}

func (s *Server) handleMusicSearch(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		s.respondError(w, http.StatusBadRequest, "missing search query (q parameter)")
		return
	}

	limit := 20
	artists, _ := s.musicRepo.SearchArtists(libraryID, query, limit)
	albums, _ := s.musicRepo.SearchAlbums(libraryID, query, limit)
	tracks, _ := s.musicRepo.SearchTracks(libraryID, query, limit)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"artists": artists,
		"albums":  albums,
		"tracks":  tracks,
	}})
}

func (s *Server) handleSmartPlaylist(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	playlistType := r.URL.Query().Get("type")
	limit := 50
	var tracks []*models.MediaItem

	switch playlistType {
	case "recently-added":
		tracks, err = s.musicRepo.ListRecentlyAddedTracks(libraryID, limit)
	case "most-played":
		tracks, err = s.musicRepo.ListMostPlayedTracks(libraryID, limit)
	case "recently-played":
		tracks, err = s.musicRepo.ListRecentlyPlayedTracks(libraryID, limit)
	default:
		s.respondError(w, http.StatusBadRequest, "invalid playlist type (recently-added, most-played, recently-played)")
		return
	}
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to fetch playlist")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: tracks})
}
