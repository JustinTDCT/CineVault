package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

// ──────────────────── Household Profiles ────────────────────

type HouseholdProfileResponse struct {
	ID               uuid.UUID       `json:"id"`
	DisplayName      *string         `json:"display_name,omitempty"`
	Username         string          `json:"username"`
	Role             models.UserRole `json:"role"`
	HasPin           bool            `json:"has_pin"`
	IsKidsProfile    bool            `json:"is_kids_profile"`
	AvatarID         *string         `json:"avatar_id,omitempty"`
	MaxContentRating *string         `json:"max_content_rating,omitempty"`
	IsMaster         bool            `json:"is_master"`
}

type HouseholdSwitchRequest struct {
	ProfileID uuid.UUID `json:"profile_id"`
	Pin       string    `json:"pin,omitempty"`
}

type CreateSubProfileRequest struct {
	DisplayName      string  `json:"display_name"`
	AvatarID         *string `json:"avatar_id,omitempty"`
	IsKidsProfile    bool    `json:"is_kids_profile"`
	MaxContentRating *string `json:"max_content_rating,omitempty"`
	Pin              string  `json:"pin,omitempty"`
}

type UpdateSubProfileRequest struct {
	DisplayName      *string `json:"display_name,omitempty"`
	AvatarID         *string `json:"avatar_id,omitempty"`
	IsKidsProfile    *bool   `json:"is_kids_profile,omitempty"`
	MaxContentRating *string `json:"max_content_rating"`
	Pin              *string `json:"pin"`
}

// handleHouseholdProfiles returns the current user's household profiles.
func (s *Server) handleHouseholdProfiles(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}

	// Determine the master ID for this household
	masterID := user.ID
	if user.ParentUserID != nil {
		masterID = *user.ParentUserID
	}

	profiles, err := s.userRepo.ListHousehold(masterID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to list household profiles")
		return
	}

	var result []HouseholdProfileResponse
	for _, p := range profiles {
		result = append(result, HouseholdProfileResponse{
			ID:               p.ID,
			DisplayName:      p.DisplayName,
			Username:         p.Username,
			Role:             p.Role,
			HasPin:           p.HasPin,
			IsKidsProfile:    p.IsKidsProfile,
			AvatarID:         p.AvatarID,
			MaxContentRating: p.MaxContentRating,
			IsMaster:         p.IsMaster,
		})
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: result})
}

// handleHouseholdSwitch switches to another profile within the same household.
func (s *Server) handleHouseholdSwitch(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	currentUser, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}

	var req HouseholdSwitchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Determine the master ID for the current user's household
	currentMasterID := currentUser.ID
	if currentUser.ParentUserID != nil {
		currentMasterID = *currentUser.ParentUserID
	}

	// Load the target profile
	target, err := s.userRepo.GetByID(req.ProfileID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "profile not found")
		return
	}

	// Determine the master ID for the target profile
	targetMasterID := target.ID
	if target.ParentUserID != nil {
		targetMasterID = *target.ParentUserID
	}

	// Verify both are in the same household
	if currentMasterID != targetMasterID {
		s.respondError(w, http.StatusForbidden, "profile is not in your household")
		return
	}

	if !target.IsActive {
		s.respondError(w, http.StatusForbidden, "profile is disabled")
		return
	}

	// If target has a PIN, verify it
	if target.HasPin {
		if req.Pin == "" {
			s.respondError(w, http.StatusUnauthorized, "PIN required for this profile")
			return
		}
		if err := s.auth.VerifyPassword(*target.PinHash, req.Pin); err != nil {
			s.respondError(w, http.StatusUnauthorized, "invalid PIN")
			return
		}
	}

	// Generate a new JWT for the target profile
	token, err := s.auth.GenerateToken(target)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	s.recordSession(target.ID, token, r)

	target.PasswordHash = ""
	s.respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    LoginResponse{Token: token, User: target},
	})
}

