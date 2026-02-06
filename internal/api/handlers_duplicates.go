package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/fingerprint"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Duplicate Handlers ────────────────────

func (s *Server) handleListDuplicates(w http.ResponseWriter, r *http.Request) {
	// Query media items that have phash set, then find pairs with high similarity
	query := `SELECT id, phash, title, file_path FROM media_items WHERE phash IS NOT NULL AND phash != '' ORDER BY title`
	rows, err := s.db.DB.Query(query)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type hashEntry struct {
		ID       uuid.UUID
		PHash    string
		Title    string
		FilePath string
	}

	var entries []hashEntry
	for rows.Next() {
		var e hashEntry
		if err := rows.Scan(&e.ID, &e.PHash, &e.Title, &e.FilePath); err != nil {
			continue
		}
		entries = append(entries, e)
	}

	var pairs []map[string]interface{}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			sim := fingerprint.Similarity(entries[i].PHash, entries[j].PHash)
			if sim >= 0.90 {
				pairs = append(pairs, map[string]interface{}{
					"media_a":          entries[i].ID,
					"media_a_title":    entries[i].Title,
					"media_b":          entries[j].ID,
					"media_b_title":    entries[j].Title,
					"similarity_score": sim,
				})
			}
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: pairs})
}

func (s *Server) handleResolveDuplicate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MediaIDA string          `json:"media_id_a"`
		MediaIDB string          `json:"media_id_b"`
		Action   models.DuplicateAction `json:"action"`
		Notes    *string         `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := s.getUserID(r)
	mediaIDA, _ := uuid.Parse(req.MediaIDA)
	mediaIDB, _ := uuid.Parse(req.MediaIDB)

	decision := &models.DuplicateDecision{
		ID:         uuid.New(),
		MediaIDA:   &mediaIDA,
		MediaIDB:   &mediaIDB,
		Action:     req.Action,
		DecidedBy:  &userID,
		Notes:      req.Notes,
	}

	query := `INSERT INTO duplicate_decisions (id, media_id_a, media_id_b, action, decided_by, notes)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.DB.Exec(query, decision.ID, decision.MediaIDA, decision.MediaIDB,
		decision.Action, decision.DecidedBy, decision.Notes)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Execute action
	switch req.Action {
	case models.DuplicateDeleted:
		// Delete the second item
		s.mediaRepo.Delete(mediaIDB)
	case models.DuplicateSplitAsSister:
		// Create a sister group
		// This is handled by the existing sister group endpoints
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: decision})
}
