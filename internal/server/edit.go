package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/JustinTDCT/CineVault/internal/catalog"
)

// editTitle saves a person's changes to a title's details. Every field sent
// is locked: lookups and cache records no longer change it (PUT …/locks
// unlocks it).
func (s *Server) editTitle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Bad title ID")
		return
	}
	var e catalog.TitleEdit
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		writeError(w, http.StatusBadRequest, "Send the fields to change as JSON: "+err.Error())
		return
	}
	switch {
	case e.Name != nil && strings.TrimSpace(*e.Name) == "":
		writeError(w, http.StatusBadRequest, "A title needs a name")
		return
	case e.Year != nil && *e.Year != 0 && (*e.Year < 1800 || *e.Year > 2200):
		writeError(w, http.StatusBadRequest, "Year must be between 1800 and 2200, or 0 for none")
		return
	case e.SeriesIndex != nil && *e.SeriesIndex < 0:
		writeError(w, http.StatusBadRequest, "The number in the series can't be negative")
		return
	case e.Overview != nil && len(*e.Overview) > 20000:
		writeError(w, http.StatusBadRequest, "The description is too long")
		return
	}
	if err := s.opt.Libraries.EditTitle(r.Context(), id, e); err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No such title")
			return
		}
		s.internalError(w, "save the title", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setTitleLocks replaces a title's locks: {"locked": true} stops every
// automatic change; "fields" lists the fields kept as edited.
func (s *Server) setTitleLocks(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "Bad title ID")
		return
	}
	var l catalog.TitleLocks
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&l); err != nil {
		writeError(w, http.StatusBadRequest, "Send {\"locked\": true|false, \"fields\": [...]}")
		return
	}
	out, err := s.opt.Libraries.SetTitleLocks(r.Context(), id, l)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			writeError(w, http.StatusNotFound, "No such title")
			return
		}
		s.internalError(w, "save the locks", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// updateLibrary changes a library's settings: prefer_local_metadata.
func (s *Server) updateLibrary(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.library(w, r)
	if !ok {
		return
	}
	var body struct {
		PreferLocalMetadata *bool `json:"prefer_local_metadata"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil || body.PreferLocalMetadata == nil {
		writeError(w, http.StatusBadRequest, "Send {\"prefer_local_metadata\": true|false}")
		return
	}
	out, err := s.opt.Libraries.SetPreferLocalMetadata(r.Context(), lib.ID, *body.PreferLocalMetadata)
	if err != nil {
		s.internalError(w, "save the library", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