// handleCreateSubProfile creates a new sub-profile under the current master user.
func (s *Server) handleCreateSubProfile(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	master, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}

	// Only master users can create sub-profiles
	if master.ParentUserID != nil {
		s.respondError(w, http.StatusForbidden, "only master users can create sub-profiles")
		return
	}

	var req CreateSubProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.DisplayName == "" {
		s.respondError(w, http.StatusBadRequest, "display_name is required")
		return
	}

	// Enforce max 5 sub-profiles
	count, err := s.userRepo.CountByParent(master.ID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to count sub-profiles")
		return
	}
	if count >= 5 {
		s.respondError(w, http.StatusConflict, "maximum 5 sub-profiles per household")
		return
	}

	// Generate username from display name (e.g. "Kids" → "jdube_kids")
	subID := uuid.New()
	slug := strings.ToLower(strings.TrimSpace(req.DisplayName))
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "_")
	slug = strings.Trim(slug, "_")
	if slug == "" {
		slug = fmt.Sprintf("profile%d", count+1)
	}
	sysUsername := fmt.Sprintf("%s_%s", master.Username, slug)
	sysEmail := fmt.Sprintf("%s@household.local", sysUsername)

	// Hash PIN if provided
	var pinHash *string
	if req.Pin != "" {
		for _, c := range req.Pin {
			if !unicode.IsDigit(c) {
				s.respondError(w, http.StatusBadRequest, "PIN must contain only digits")
				return
			}
		}
		pinLenStr, _ := s.settingsRepo.Get("fast_login_pin_length")
		if pinLenStr == "" {
			pinLenStr = "4"
		}
		requiredLen, _ := strconv.Atoi(pinLenStr)
		if len(req.Pin) != requiredLen {
			s.respondError(w, http.StatusBadRequest, fmt.Sprintf("PIN must be exactly %d digits", requiredLen))
			return
		}
		h, err := s.auth.HashPassword(req.Pin)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, "failed to hash PIN")
			return
		}
		pinHash = &h
	}

	subUser := &models.User{
		ID:               subID,
		Username:         sysUsername,
		Email:            sysEmail,
		PasswordHash:     "$2a$10$UNUSABLE_HASH_FOR_SUBPROFILE_NO_DIRECT_LOGIN", // unusable bcrypt
		PinHash:          pinHash,
		DisplayName:      &req.DisplayName,
		Role:             models.RoleUser,
		IsActive:         true,
		IsKidsProfile:    req.IsKidsProfile,
		MaxContentRating: req.MaxContentRating,
		AvatarID:         req.AvatarID,
		ParentUserID:     &master.ID,
	}

	if err := s.userRepo.Create(subUser); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to create sub-profile")
		return
	}

	subUser.PasswordHash = ""
	s.respondJSON(w, http.StatusCreated, Response{Success: true, Data: subUser})
}

// handleUpdateSubProfile updates a sub-profile (master only).
func (s *Server) handleUpdateSubProfile(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	master, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}

	if master.ParentUserID != nil {
		s.respondError(w, http.StatusForbidden, "only master users can edit sub-profiles")
		return
	}

	subID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid profile id")
		return
	}

	// Verify ownership
	sub, err := s.userRepo.GetByID(subID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "sub-profile not found")
		return
	}
	if sub.ParentUserID == nil || *sub.ParentUserID != master.ID {
		s.respondError(w, http.StatusForbidden, "profile does not belong to your household")
		return
	}

	var req UpdateSubProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Update profile fields
	if err := s.userRepo.UpdateSubProfile(subID, req.DisplayName, req.AvatarID, req.IsKidsProfile, req.MaxContentRating); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to update sub-profile")
		return
	}

	// Update PIN if provided
	if req.Pin != nil {
		if *req.Pin == "" {
			// Clear PIN
			if err := s.userRepo.UpdatePinHash(subID, nil); err != nil {
				s.respondError(w, http.StatusInternalServerError, "failed to clear PIN")
				return
			}
		} else {
			for _, c := range *req.Pin {
				if !unicode.IsDigit(c) {
					s.respondError(w, http.StatusBadRequest, "PIN must contain only digits")
					return
				}
			}
			pinLenStr, _ := s.settingsRepo.Get("fast_login_pin_length")
			if pinLenStr == "" {
				pinLenStr = "4"
			}
			requiredLen, _ := strconv.Atoi(pinLenStr)
			if len(*req.Pin) != requiredLen {
				s.respondError(w, http.StatusBadRequest, fmt.Sprintf("PIN must be exactly %d digits", requiredLen))
				return
			}
			h, err := s.auth.HashPassword(*req.Pin)
			if err != nil {
				s.respondError(w, http.StatusInternalServerError, "failed to hash PIN")
				return
			}
			if err := s.userRepo.UpdatePinHash(subID, &h); err != nil {
				s.respondError(w, http.StatusInternalServerError, "failed to set PIN")
				return
			}
		}
	}

	// Return updated profile
	updated, err := s.userRepo.GetByID(subID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to fetch updated profile")
		return
	}
	updated.PasswordHash = ""
	s.respondJSON(w, http.StatusOK, Response{Success: true, Data: updated})
}

// handleDeleteSubProfile deletes a sub-profile (master only).
func (s *Server) handleDeleteSubProfile(w http.ResponseWriter, r *http.Request) {
	userID := s.getUserID(r)
	master, err := s.userRepo.GetByID(userID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "user not found")
		return
	}

	if master.ParentUserID != nil {
		s.respondError(w, http.StatusForbidden, "only master users can delete sub-profiles")
		return
	}

	subID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid profile id")
		return
	}

	// Verify ownership
	sub, err := s.userRepo.GetByID(subID)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "sub-profile not found")
		return
	}
	if sub.ParentUserID == nil || *sub.ParentUserID != master.ID {
		s.respondError(w, http.StatusForbidden, "profile does not belong to your household")
		return
	}

	if err := s.userRepo.Delete(subID); err != nil {
		s.respondError(w, http.StatusInternalServerError, "failed to delete sub-profile")
		return
	}

	s.respondJSON(w, http.StatusOK, Response{Success: true})
}
