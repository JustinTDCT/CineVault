package libraries

import "time"

type Library struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	LibraryType       LibraryType `json:"library_type"`
	Folders           []string    `json:"folders"`
	IncludeInHomepage bool        `json:"include_in_homepage"`
	IncludeInSearch   bool        `json:"include_in_search"`
	RetrieveMetadata  bool        `json:"retrieve_metadata"`
	ImportNFO         bool        `json:"import_nfo"`
	ExportNFO         bool        `json:"export_nfo"`
	NormalizeAudio    bool        `json:"normalize_audio"`
	TimelineScrubbing bool        `json:"timeline_scrubbing"`
	PreviewVideos     bool        `json:"preview_videos"`
	IntroDetection    bool        `json:"intro_detection"`
	CreditsDetection  bool        `json:"credits_detection"`
	RecapDetection    bool        `json:"recap_detection"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type PermissionLevel string

const (
	PermissionNone PermissionLevel = "none"
	PermissionView PermissionLevel = "view"
	PermissionEdit PermissionLevel = "edit"
)

type LibraryPermission struct {
	ID              string          `json:"id"`
	LibraryID       string          `json:"library_id"`
	UserID          string          `json:"user_id"`
	PermissionLevel PermissionLevel `json:"permission_level"`
}

type ScanState struct {
	ID                string     `json:"id"`
	LibraryID         string     `json:"library_id"`
	LastScanStarted   *time.Time `json:"last_scan_started,omitempty"`
	LastScanCompleted *time.Time `json:"last_scan_completed,omitempty"`
	FilesScanned      int        `json:"files_scanned"`
	FilesAdded        int        `json:"files_added"`
	FilesRemoved      int        `json:"files_removed"`
	Status            string     `json:"status"`
}
