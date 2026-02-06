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
	libraries, err := s.libRepo.List()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list libraries")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: libraries})
}

func (s *Server) handleCreateLibrary(w http.ResponseWriter, r *http.Request) {
	var library models.Library
	if err := json.NewDecoder(r.Body).Decode(&library); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	library.ID = uuid.New()
	if err := s.libRepo.Create(&library); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create library")
		return
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

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: library})
}

func (s *Server) handleUpdateLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	var library models.Library
	if err := json.NewDecoder(r.Body).Decode(&library); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	library.ID = id
	if err := s.libRepo.Update(&library); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update library")
		return
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
