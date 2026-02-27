package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListSisters(w http.ResponseWriter, r *http.Request) {
	groups, err := s.sisterRepo.List(100, 0)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list sister groups")
		return
	}
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: groups})
}

func (s *Server) handleCreateSister(w http.ResponseWriter, r *http.Request) {
	var group models.SisterGroup
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	group.ID = uuid.New()
	userID := s.getUserID(r)
	group.CreatedBy = &userID

	if err := s.sisterRepo.Create(&group); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create sister group")
		return
	}

	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: group})
}

func (s *Server) handleGetSister(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid sister group ID")
		return
	}

	group, err := s.sisterRepo.GetByID(id)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "sister group not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: group})
}

func (s *Server) handleDeleteSister(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid sister group ID")
		return
	}

	if err := s.sisterRepo.Delete(id); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete sister group")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "sister group deleted"}})
}

type AddSisterItemRequest struct {
	MediaItemID string `json:"media_item_id"`
}

func (s *Server) handleAddSisterItem(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid sister group ID")
		return
	}

	var req AddSisterItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mediaItemID, err := uuid.Parse(req.MediaItemID)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid media item ID")
		return
	}

	if err := s.sisterRepo.AddMember(groupID, mediaItemID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to add item to sister group")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "item added"}})
}

func (s *Server) handleRemoveSisterItem(w http.ResponseWriter, r *http.Request) {
	groupID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid sister group ID")
		return
	}
	itemID, err := uuid.Parse(r.PathValue("itemId"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid item ID")
		return
	}

	if err := s.sisterRepo.RemoveMember(groupID, itemID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to remove item from sister group")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"message": "item removed"}})
}

var regroupMultiPartRx = regexp.MustCompile(`(?i)[\s._-]+(CD|DISC|DISK|PART|PT)[\s._-]*(\d+)`)

// handleRegroupMultiParts scans all ungrouped media items for multi-part patterns
// and creates sister groups retroactively.
func (s *Server) handleRegroupMultiParts(w http.ResponseWriter, r *http.Request) {
	libs, err := s.libRepo.List()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list libraries")
		return
	}

	type multiPartEntry struct {
		itemID     uuid.UUID
		partNumber int
	}

	totalGrouped := 0
	for _, lib := range libs {
		if lib.MediaType != models.MediaTypeMovies && lib.MediaType != models.MediaTypeAdultMovies {
			continue
		}

		items, err := s.mediaRepo.ListAllByLibrary(lib.ID)
		if err != nil {
			log.Printf("Regroup: failed to list items for library %s: %v", lib.ID, err)
			continue
		}

		pending := make(map[string][]multiPartEntry)
		displayNames := make(map[string]string) // key → original-cased title
		for _, item := range items {
			if item.SisterGroupID != nil {
				continue
			}
			baseName := strings.TrimSuffix(item.FileName, filepath.Ext(item.FileName))
			allLocs := regroupMultiPartRx.FindAllStringSubmatchIndex(baseName, -1)
			if len(allLocs) == 0 {
				continue
			}
			last := allLocs[len(allLocs)-1]
			partStr := baseName[last[4]:last[5]]
			partNum, _ := strconv.Atoi(partStr)
			baseTitle := strings.TrimSpace(baseName[:last[0]])

			dir := filepath.Dir(item.FilePath)
			key := dir + "|" + strings.ToLower(baseTitle)
			pending[key] = append(pending[key], multiPartEntry{
				itemID:     item.ID,
				partNumber: partNum,
			})
			if _, exists := displayNames[key]; !exists {
				displayNames[key] = baseTitle
			}
		}

		for key, parts := range pending {
			if len(parts) < 2 {
				continue
			}

			groupName := displayNames[key]

			group := &models.SisterGroup{
				ID:   uuid.New(),
				Name: groupName,
			}
			userID := s.getUserID(r)
			group.CreatedBy = &userID

			if err := s.sisterRepo.Create(group); err != nil {
				log.Printf("Regroup: failed to create sister group for %q: %v", groupName, err)
				continue
			}

			for _, part := range parts {
				if err := s.sisterRepo.AddMemberWithPosition(group.ID, part.itemID, part.partNumber); err != nil {
					log.Printf("Regroup: failed to add part %d to group: %v", part.partNumber, err)
				}
			}

			totalGrouped++
			log.Printf("Regroup: grouped %d parts as %q", len(parts), groupName)
		}
	}

	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]interface{}{"message": fmt.Sprintf("Created %d sister groups", totalGrouped)},
	})
}
