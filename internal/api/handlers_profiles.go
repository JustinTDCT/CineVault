package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Profile Settings (Parental Controls, Kids Mode, Avatar) ────────────────────

type UpdateProfileSettingsRequest struct {
	MaxContentRating *string `json:"max_content_rating"` // null = unrestricted, "G", "PG", "PG-13", "R", "NC-17"
	IsKidsProfile    *bool   `json:"is_kids_profile"`
	AvatarID         *string `json:"avatar_id"`
}

// handleGetProfileSettings returns the current user's profile settings.
func (s *Server) handleGetProfileSettings(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"max_content_rating": user.MaxContentRating,
			"is_kids_profile":    user.IsKidsProfile,
			"avatar_id":          user.AvatarID,
		},
	})
}

// handleUpdateProfileSettings updates parental controls, kids mode, and avatar.
func (s *Server) handleUpdateProfileSettings(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate content rating if provided
	if req.MaxContentRating != nil && *req.MaxContentRating != "" {
		validRatings := map[string]bool{
			"G": true, "PG": true, "PG-13": true, "R": true, "NC-17": true,
		}
		if !validRatings[*req.MaxContentRating] {
			s.respondError(w, http.StatusBadRequest, "invalid content rating. Use: G, PG, PG-13, R, NC-17")
			return
		}
	}

	userID := s.getUserID(r)

	// Handle "clear" — if max_content_rating is empty string, set to nil (unrestricted)
	var maxRating *string
	if req.MaxContentRating != nil {
		if *req.MaxContentRating == "" {
			maxRating = nil
		} else {
			maxRating = req.MaxContentRating
		}
	}

	// If enabling kids mode, auto-set max rating to PG if not already set
	if req.IsKidsProfile != nil && *req.IsKidsProfile && maxRating == nil && req.MaxContentRating == nil {
		pg := "PG"
		maxRating = &pg
	}

	if err := s.userRepo.UpdateProfileSettings(userID, maxRating, req.IsKidsProfile, req.AvatarID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update profile settings")
		return
	}

	// Re-fetch and return updated user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to fetch updated profile")
		return
	}
	user.PasswordHash = ""

	// Update the JWT token with fresh data
	token, err := s.auth.GenerateToken(user)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"user":  user,
			"token": token,
		},
	})
}

// handleAdminUpdateUserSettings allows an admin to set parental controls for any user.
func (s *Server) handleAdminUpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req UpdateProfileSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate content rating if provided
	if req.MaxContentRating != nil && *req.MaxContentRating != "" {
		validRatings := map[string]bool{
			"G": true, "PG": true, "PG-13": true, "R": true, "NC-17": true,
		}
		if !validRatings[*req.MaxContentRating] {
			s.respondError(w, http.StatusBadRequest, "invalid content rating")
			return
		}
	}

	var maxRating *string
	if req.MaxContentRating != nil {
		if *req.MaxContentRating == "" {
			maxRating = nil
		} else {
			maxRating = req.MaxContentRating
		}
	}

	if req.IsKidsProfile != nil && *req.IsKidsProfile && maxRating == nil && req.MaxContentRating == nil {
		pg := "PG"
		maxRating = &pg
	}

	if err := s.userRepo.UpdateProfileSettings(userID, maxRating, req.IsKidsProfile, req.AvatarID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update user settings")
		return
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to fetch updated user")
		return
	}
	user.PasswordHash = ""

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: user})
}

// ──────────────────── Recommendations ────────────────────

// getRecommendationContext extracts common recommendation params for the current user.
func (s *Server) getRecommendationContext(r *http.Request) (uuid.UUID, []string, []uuid.UUID, error) {
	userID := s.getUserID(r)
	role := models.UserRole(r.Header.Get("X-User-Role"))

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return uuid.Nil, nil, nil, err
	}

	libs, err := s.libRepo.ListForUser(userID, role)
	if err != nil {
		return uuid.Nil, nil, nil, err
	}
	var libIDs []uuid.UUID
	for _, lib := range libs {
		if lib.IsEnabled {
			libIDs = append(libIDs, lib.ID)
		}
	}

	var allowedRatings []string
	if user.MaxContentRating != nil && *user.MaxContentRating != "" {
		allowedRatings = models.AllowedContentRatings(*user.MaxContentRating)
	}

	return userID, allowedRatings, libIDs, nil
}

// handleRecommendations returns personalized media recommendations for the current user.
func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	userID, allowedRatings, libIDs, err := s.getRecommendationContext(r)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get user context")
		return
	}

	items, err := s.watchRepo.Recommendations(userID, 20, allowedRatings, libIDs)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get recommendations")
		return
	}

	if items == nil {
		items = []*models.MediaItem{}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: items})
}

// handleBecauseYouWatched returns "Because you watched X" recommendation rows.
func (s *Server) handleBecauseYouWatched(w http.ResponseWriter, r *http.Request) {
	userID, allowedRatings, libIDs, err := s.getRecommendationContext(r)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get user context")
		return
	}

	rows, err := s.watchRepo.BecauseYouWatched(userID, 5, 8, allowedRatings, libIDs)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to get because-you-watched")
		return
	}

	if rows == nil {
		rows = []*models.BecauseYouWatchedRow{}
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: rows})
}
