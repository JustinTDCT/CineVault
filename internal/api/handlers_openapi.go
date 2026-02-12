package api

import (
	"encoding/json"
	"net/http"

	"github.com/JustinTDCT/CineVault/internal/version"
)

// handleOpenAPISpec serves the OpenAPI 3.0 specification (P10-05)
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "CineVault API",
			"description": "Media management and streaming server API",
			"version":     getAppVersion(),
		},
		"servers": []map[string]interface{}{
			{"url": "/api/v1", "description": "CineVault Server"},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
				"apiKeyAuth": map[string]interface{}{
					"type": "apiKey",
					"in":   "header",
					"name": "X-API-Key",
				},
			},
		},
		"security": []map[string]interface{}{
			{"bearerAuth": []string{}},
			{"apiKeyAuth": []string{}},
		},
		"paths": buildAPIPaths(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}

func getAppVersion() string {
	return version.Get().Version
}

func buildAPIPaths() map[string]interface{} {
	paths := map[string]interface{}{}

	// Helper to merge path entries (for multiple methods on same path)
	merge := func(path string, method string, e map[string]interface{}) {
		if p, ok := paths[path].(map[string]interface{}); ok {
			p[method] = e
		} else {
			paths[path] = map[string]interface{}{method: e}
		}
	}

	// ── Public ──
	merge("/status", "get", endpoint("Status", "public", "API status and version"))
	merge("/setup/check", "get", endpoint("Setup Check", "public", "Check if initial setup is required"))
	merge("/setup", "post", endpoint("Setup", "public", "Complete initial server setup"))

	// ── Auth ──
	merge("/auth/register", "post", endpoint("Register", "auth", "Register new user"))
	merge("/auth/login", "post", endpoint("Login", "auth", "Authenticate with username and password"))
	merge("/auth/fast-login/settings", "get", endpoint("Fast Login Settings", "auth", "Get fast login (PIN) configuration"))
	merge("/auth/fast-login/users", "get", endpoint("Fast Login Users", "auth", "List users available for PIN login"))
	merge("/auth/fast-login", "post", endpoint("PIN Login", "auth", "Authenticate with PIN"))
	merge("/auth/reset-token", "post", endpoint("Create Reset Token", "auth", "Admin creates password reset token (admin only)"))
	merge("/auth/reset-password", "post", endpoint("Reset Password", "auth", "Reset password using token"))
	merge("/auth/logout", "post", endpoint("Logout", "auth", "Invalidate current session"))
	merge("/auth/sessions", "get", endpoint("List Sessions", "auth", "List active sessions for user"))
	merge("/auth/sessions/{id}", "delete", endpoint("Revoke Session", "auth", "Revoke a specific session"))
	merge("/auth/pin", "put", endpoint("Set PIN", "auth", "Set or update user PIN"))
	merge("/auth/2fa/setup", "post", endpoint("Setup 2FA", "auth", "Generate TOTP secret and QR code URL"))
	merge("/auth/2fa/verify", "post", endpoint("Verify 2FA", "auth", "Verify TOTP code and enable 2FA"))
	merge("/auth/2fa", "delete", endpoint("Disable 2FA", "auth", "Disable two-factor authentication"))
	merge("/auth/2fa/status", "get", endpoint("2FA Status", "auth", "Get 2FA enrollment status"))
	merge("/auth/2fa/validate", "post", endpoint("Validate 2FA", "auth", "Validate TOTP during login"))

	// ── WebSocket ──
	merge("/ws", "get", endpoint("WebSocket", "ws", "WebSocket connection for real-time updates"))

	// ── Profile ──
	merge("/users", "get", endpoint("List Users", "profile", "List all users (admin only)"))
	merge("/profile", "get", endpoint("Get Profile", "profile", "Get current user profile"))
	merge("/profile", "put", endpoint("Update Profile", "profile", "Update current user profile"))
	merge("/users/{id}/pin", "put", endpoint("Admin Set PIN", "profile", "Admin sets PIN for user"))
	merge("/profile/settings", "get", endpoint("Get Profile Settings", "profile", "Get parental controls, kids mode, avatar"))
	merge("/profile/settings", "put", endpoint("Update Profile Settings", "profile", "Update profile settings"))
	merge("/users/{id}/settings", "put", endpoint("Admin Update User Settings", "profile", "Admin updates user settings"))
	merge("/profile/stats", "get", endpoint("Profile Stats", "profile", "Get user watch statistics"))
	merge("/profile/wrapped/{year}", "get", endpoint("Year Wrapped", "profile", "Get year-in-review summary"))

	// ── Browse (admin) ──
	merge("/browse", "get", endpoint("Browse Filesystem", "admin", "Browse filesystem (admin only)"))

	// ── Libraries ──
	merge("/libraries", "get", endpoint("List Libraries", "libraries", "Get all libraries"))
	merge("/libraries", "post", endpoint("Create Library", "libraries", "Create new library"))
	merge("/libraries/{id}", "get", endpoint("Get Library", "libraries", "Get library details"))
	merge("/libraries/{id}", "put", endpoint("Update Library", "libraries", "Update library"))
	merge("/libraries/{id}", "delete", endpoint("Delete Library", "libraries", "Delete library"))
	merge("/libraries/{id}/scan", "post", endpoint("Scan Library", "libraries", "Trigger library scan"))
	merge("/libraries/{id}/auto-match", "post", endpoint("Auto Match Library", "libraries", "Run auto-match on library"))
	merge("/libraries/{id}/refresh-metadata", "post", endpoint("Refresh Metadata", "libraries", "Refresh library metadata"))
	merge("/libraries/{id}/phash", "post", endpoint("Generate pHash", "libraries", "Generate perceptual hashes"))
	merge("/libraries/{id}/filters", "get", endpoint("Library Filters", "libraries", "Get available filters for library"))
	merge("/libraries/{id}/detect-segments", "post", endpoint("Detect Segments", "libraries", "Trigger segment detection"))

	// ── TV Shows ──
	merge("/libraries/{id}/shows", "get", endpoint("List Library Shows", "media", "List TV shows in library"))
	merge("/libraries/{id}/missing-episodes", "get", endpoint("Missing Episodes", "media", "Get missing episodes"))
	merge("/tv/shows/{id}", "get", endpoint("Get Show", "media", "Get TV show details"))
	merge("/tv/shows/{id}/seasons", "get", endpoint("List Show Seasons", "media", "List seasons for show"))
	merge("/tv/seasons/{id}/episodes", "get", endpoint("List Season Episodes", "media", "List episodes in season"))
	merge("/tv/seasons/{id}/missing", "get", endpoint("Season Missing Episodes", "media", "Get missing episodes for season"))

	// ── Media ──
	merge("/libraries/{id}/media", "get", endpoint("List Media", "media", "List media items in library"))
	merge("/libraries/{id}/media/index", "get", endpoint("Media Letter Index", "media", "Get media index by letter"))
	merge("/media/bulk", "put", endpoint("Bulk Update Media", "media", "Bulk update media items"))
	merge("/media/bulk-action", "post", endpoint("Bulk Action", "media", "Execute bulk action on media"))
	merge("/media/{id}", "get", endpoint("Get Media", "media", "Get full media item details"))
	merge("/media/{id}", "put", endpoint("Update Media", "media", "Update media item"))
	merge("/media/{id}/reset", "post", endpoint("Reset Media Lock", "media", "Reset media lock"))
	merge("/media/{id}/edition", "get", endpoint("Get Media Edition", "media", "Get edition for media"))
	merge("/media/{id}/editions", "get", endpoint("Get Media Editions", "media", "Get all editions for media"))
	merge("/media/{id}/edition-parent", "post", endpoint("Set Edition Parent", "media", "Set edition parent"))
	merge("/media/{id}/edition-parent", "delete", endpoint("Remove Edition Parent", "media", "Remove edition parent"))
	merge("/media/search", "get", endpoint("Search Media", "media", "Search media items"))
	merge("/media/{id}/identify", "post", endpoint("Identify Media", "media", "Trigger media identification"))
	merge("/media/{id}/apply-meta", "post", endpoint("Apply Metadata", "media", "Apply metadata to media"))
	merge("/media/{id}/locked-fields", "get", endpoint("Get Locked Fields", "media", "Get locked metadata fields"))
	merge("/media/{id}/locked-fields", "put", endpoint("Update Locked Fields", "media", "Update locked fields"))
	merge("/media/{id}/cast", "get", endpoint("Get Media Cast", "media", "Get cast/performers for media"))
	merge("/media/{id}/performers", "post", endpoint("Link Performer", "media", "Link performer to media"))
	merge("/media/{id}/performers/{performerId}", "delete", endpoint("Unlink Performer", "media", "Unlink performer from media"))
	merge("/media/{id}/tags", "get", endpoint("Get Media Tags", "media", "Get tags for media"))
	merge("/media/{id}/tags", "post", endpoint("Assign Tags", "media", "Assign tags to media"))
	merge("/media/{id}/tags/{tagId}", "delete", endpoint("Remove Tag", "media", "Remove tag from media"))
	merge("/media/{id}/studios", "post", endpoint("Link Studio", "media", "Link studio to media"))
	merge("/media/{id}/studios/{studioId}", "delete", endpoint("Unlink Studio", "media", "Unlink studio from media"))
	merge("/media/{id}/series", "get", endpoint("Get Media Series", "media", "Get series for media"))
	merge("/media/{id}/extras", "get", endpoint("Get Media Extras", "media", "Get extras for media"))
	merge("/media/{id}/lyrics", "get", endpoint("Get Lyrics", "media", "Get lyrics for media"))
	merge("/media/{id}/anime-info", "get", endpoint("Get Anime Info", "media", "Get anime metadata"))
	merge("/media/{id}/anime-info", "put", endpoint("Update Anime Info", "media", "Update anime metadata"))
	merge("/media/{id}/reading-progress", "get", endpoint("Get Reading Progress", "media", "Get comics/eBooks reading progress"))
	merge("/media/{id}/reading-progress", "put", endpoint("Update Reading Progress", "media", "Update reading progress"))
	merge("/media/{id}/markers", "get", endpoint("Get Markers", "media", "Get scene markers"))
	merge("/media/{id}/markers", "post", endpoint("Create Marker", "media", "Create scene marker"))
	merge("/media/{id}/duplicates", "get", endpoint("Get Media Duplicates", "media", "Get duplicates for media"))
	merge("/media/{id}/segments", "get", endpoint("Get Segments", "segments", "Get skip segments for media"))
	merge("/media/{id}/segments", "post", endpoint("Upsert Segment", "segments", "Create or update segment"))
	merge("/media/{mediaId}/segments/{type}", "delete", endpoint("Delete Segment", "segments", "Delete segment by type"))
	merge("/media/{id}/rating", "post", endpoint("Rate Media", "engagement", "Submit user rating"))
	merge("/media/{id}/rating", "get", endpoint("Get User Rating", "engagement", "Get user rating for media"))
	merge("/media/{id}/rating", "delete", endpoint("Delete Rating", "engagement", "Remove user rating"))
	merge("/media/{id}/community-rating", "get", endpoint("Get Community Rating", "engagement", "Get community rating"))

	// ── Streaming ──
	merge("/stream/{mediaId}/info", "get", endpoint("Stream Info", "streaming", "Get stream metadata and available qualities"))
	merge("/stream/{mediaId}/master.m3u8", "get", endpoint("HLS Master", "streaming", "Get HLS master playlist"))
	merge("/stream/{mediaId}/{quality}/{segment}", "get", endpoint("HLS Segment", "streaming", "Get HLS segment"))
	merge("/stream/{mediaId}/direct", "get", endpoint("Direct Stream", "streaming", "Stream media directly or via remux"))
	merge("/stream/{mediaId}/subtitles/{id}", "get", endpoint("Stream Subtitle", "streaming", "Get subtitle content"))
	merge("/stream/{mediaId}/manifest.mpd", "get", endpoint("DASH Manifest", "streaming", "Get DASH manifest"))

	// ── Editions ──
	merge("/editions", "get", endpoint("List Editions", "media", "List edition groups"))
	merge("/editions", "post", endpoint("Create Edition", "media", "Create edition group"))
	merge("/editions/{id}", "get", endpoint("Get Edition", "media", "Get edition details"))
	merge("/editions/{id}", "put", endpoint("Update Edition", "media", "Update edition"))
	merge("/editions/{id}", "delete", endpoint("Delete Edition", "media", "Delete edition"))
	merge("/editions/{id}/items", "post", endpoint("Add Edition Item", "media", "Add item to edition"))
	merge("/editions/{id}/items/{itemId}", "delete", endpoint("Remove Edition Item", "media", "Remove item from edition"))

	// ── Sisters ──
	merge("/sisters", "get", endpoint("List Sisters", "media", "List sister groups"))
	merge("/sisters", "post", endpoint("Create Sister", "media", "Create sister group"))
	merge("/sisters/{id}", "get", endpoint("Get Sister", "media", "Get sister group details"))
	merge("/sisters/{id}/items", "post", endpoint("Add Sister Item", "media", "Add item to sister group"))
	merge("/sisters/{id}/items/{itemId}", "delete", endpoint("Remove Sister Item", "media", "Remove item from sister group"))
	merge("/sisters/{id}", "delete", endpoint("Delete Sister", "media", "Delete sister group"))

	// ── Collections ──
	merge("/collections", "get", endpoint("List Collections", "media", "List collections"))
	merge("/collections", "post", endpoint("Create Collection", "media", "Create collection"))
	merge("/collections/templates", "post", endpoint("Create Collection Templates", "media", "Create from templates"))
	merge("/collections/{id}", "get", endpoint("Get Collection", "media", "Get collection details"))
	merge("/collections/{id}", "put", endpoint("Update Collection", "media", "Update collection"))
	merge("/collections/{id}", "delete", endpoint("Delete Collection", "media", "Delete collection"))
	merge("/collections/{id}/evaluate", "get", endpoint("Evaluate Smart Collection", "media", "Evaluate smart collection rules"))
	merge("/collections/{id}/stats", "get", endpoint("Get Collection Stats", "media", "Get collection statistics"))
	merge("/collections/{id}/children", "get", endpoint("List Collection Children", "media", "List child collections"))
	merge("/collections/{id}/items", "post", endpoint("Add Collection Item", "media", "Add item to collection"))
	merge("/collections/{id}/items/bulk", "post", endpoint("Bulk Add Collection Items", "media", "Bulk add items"))
	merge("/collections/{id}/items/{itemId}", "delete", endpoint("Remove Collection Item", "media", "Remove item from collection"))

	// ── Series (Movie) ──
	merge("/series", "get", endpoint("List Series", "media", "List movie series"))
	merge("/series", "post", endpoint("Create Series", "media", "Create movie series"))
	merge("/series/{id}", "get", endpoint("Get Series", "media", "Get series details"))
	merge("/series/{id}", "put", endpoint("Update Series", "media", "Update series"))
	merge("/series/{id}", "delete", endpoint("Delete Series", "media", "Delete series"))
	merge("/series/{id}/items", "post", endpoint("Add Series Item", "media", "Add item to series"))
	merge("/series/{id}/items/{itemId}", "delete", endpoint("Remove Series Item", "media", "Remove item from series"))

	// ── Engagement (watch, recommendations, household) ──
	merge("/watch/{mediaId}/progress", "post", endpoint("Update Progress", "engagement", "Report playback progress"))
	merge("/watch/continue", "get", endpoint("Continue Watching", "engagement", "Get items in progress"))
	merge("/watch/on-deck", "get", endpoint("On Deck", "engagement", "Get on-deck items"))
	merge("/recommendations", "get", endpoint("Recommendations", "engagement", "Get recommendations"))
	merge("/recommendations/because-you-watched", "get", endpoint("Because You Watched", "engagement", "Get because-you-watched recommendations"))
	merge("/household/profiles", "get", endpoint("Household Profiles", "engagement", "List household profiles"))
	merge("/household/profiles", "post", endpoint("Create Sub Profile", "engagement", "Create sub profile"))
	merge("/household/switch", "post", endpoint("Switch Household", "engagement", "Switch active profile"))
	merge("/household/profiles/{id}", "put", endpoint("Update Sub Profile", "engagement", "Update sub profile"))
	merge("/household/profiles/{id}", "delete", endpoint("Delete Sub Profile", "engagement", "Delete sub profile"))

	// ── Performers ──
	merge("/performers", "get", endpoint("List Performers", "media", "List performers"))
	merge("/performers", "post", endpoint("Create Performer", "media", "Create performer"))
	merge("/performers/{id}", "get", endpoint("Get Performer", "media", "Get performer details"))
	merge("/performers/{id}", "put", endpoint("Update Performer", "media", "Update performer"))
	merge("/performers/{id}", "delete", endpoint("Delete Performer", "media", "Delete performer"))
	merge("/performers/{id}/extended", "get", endpoint("Get Performer Extended", "media", "Get extended performer metadata"))
	merge("/performers/{id}/extended", "put", endpoint("Update Performer Extended", "media", "Update extended performer metadata"))

	// ── Tags ──
	merge("/tags", "get", endpoint("List Tags", "media", "List tags"))
	merge("/tags", "post", endpoint("Create Tag", "media", "Create tag"))
	merge("/tags/{id}", "put", endpoint("Update Tag", "media", "Update tag"))
	merge("/tags/{id}", "delete", endpoint("Delete Tag", "media", "Delete tag"))

	// ── Studios ──
	merge("/studios", "get", endpoint("List Studios", "media", "List studios"))
	merge("/studios", "post", endpoint("Create Studio", "media", "Create studio"))
	merge("/studios/{id}", "get", endpoint("Get Studio", "media", "Get studio details"))
	merge("/studios/{id}", "put", endpoint("Update Studio", "media", "Update studio"))
	merge("/studios/{id}", "delete", endpoint("Delete Studio", "media", "Delete studio"))

	// ── Duplicates ──
	merge("/duplicates", "get", endpoint("List Duplicates", "admin", "List duplicate media"))
	merge("/duplicates/resolve", "post", endpoint("Resolve Duplicate", "admin", "Resolve duplicate"))
	merge("/duplicates/count", "get", endpoint("Get Duplicate Count", "admin", "Get duplicate count"))

	// ── Sort order ──
	merge("/sort", "patch", endpoint("Update Sort Order", "admin", "Update global sort order"))

	// ── Jobs ──
	merge("/jobs", "get", endpoint("List Jobs", "admin", "List background jobs"))
	merge("/jobs/{id}", "get", endpoint("Get Job", "admin", "Get job details"))

	// ── Settings ──
	merge("/settings/playback", "get", endpoint("Get Playback Prefs", "profile", "Get playback preferences"))
	merge("/settings/playback", "put", endpoint("Update Playback Prefs", "profile", "Update playback preferences"))
	merge("/settings/display", "get", endpoint("Get Display Prefs", "profile", "Get display preferences"))
	merge("/settings/display", "put", endpoint("Update Display Prefs", "profile", "Update display preferences"))
	merge("/settings/skip", "get", endpoint("Get Skip Prefs", "profile", "Get skip preferences"))
	merge("/settings/skip", "put", endpoint("Update Skip Prefs", "profile", "Update skip preferences"))
	merge("/settings/system", "get", endpoint("Get System Settings", "admin", "Get system settings"))
	merge("/settings/system", "put", endpoint("Update System Settings", "admin", "Update system settings"))
	merge("/settings/home-layout", "get", endpoint("Get Home Layout", "profile", "Get home page layout"))
	merge("/settings/home-layout", "put", endpoint("Update Home Layout", "profile", "Update home page layout"))
	merge("/settings/api-keys", "get", endpoint("List API Keys", "api-keys", "Get user API keys"))
	merge("/settings/api-keys", "post", endpoint("Create API Key", "api-keys", "Create new API key"))
	merge("/settings/api-keys/{id}", "delete", endpoint("Delete API Key", "api-keys", "Delete API key"))

	// ── Analytics ──
	merge("/analytics/overview", "get", endpoint("Analytics Overview", "analytics", "Get analytics overview"))
	merge("/analytics/streams", "get", endpoint("Analytics Streams", "analytics", "Get stream analytics"))
	merge("/analytics/streams/breakdown", "get", endpoint("Stream Breakdown", "analytics", "Get stream breakdown"))
	merge("/analytics/watch-activity", "get", endpoint("Watch Activity", "analytics", "Get watch activity"))
	merge("/analytics/users/activity", "get", endpoint("User Activity", "analytics", "Get user activity"))
	merge("/analytics/transcodes", "get", endpoint("Transcode Analytics", "analytics", "Get transcode stats"))
	merge("/analytics/system", "get", endpoint("System Analytics", "analytics", "Get system metrics"))
	merge("/analytics/system/history", "get", endpoint("System History", "analytics", "Get system history"))
	merge("/analytics/storage", "get", endpoint("Storage Analytics", "analytics", "Get storage stats"))
	merge("/analytics/library-health", "get", endpoint("Library Health", "analytics", "Get library health"))
	merge("/analytics/trends", "get", endpoint("Analytics Trends", "analytics", "Get trend data"))

	// ── Discovery ──
	merge("/discover/trending", "get", endpoint("Trending", "discovery", "Get trending items from last 7 days"))
	merge("/discover/genre/{slug}", "get", endpoint("Genre Hub", "discovery", "Get genre page with items"))
	merge("/discover/decade/{year}", "get", endpoint("Decade Hub", "discovery", "Get decade page with items"))

	// ── Content Requests ──
	merge("/requests", "post", endpoint("Create Request", "requests", "Submit content request"))
	merge("/requests", "get", endpoint("List Requests", "requests", "List all requests (admin)"))
	merge("/requests/mine", "get", endpoint("My Requests", "requests", "Get user content requests"))
	merge("/requests/{id}", "put", endpoint("Resolve Request", "requests", "Resolve content request"))

	// ── Watchlist ──
	merge("/watchlist", "get", endpoint("Get Watchlist", "engagement", "Get user watchlist"))
	merge("/watchlist/{itemId}", "post", endpoint("Add to Watchlist", "engagement", "Add item to watchlist"))
	merge("/watchlist/{itemId}", "delete", endpoint("Remove from Watchlist", "engagement", "Remove item from watchlist"))
	merge("/watchlist/{itemId}/check", "get", endpoint("Check Watchlist", "engagement", "Check if item is in watchlist"))

	// ── Favorites ──
	merge("/favorites", "get", endpoint("Get Favorites", "engagement", "Get user favorites"))
	merge("/favorites/{itemId}", "post", endpoint("Toggle Favorite", "engagement", "Toggle favorite status"))
	merge("/favorites/{itemId}/check", "get", endpoint("Check Favorite", "engagement", "Check if item is favored"))

	// ── Playlists ──
	merge("/playlists", "get", endpoint("List Playlists", "playlists", "Get user playlists"))
	merge("/playlists", "post", endpoint("Create Playlist", "playlists", "Create playlist"))
	merge("/playlists/{id}", "put", endpoint("Update Playlist", "playlists", "Update playlist"))
	merge("/playlists/{id}", "delete", endpoint("Delete Playlist", "playlists", "Delete playlist"))
	merge("/playlists/{id}/items", "get", endpoint("Get Playlist Items", "playlists", "Get playlist items"))
	merge("/playlists/{id}/items", "post", endpoint("Add Playlist Item", "playlists", "Add item to playlist"))
	merge("/playlists/{id}/items/{itemId}", "delete", endpoint("Remove Playlist Item", "playlists", "Remove item from playlist"))
	merge("/playlists/{id}/reorder", "put", endpoint("Reorder Playlist", "playlists", "Reorder playlist items"))

	// ── Filters ──
	merge("/filters", "get", endpoint("List Saved Filters", "profile", "List saved filter presets"))
	merge("/filters", "post", endpoint("Create Saved Filter", "profile", "Create saved filter"))
	merge("/filters/{id}", "delete", endpoint("Delete Saved Filter", "profile", "Delete saved filter"))

	// ── Integrations ──
	merge("/trakt/device-code", "post", endpoint("Trakt Device Code", "integrations", "Start Trakt.tv device code auth"))
	merge("/trakt/activate", "post", endpoint("Trakt Activate", "integrations", "Complete Trakt.tv activation"))
	merge("/trakt/status", "get", endpoint("Trakt Status", "integrations", "Get Trakt.tv connection status"))
	merge("/trakt/disconnect", "delete", endpoint("Trakt Disconnect", "integrations", "Disconnect Trakt.tv"))
	merge("/trakt/scrobble", "post", endpoint("Trakt Scrobble", "integrations", "Scrobble playback to Trakt.tv"))
	merge("/lastfm/connect", "post", endpoint("Last.fm Connect", "integrations", "Connect Last.fm"))
	merge("/lastfm/status", "get", endpoint("Last.fm Status", "integrations", "Get Last.fm connection status"))
	merge("/lastfm/disconnect", "delete", endpoint("Last.fm Disconnect", "integrations", "Disconnect Last.fm"))
	merge("/lastfm/scrobble", "post", endpoint("Last.fm Scrobble", "integrations", "Scrobble to Last.fm"))

	// ── Webhooks ──
	merge("/webhooks/arr", "post", endpoint("Arr Webhook", "integrations", "Receive Sonarr/Radarr/Lidarr webhook"))
	merge("/admin/webhooks", "get", endpoint("List Webhook Secrets", "admin", "List webhook secrets"))
	merge("/admin/webhooks", "post", endpoint("Create Webhook Secret", "admin", "Create webhook secret"))
	merge("/admin/webhooks/{id}", "delete", endpoint("Delete Webhook Secret", "admin", "Delete webhook secret"))

	// ── Admin ──
	merge("/admin/backup", "post", endpoint("Create Backup", "admin", "Create database backup"))
	merge("/admin/backups", "get", endpoint("List Backups", "admin", "List backup history"))
	merge("/admin/backups/{id}/download", "get", endpoint("Download Backup", "admin", "Download backup file"))
	merge("/admin/import", "post", endpoint("Start Import", "admin", "Start Plex/Jellyfin import"))
	merge("/admin/imports", "get", endpoint("List Imports", "admin", "List import jobs"))
	merge("/admin/users/{id}/stream-limits", "get", endpoint("Get Stream Limits", "admin", "Get user stream limits"))
	merge("/admin/users/{id}/stream-limits", "put", endpoint("Update Stream Limits", "admin", "Update user stream limits"))

	// ── Sync (Watch Together) ──
	merge("/sync/create", "post", endpoint("Create Sync Session", "sync", "Create Watch Together session"))
	merge("/sync/join", "post", endpoint("Join Sync Session", "sync", "Join Watch Together session"))
	merge("/sync/{id}", "get", endpoint("Sync Info", "sync", "Get sync session info"))
	merge("/sync/{id}", "delete", endpoint("End Sync Session", "sync", "End Watch Together session"))
	merge("/sync/{id}/action", "post", endpoint("Sync Action", "sync", "Send sync action (play/pause/seek)"))
	merge("/sync/{id}/chat", "post", endpoint("Sync Chat", "sync", "Send chat message"))

	// ── Cinema ──
	merge("/cinema/pre-rolls", "get", endpoint("List Pre-Rolls", "admin", "List cinema pre-rolls"))
	merge("/cinema/pre-rolls", "post", endpoint("Create Pre-Roll", "admin", "Create pre-roll"))
	merge("/cinema/pre-rolls/{id}", "delete", endpoint("Delete Pre-Roll", "admin", "Delete pre-roll"))
	merge("/cinema/queue/{mediaId}", "get", endpoint("Cinema Queue", "streaming", "Get cinema mode queue"))

	// ── DLNA ──
	merge("/dlna/config", "get", endpoint("Get DLNA Config", "dlna", "Get DLNA configuration"))
	merge("/dlna/config", "put", endpoint("Update DLNA Config", "dlna", "Update DLNA configuration"))

	// ── Casting (Chromecast) ──
	merge("/cast/session", "post", endpoint("Create Cast Session", "casting", "Start Chromecast session"))
	merge("/cast/sessions", "get", endpoint("List Cast Sessions", "casting", "List active cast sessions"))
	merge("/cast/{id}", "delete", endpoint("End Cast Session", "casting", "End Chromecast session"))
	merge("/cast/session/{id}/command", "put", endpoint("Cast Command", "casting", "Send command to cast session (play/pause/seek)"))

	// ── Markers ──
	merge("/markers/{id}", "delete", endpoint("Delete Marker", "media", "Delete scene marker"))

	// ── Live TV ──
	merge("/livetv/tuners", "get", endpoint("List Tuners", "live-tv", "List tuners"))
	merge("/livetv/tuners", "post", endpoint("Create Tuner", "live-tv", "Create tuner"))
	merge("/livetv/tuners/{id}", "delete", endpoint("Delete Tuner", "live-tv", "Delete tuner"))
	merge("/livetv/epg", "get", endpoint("Get EPG", "live-tv", "Get electronic program guide"))
	merge("/livetv/recordings", "post", endpoint("Schedule Recording", "live-tv", "Schedule DVR recording"))
	merge("/livetv/recordings", "get", endpoint("List Recordings", "live-tv", "List DVR recordings"))

	// ── Notifications ──
	merge("/notifications/channels", "get", endpoint("List Notification Channels", "notifications", "List notification channels"))
	merge("/notifications/channels", "post", endpoint("Create Notification Channel", "notifications", "Create channel"))
	merge("/notifications/channels/{id}", "put", endpoint("Update Notification Channel", "notifications", "Update channel"))
	merge("/notifications/channels/{id}", "delete", endpoint("Delete Notification Channel", "notifications", "Delete channel"))
	merge("/notifications/channels/{id}/test", "post", endpoint("Test Notification Channel", "notifications", "Test channel"))
	merge("/notifications/alerts", "get", endpoint("List Alert Rules", "notifications", "List alert rules"))
	merge("/notifications/alerts", "post", endpoint("Create Alert Rule", "notifications", "Create alert rule"))
	merge("/notifications/alerts/{id}", "put", endpoint("Update Alert Rule", "notifications", "Update alert rule"))
	merge("/notifications/alerts/{id}", "delete", endpoint("Delete Alert Rule", "notifications", "Delete alert rule"))
	merge("/notifications/log", "get", endpoint("Get Alert Log", "notifications", "Get notification log"))

	return paths
}

func endpoint(summary, tag, description string) map[string]interface{} {
	return map[string]interface{}{
		"summary":     summary,
		"description": description,
		"tags":        []string{tag},
		"responses": map[string]interface{}{
			"200": map[string]interface{}{
				"description": "Success",
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"success": map[string]interface{}{"type": "boolean"},
								"data":    map[string]interface{}{"type": "object"},
								"error":   map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
		},
	}
}
