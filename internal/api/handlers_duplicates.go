package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/JustinTDCT/CineVault/internal/fingerprint"
	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Duplicate Handlers ────────────────────

// DuplicateGroup represents a flagged item with its matching items and prior decisions.
type DuplicateGroup struct {
	Item       *models.MediaItem        `json:"item"`
	DupType    string                   `json:"dup_type"`  // "potential"
	Matches    []DuplicateMatch         `json:"matches"`
	Decisions  []DuplicateDecisionInfo  `json:"decisions"`
}

type DuplicateMatch struct {
	Item       *models.MediaItem `json:"item"`
	MatchType  string            `json:"match_type"` // "phash"
	Similarity float64           `json:"similarity"`
}

type DuplicateDecisionInfo struct {
	Action    string    `json:"action"`
	DecidedAt time.Time `json:"decided_at"`
	DecidedBy string    `json:"decided_by"`
	Notes     *string   `json:"notes,omitempty"`
}

// handleListDuplicates returns items flagged as exact or potential duplicates,
// grouped with their matching items and prior decisions.
func (s *Server) handleListDuplicates(w http.ResponseWriter, r *http.Request) {
	items, err := s.mediaRepo.ListDuplicateItems()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build groups
	var groups []DuplicateGroup
	seen := make(map[uuid.UUID]bool)

	for _, item := range items {
		if seen[item.ID] {
			continue
		}
		seen[item.ID] = true

		group := DuplicateGroup{
			Item:    item,
			DupType: item.DuplicateStatus,
		}

		// Find matching items
		matches := s.findMatches(item)
		for _, m := range matches {
			seen[m.Item.ID] = true
		}
		group.Matches = matches

		// Get prior decisions
		group.Decisions = s.getPriorDecisions(item.ID)

		groups = append(groups, group)
	}

	// Also return count for nav badge
	count, _ := s.mediaRepo.CountUnreviewedDuplicates()

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"groups":           groups,
		"unreviewed_count": count,
	}})
}

// findMatches finds items that match the given item by perceptual hash similarity.
// Uses duration pre-filter (within 5%) before comparing hashes.
func (s *Server) findMatches(item *models.MediaItem) []DuplicateMatch {
	var matches []DuplicateMatch

	if item.Phash == nil || *item.Phash == "" {
		return matches
	}

	phashItems, err := s.mediaRepo.ListPhashesInLibrary(item.LibraryID)
	if err != nil {
		return matches
	}

	for _, p := range phashItems {
		if p.ID == item.ID || p.Phash == nil {
			continue
		}

		// Duration pre-filter: skip if durations differ by more than 5%
		if item.DurationSeconds != nil && p.DurationSeconds != nil {
			durA := float64(*item.DurationSeconds)
			durB := float64(*p.DurationSeconds)
			if durA > 0 && durB > 0 {
				ratio := durA / durB
				if ratio < 0.95 || ratio > 1.05 {
					continue
				}
			}
		}

		sim := fingerprint.Similarity(*item.Phash, *p.Phash)
		if sim >= 0.90 {
			matches = append(matches, DuplicateMatch{
				Item:       p,
				MatchType:  "phash",
				Similarity: sim,
			})
		}
	}

	return matches
}

// getPriorDecisions returns previous duplicate decisions involving this item.
func (s *Server) getPriorDecisions(itemID uuid.UUID) []DuplicateDecisionInfo {
	query := `SELECT dd.action, dd.decided_at, COALESCE(u.username, 'unknown'), dd.notes
		FROM duplicate_decisions dd
		LEFT JOIN users u ON dd.decided_by = u.id
		WHERE dd.media_id_a = $1 OR dd.media_id_b = $1
		ORDER BY dd.decided_at DESC`
	rows, err := s.db.DB.Query(query, itemID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var decisions []DuplicateDecisionInfo
	for rows.Next() {
		var d DuplicateDecisionInfo
		if err := rows.Scan(&d.Action, &d.DecidedAt, &d.DecidedBy, &d.Notes); err != nil {
			continue
		}
		decisions = append(decisions, d)
	}
	return decisions
}

// handleResolveDuplicate resolves a duplicate with the specified action.
func (s *Server) handleResolveDuplicate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MediaID     string              `json:"media_id"`
		PartnerID   string              `json:"partner_id"`
		Action      models.DuplicateAction `json:"action"`
		Notes       *string             `json:"notes"`
		DeleteFile  bool                `json:"delete_file"`
		// Edition merge fields
		EditionLabel string             `json:"edition_label"`
		PrimaryID    string             `json:"primary_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := s.getUserID(r)
	mediaID, _ := uuid.Parse(req.MediaID)
	partnerID, _ := uuid.Parse(req.PartnerID)

	// Record the decision (media_id_b is nullable when deleting a single item)
	decision := &models.DuplicateDecision{
		ID:       uuid.New(),
		MediaIDA: &mediaID,
		Action:   req.Action,
		DecidedBy: &userID,
		Notes:    req.Notes,
	}
	if partnerID != uuid.Nil {
		decision.MediaIDB = &partnerID
	}

	insertQuery := `INSERT INTO duplicate_decisions (id, media_id_a, media_id_b, action, decided_by, notes)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.DB.Exec(insertQuery, decision.ID, decision.MediaIDA, decision.MediaIDB,
		decision.Action, decision.DecidedBy, decision.Notes)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Execute the action
	switch req.Action {
	case models.DuplicateEdit:
		// Mark as addressed; actual editing is done via PUT /media/{id}
		_ = s.mediaRepo.UpdateDuplicateStatus(mediaID, "addressed")
		if partnerID != uuid.Nil {
			_ = s.mediaRepo.UpdateDuplicateStatus(partnerID, "addressed")
		}

	case models.DuplicateEdition:
		// Create/link edition group
		if err := s.mergeAsEdition(mediaID, partnerID, req.EditionLabel, req.PrimaryID, userID); err != nil {
			s.respondError(w, http.StatusInternalServerError, "edition merge failed: "+err.Error())
			return
		}
		_ = s.mediaRepo.UpdateDuplicateStatus(mediaID, "addressed")
		_ = s.mediaRepo.UpdateDuplicateStatus(partnerID, "addressed")

	case models.DuplicateDeleted:
		// Delete the media item (optionally the file)
		item, err := s.mediaRepo.GetByID(mediaID)
		if err != nil || item == nil {
			s.respondError(w, http.StatusNotFound, "media item not found")
			return
		}
		if req.DeleteFile {
			log.Printf("Duplicate delete: attempting to remove file: %s", item.FilePath)
			if err := os.Remove(item.FilePath); err != nil {
				log.Printf("Failed to delete file %s: %v", item.FilePath, err)
				s.respondError(w, http.StatusInternalServerError, "Failed to delete file from disk: "+err.Error())
				return
			}
			log.Printf("Duplicate delete: successfully removed file: %s", item.FilePath)
		}
		_ = s.mediaRepo.Delete(mediaID)
		// Reset partner back to none since its match was deleted
		if partnerID != uuid.Nil {
			_ = s.mediaRepo.UpdateDuplicateStatus(partnerID, "none")
		}

	case models.DuplicateIgnored:
		_ = s.mediaRepo.UpdateDuplicateStatus(mediaID, "addressed")
		if partnerID != uuid.Nil {
			_ = s.mediaRepo.UpdateDuplicateStatus(partnerID, "addressed")
		}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: decision})
}

