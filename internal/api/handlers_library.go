package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/JustinTDCT/CineVault/internal/jobs"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
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
	Name               string   `json:"name"`
	MediaType          string   `json:"media_type"`
	Path               string   `json:"path"`
	Folders            []string `json:"folders"`
	IsEnabled          bool     `json:"is_enabled"`
	SeasonGrouping     bool     `json:"season_grouping"`
	AccessLevel        string   `json:"access_level"`
	AllowedUsers       []string `json:"allowed_users"`
	IncludeInHomepage  *bool    `json:"include_in_homepage"`
	IncludeInSearch    *bool    `json:"include_in_search"`
	RetrieveMetadata   *bool    `json:"retrieve_metadata"`
	NFOImport          *bool    `json:"nfo_import"`
	NFOExport          *bool    `json:"nfo_export"`
	PreferLocalArtwork *bool    `json:"prefer_local_artwork"`
	AdultContentType   *string  `json:"adult_content_type"`
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

	// Defaults for booleans (true if not specified)
	includeHomepage := true
	if req.IncludeInHomepage != nil {
		includeHomepage = *req.IncludeInHomepage
	}
	includeSearch := true
	if req.IncludeInSearch != nil {
		includeSearch = *req.IncludeInSearch
	}
	retrieveMeta := true
	if req.RetrieveMetadata != nil {
		retrieveMeta = *req.RetrieveMetadata
	}
	nfoImport := false
	if req.NFOImport != nil {
		nfoImport = *req.NFOImport
	}
	nfoExport := false
	if req.NFOExport != nil {
		nfoExport = *req.NFOExport
	}
	preferLocalArtwork := true // default on
	if req.PreferLocalArtwork != nil {
		preferLocalArtwork = *req.PreferLocalArtwork
	}

	// Determine primary path from folders or path field
	primaryPath := req.Path
	if len(req.Folders) > 0 && req.Folders[0] != "" {
		primaryPath = req.Folders[0]
	}

	library := models.Library{
		ID:                 uuid.New(),
		Name:               req.Name,
		MediaType:          models.MediaType(req.MediaType),
		Path:               primaryPath,
		IsEnabled:          req.IsEnabled,
		SeasonGrouping:     req.SeasonGrouping,
		AccessLevel:        accessLevel,
		IncludeInHomepage:  includeHomepage,
		IncludeInSearch:    includeSearch,
		RetrieveMetadata:   retrieveMeta,
		NFOImport:          nfoImport,
		NFOExport:          nfoExport,
		PreferLocalArtwork: preferLocalArtwork,
		AdultContentType:   req.AdultContentType,
	}

	if err := s.libRepo.Create(&library); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create library")
		return
	}

	// Set folders
	folders := req.Folders
	if len(folders) == 0 && primaryPath != "" {
		folders = []string{primaryPath}
	}
	if len(folders) > 0 {
		if err := s.libRepo.SetFolders(library.ID, folders); err != nil {
			log.Printf("Failed to set library folders: %v", err)
		}
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

	// Reload with folders
	created, _ := s.libRepo.GetByID(library.ID)
	if created == nil {
		created = &library
	}

	// Auto-scan the newly created library
	go func() {
		if s.jobQueue != nil {
			uniqueID := "scan:" + library.ID.String()
			jobID, err := s.jobQueue.EnqueueUnique(jobs.TaskScanLibrary, jobs.ScanPayload{
				LibraryID: library.ID.String(),
			}, uniqueID, asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour))
			if err != nil {
				log.Printf("Auto-scan: failed to enqueue for library %q, falling back to sync: %v", library.Name, err)
			} else {
				log.Printf("Auto-scan: job enqueued for new library %q: %s", library.Name, jobID)
				return
			}
		}
		// Synchronous fallback
		log.Printf("Auto-scan: starting sync scan for new library %q", library.Name)
		result, err := s.scanner.ScanLibrary(created)
		if err != nil {
			log.Printf("Auto-scan: sync scan failed for library %q: %v", library.Name, err)
			return
		}
		_ = s.libRepo.UpdateLastScan(library.ID)
		log.Printf("Auto-scan: complete for %q — %d found, %d added, %d skipped, %d errors",
			library.Name, result.FilesFound, result.FilesAdded, result.FilesSkipped, len(result.Errors))
	}()

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: created})
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
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	Folders            []string `json:"folders"`
	IsEnabled          bool     `json:"is_enabled"`
	ScanOnStartup      bool     `json:"scan_on_startup"`
	SeasonGrouping     bool     `json:"season_grouping"`
	AccessLevel        string   `json:"access_level"`
	AllowedUsers       []string `json:"allowed_users"`
	IncludeInHomepage  *bool    `json:"include_in_homepage"`
	IncludeInSearch    *bool    `json:"include_in_search"`
	RetrieveMetadata   *bool    `json:"retrieve_metadata"`
	NFOImport          *bool    `json:"nfo_import"`
	NFOExport          *bool    `json:"nfo_export"`
	PreferLocalArtwork *bool    `json:"prefer_local_artwork"`
	AdultContentType   *string  `json:"adult_content_type"`
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

	// Defaults: preserve existing values if not specified
	includeHomepage := existing.IncludeInHomepage
	if req.IncludeInHomepage != nil {
		includeHomepage = *req.IncludeInHomepage
	}
	includeSearch := existing.IncludeInSearch
	if req.IncludeInSearch != nil {
		includeSearch = *req.IncludeInSearch
	}
	retrieveMeta := existing.RetrieveMetadata
	if req.RetrieveMetadata != nil {
		retrieveMeta = *req.RetrieveMetadata
	}
	nfoImport := existing.NFOImport
	if req.NFOImport != nil {
		nfoImport = *req.NFOImport
	}
	nfoExport := existing.NFOExport
	if req.NFOExport != nil {
		nfoExport = *req.NFOExport
	}
	preferLocalArtwork := existing.PreferLocalArtwork
	if req.PreferLocalArtwork != nil {
		preferLocalArtwork = *req.PreferLocalArtwork
	}
	adultContentType := existing.AdultContentType
	if req.AdultContentType != nil {
		adultContentType = req.AdultContentType
	}

	// Determine primary path
	primaryPath := req.Path
	if len(req.Folders) > 0 && req.Folders[0] != "" {
		primaryPath = req.Folders[0]
	}

	library := models.Library{
		ID:                 id,
		Name:               req.Name,
		MediaType:          existing.MediaType,
		Path:               primaryPath,
		IsEnabled:          req.IsEnabled,
		ScanOnStartup:      req.ScanOnStartup,
		SeasonGrouping:     req.SeasonGrouping,
		AccessLevel:        accessLevel,
		IncludeInHomepage:  includeHomepage,
		IncludeInSearch:    includeSearch,
		RetrieveMetadata:   retrieveMeta,
		NFOImport:          nfoImport,
		NFOExport:          nfoExport,
		PreferLocalArtwork: preferLocalArtwork,
		AdultContentType:   adultContentType,
	}

	if err := s.libRepo.Update(&library); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update library")
		return
	}

	// Update folders if provided
	if len(req.Folders) > 0 {
		if err := s.libRepo.SetFolders(id, req.Folders); err != nil {
			log.Printf("Failed to update library folders: %v", err)
		}
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

	// If job queue is available, enqueue async scan (deduplicated by library ID)
	if s.jobQueue != nil {
		uniqueID := "scan:" + id.String()
		jobID, err := s.jobQueue.EnqueueUnique(jobs.TaskScanLibrary, jobs.ScanPayload{
			LibraryID: id.String(),
		}, uniqueID, asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour))
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

// handlePhashLibrary enqueues a perceptual hash computation job for a library.
func (s *Server) handlePhashLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library id")
		return
	}

	library, err := s.libRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "library not found")
		return
	}

	if s.jobQueue == nil {
		s.respondError(w, http.StatusServiceUnavailable, "job queue not available")
		return
	}

	uniqueID := "phash:" + id.String()
	jobID, err := s.jobQueue.EnqueueUnique(jobs.TaskPhashLibrary, jobs.PhashLibraryPayload{
		LibraryID: id.String(),
	}, uniqueID, asynq.Timeout(6*time.Hour), asynq.Retention(1*time.Hour))
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	log.Printf("Phash job enqueued for library %q: %s", library.Name, jobID)
	s.respondJSON(w, http.StatusAccepted, Response{Success: true, Data: map[string]string{
		"job_id":  jobID,
		"message": "phash job enqueued",
	}})
}

// handleLibraryFilters returns available filter options for a library.
func (s *Server) handleLibraryFilters(w http.ResponseWriter, r *http.Request) {
	libraryID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid library ID")
		return
	}

	opts, err := s.mediaRepo.GetLibraryFilterOptions(libraryID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get filter options")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: opts})
}
