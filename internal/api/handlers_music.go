package api

import (
	"net/http"

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