// mergeAsEdition creates or extends an edition group with two items.
func (s *Server) mergeAsEdition(itemA, itemB uuid.UUID, editionLabel, primaryIDStr string, userID uuid.UUID) error {
	primaryID, _ := uuid.Parse(primaryIDStr)
	if primaryID == uuid.Nil {
		primaryID = itemA
	}

	// Determine the non-primary item
	secondaryID := itemB
	if primaryID == itemB {
		secondaryID = itemA
	}

	// Get primary item for group metadata
	primary, err := s.mediaRepo.GetByID(primaryID)
	if err != nil {
		return err
	}

	// Check if primary is already in an edition group
	var existingGroupID *uuid.UUID
	err = s.db.DB.QueryRow(`SELECT eg.id FROM edition_groups eg
		JOIN edition_items ei ON ei.edition_group_id = eg.id
		WHERE ei.media_item_id = $1 LIMIT 1`, primaryID).Scan(&existingGroupID)

	var groupID uuid.UUID
	if err == nil && existingGroupID != nil {
		groupID = *existingGroupID
	} else {
		// Create new edition group
		groupID = uuid.New()
		_, err = s.db.DB.Exec(`INSERT INTO edition_groups
			(id, library_id, media_type, title, sort_title, year, description, poster_path, backdrop_path)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			groupID, primary.LibraryID, primary.MediaType, primary.Title,
			primary.SortTitle, primary.Year, primary.Description,
			primary.PosterPath, primary.BackdropPath)
		if err != nil {
			return err
		}

		// Add primary item to group
		_, err = s.db.DB.Exec(`INSERT INTO edition_items
			(id, edition_group_id, media_item_id, edition_type, is_default, sort_order, added_by)
			VALUES ($1, $2, $3, $4, true, 0, $5)`,
			uuid.New(), groupID, primaryID, "Theatrical", userID)
		if err != nil {
			return err
		}
	}

	// Add secondary item to group
	label := editionLabel
	if label == "" {
		label = "Alternate"
	}

	// Count existing items to determine sort order
	var sortOrder int
	s.db.DB.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM edition_items WHERE edition_group_id = $1`, groupID).Scan(&sortOrder)

	_, err = s.db.DB.Exec(`INSERT INTO edition_items
		(id, edition_group_id, media_item_id, edition_type, is_default, sort_order, added_by)
		VALUES ($1, $2, $3, $4, false, $5, $6)
		ON CONFLICT DO NOTHING`,
		uuid.New(), groupID, secondaryID, label, sortOrder, userID)
	return err
}

// handleGetMediaDuplicates returns matching items for a specific media item.
func (s *Server) handleGetMediaDuplicates(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media ID")
		return
	}

	item, err := s.mediaRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "media item not found")
		return
	}

	matches := s.findMatches(item)
	decisions := s.getPriorDecisions(id)

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]interface{}{
		"item":      item,
		"matches":   matches,
		"decisions": decisions,
	}})
}

// handleGetDuplicateCount returns just the unreviewed duplicate count (for nav badge).
func (s *Server) handleGetDuplicateCount(w http.ResponseWriter, r *http.Request) {
	count, err := s.mediaRepo.CountUnreviewedDuplicates()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]int{"count": count}})
}

// handleClearStaleDuplicates resets duplicate_status to 'none' for items whose
// matches no longer exist (deleted or resolved). Avoids a full library rescan.
func (s *Server) handleClearStaleDuplicates(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.DB.Exec(`UPDATE media_items SET duplicate_status = 'none'
		WHERE duplicate_status != 'none'
		AND id NOT IN (
			SELECT DISTINCT m1.id FROM media_items m1
			JOIN media_items m2 ON m1.library_id = m2.library_id
				AND m1.id != m2.id
				AND m1.phash IS NOT NULL AND m2.phash IS NOT NULL
				AND m1.duplicate_status != 'none' AND m2.duplicate_status != 'none'
		)`)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	count, _ := result.RowsAffected()
	log.Printf("Cleared %d stale duplicate flags", count)
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]int64{"cleared": count}})
}
