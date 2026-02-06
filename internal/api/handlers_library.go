package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/jobs"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListLibraries(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	role := models.UserRole(r.Header.Get("X-User-Role"))

	libraries, err := s.libRepo.ListForUser(userID, role)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list libraries")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: libraries})
}

type createLibraryRequest struct {
	Name           string        `json:"name"`
	MediaType      string        `json:"media_type"`
	Path           string        `json:"path"`
	IsEnabled      bool          `json:"is_enabled"`
	SeasonGrouping bool          `json:"season_grouping"`
	AccessLevel    string        `json:"access_level"`
	AllowedUsers   []string      `json:"allowed_users"`
}

func (s *Server) handleCreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req createLibraryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	accessLevel := models.LibraryAccess(req.AccessLevel)
	if accessLevel == "" {
		accessLevel = models.LibraryAccessEveryone
	}

	library := models.Library{
		ID:             uuid.New(),
		Name:           req.Name,
		MediaType:      models.MediaType(req.MediaType),
		Path:           req.Path,
		IsEnabled:      req.IsEnabled,
		SeasonGrouping: req.SeasonGrouping,
		AccessLevel:    accessLevel,
	}

	if err := s.libRepo.Create(&library); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create library")
		return
	}

	// Set user permissions if access_level is select_users
	if accessLevel == models.LibraryAccessSelectUsers && len(req.AllowedUsers) > 0 {
		var userIDs []uuid.UUID
		for _, uid := range req.AllowedUsers {
			if id, err := uuid.Parse(uid); err == nil {
				userIDs = append(userIDs, id)
			}
		}
		if err := s.libRepo.SetPermissions(library.ID, userIDs); err != nil {
			log.Printf("Failed to set library permissions: %v", err)
		}
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: library})
}

func (s *Server) handleGetLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	library, err := s.libRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "library not found")
		return
	}

	// Include allowed user IDs if select_users
	type libraryWithPerms struct {
		*models.Library
		AllowedUsers []uuid.UUID `json:"allowed_users,omitempty"`
	}
	resp := libraryWithPerms{Library: library}
	if library.AccessLevel == models.LibraryAccessSelectUsers {
		resp.AllowedUsers, _ = s.libRepo.GetPermissions(library.ID)
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: resp})
}

type updateLibraryRequest struct {
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	IsEnabled      bool     `json:"is_enabled"`
	ScanOnStartup  bool     `json:"scan_on_startup"`
	SeasonGrouping bool     `json:"season_grouping"`
	AccessLevel    string   `json:"access_level"`
	AllowedUsers   []string `json:"allowed_users"`
}

func (s *Server) handleUpdateLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	var req updateLibraryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Get existing library to preserve media_type
	existing, err := s.libRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "library not found")
		return
	}

	accessLevel := models.LibraryAccess(req.AccessLevel)
	if accessLevel == "" {
		accessLevel = existing.AccessLevel
	}

	library := models.Library{
		ID:             id,
		Name:           req.Name,
		MediaType:      existing.MediaType,
		Path:           req.Path,
		IsEnabled:      req.IsEnabled,
		ScanOnStartup:  req.ScanOnStartup,
		SeasonGrouping: req.SeasonGrouping,
		AccessLevel:    accessLevel,
	}

	if err := s.libRepo.Update(&library); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update library")
		return
	}

	// Update permissions
	if accessLevel == models.LibraryAccessSelectUsers {
		var userIDs []uuid.UUID
		for _, uid := range req.AllowedUsers {
			if parsed, err := uuid.Parse(uid); err == nil {
				userIDs = append(userIDs, parsed)
			}
		}
		if err := s.libRepo.SetPermissions(id, userIDs); err != nil {
			log.Printf("Failed to update library permissions: %v", err)
		}
	} else {
		// Clear permissions if no longer select_users
		_ = s.libRepo.SetPermissions(id, nil)
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: library})
}

func (s *Server) handleDeleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	if err := s.libRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete library")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "library deleted"}})
}

func (s *Server) handleScanLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	library, err := s.libRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "library not found")
		return
	}

	// If job queue is available, enqueue async scan
	if s.jobQueue != nil {
		jobID, err := s.jobQueue.Enqueue(jobs.TaskScanLibrary, jobs.ScanPayload{
			LibraryID: id.String(),
		})
		if err != nil {
			// Fallback to synchronous scan
			log.Printf("Failed to enqueue scan job, falling back to sync: %v", err)
		} else {
			log.Printf("Scan job enqueued for library %q: %s", library.Name, jobID)
			s.respondJSON(w, http.StatusAccepted, Response{Success: true, Data: map[string]string{
				"job_id":  jobID,
				"message": "scan job enqueued",
			}})
			return
		}
	}

	// Synchronous fallback
	log.Printf("Starting sync scan for library %q (%s) at %s", library.Name, library.MediaType, library.Path)

	result, err := s.scanner.ScanLibrary(library)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "scan failed: "+err.Error())
		return
	}

	_ = s.libRepo.UpdateLastScan(id)
	log.Printf("Scan complete: %d found, %d added, %d skipped, %d errors",
		result.FilesFound, result.FilesAdded, result.FilesSkipped, len(result.Errors))

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: result})
}
