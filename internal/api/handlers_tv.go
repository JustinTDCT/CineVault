package api

import (
	"net/http"

	"github.com/google/uuid"
)

// handleListLibraryShows returns TV shows belonging to a library.
func (s *Server) handleListLibraryShows(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	shows, err := s.tvRepo.ListShowsByLibrary(libraryID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list shows")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: shows})
}

// handleListShowSeasons returns seasons for a TV show.
func (s *Server) handleListShowSeasons(w http.ResponseWriter, r *http.Request) {
	showID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid show ID")
		return
	}

	seasons, err := s.tvRepo.ListSeasonsByShow(showID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list seasons")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: seasons})
}

// handleListSeasonEpisodes returns episodes for a season.
func (s *Server) handleListSeasonEpisodes(w http.ResponseWriter, r *http.Request) {
	seasonID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid season ID")
		return
	}

	episodes, err := s.tvRepo.ListEpisodesBySeason(seasonID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list episodes")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: episodes})
}

// handleGetShow returns a single TV show by ID.
func (s *Server) handleGetShow(w http.ResponseWriter, r *http.Request) {
	showID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid show ID")
		return
	}

	show, err := s.tvRepo.GetShowByID(showID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "show not found")
		return
	}

	// Get seasons
	seasons, _ := s.tvRepo.ListSeasonsByShow(showID)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"show":    show,
		"seasons": seasons,
	}})
}
