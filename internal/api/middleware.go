package api

import "github.com/JustinTDCT/CineVault/internal/httputil"

var (
	WriteJSON  = httputil.WriteJSON
	WriteError = httputil.WriteError
	ReadJSON   = httputil.ReadJSON
)
