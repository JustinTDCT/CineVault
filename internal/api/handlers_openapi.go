package api

import (
	"encoding/json"
	"net/http"
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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(spec)
}

func getAppVersion() string {
	// Read from version.json at runtime
	return "0.58.00"
}

func buildAPIPaths() map[string]interface{} {
	paths := map[string]interface{}{
		"/auth/login": map[string]interface{}{
			"post": endpoint("Login", "auth", "Authenticate with username and password"),
		},
		"/auth/2fa/setup": map[string]interface{}{
			"post": endpoint("Setup 2FA", "auth", "Generate TOTP secret and QR code URL"),
		},
		"/auth/2fa/verify": map[string]interface{}{
			"post": endpoint("Verify 2FA", "auth", "Verify TOTP code and enable 2FA"),
		},
		"/libraries": map[string]interface{}{
			"get": endpoint("List Libraries", "libraries", "Get all libraries"),
		},
		"/libraries/{id}/media": map[string]interface{}{
			"get": endpoint("Get Library Media", "libraries", "Get media items in a library"),
		},
		"/media/{id}": map[string]interface{}{
			"get": endpoint("Get Media Detail", "media", "Get full media item details"),
		},
		"/watch/{mediaId}/progress": map[string]interface{}{
			"post": endpoint("Update Progress", "watch", "Report playback progress"),
		},
		"/watch/continue": map[string]interface{}{
			"get": endpoint("Continue Watching", "watch", "Get items in progress"),
		},
		"/stream/{mediaId}/info": map[string]interface{}{
			"get": endpoint("Stream Info", "streaming", "Get stream metadata and available qualities"),
		},
		"/stream/{mediaId}/direct": map[string]interface{}{
			"get": endpoint("Direct Stream", "streaming", "Stream media directly or via remux"),
		},
		"/stream/{mediaId}/master.m3u8": map[string]interface{}{
			"get": endpoint("HLS Master", "streaming", "Get HLS master playlist"),
		},
		"/watchlist": map[string]interface{}{
			"get": endpoint("Get Watchlist", "engagement", "Get user watchlist"),
		},
		"/watchlist/{itemId}": map[string]interface{}{
			"post":   endpoint("Add to Watchlist", "engagement", "Add item to watchlist"),
			"delete": endpoint("Remove from Watchlist", "engagement", "Remove item from watchlist"),
		},
		"/favorites": map[string]interface{}{
			"get": endpoint("Get Favorites", "engagement", "Get user favorites"),
		},
		"/ratings/{itemId}": map[string]interface{}{
			"post":   endpoint("Rate Media", "engagement", "Submit user rating"),
			"get":    endpoint("Get Rating", "engagement", "Get user rating for item"),
			"delete": endpoint("Delete Rating", "engagement", "Remove user rating"),
		},
		"/playlists": map[string]interface{}{
			"get":  endpoint("List Playlists", "playlists", "Get user playlists"),
			"post": endpoint("Create Playlist", "playlists", "Create a new playlist"),
		},
		"/profile/stats": map[string]interface{}{
			"get": endpoint("Profile Stats", "profile", "Get user watch statistics"),
		},
		"/profile/wrapped/{year}": map[string]interface{}{
			"get": endpoint("Year Wrapped", "profile", "Get year-in-review summary"),
		},
		"/discover/trending": map[string]interface{}{
			"get": endpoint("Trending", "discovery", "Get trending items from last 7 days"),
		},
		"/discover/genre/{slug}": map[string]interface{}{
			"get": endpoint("Genre Hub", "discovery", "Get genre page with items"),
		},
		"/discover/decade/{year}": map[string]interface{}{
			"get": endpoint("Decade Hub", "discovery", "Get decade page with items"),
		},
		"/requests": map[string]interface{}{
			"post": endpoint("Create Request", "requests", "Submit content request"),
		},
		"/requests/mine": map[string]interface{}{
			"get": endpoint("My Requests", "requests", "Get user content requests"),
		},
		"/trakt/device-code": map[string]interface{}{
			"post": endpoint("Trakt Device Code", "integrations", "Start Trakt.tv device code auth flow"),
		},
		"/trakt/status": map[string]interface{}{
			"get": endpoint("Trakt Status", "integrations", "Get Trakt.tv connection status"),
		},
		"/trakt/scrobble": map[string]interface{}{
			"post": endpoint("Trakt Scrobble", "integrations", "Scrobble playback to Trakt.tv"),
		},
		"/lastfm/status": map[string]interface{}{
			"get": endpoint("Last.fm Status", "integrations", "Get Last.fm connection status"),
		},
		"/webhooks/arr": map[string]interface{}{
			"post": endpoint("Arr Webhook", "webhooks", "Receive Sonarr/Radarr/Lidarr webhook"),
		},
		"/settings/api-keys": map[string]interface{}{
			"get":  endpoint("List API Keys", "api-keys", "Get user API keys"),
			"post": endpoint("Create API Key", "api-keys", "Create a new API key"),
		},
		"/admin/backup": map[string]interface{}{
			"post": endpoint("Create Backup", "admin", "Create database backup"),
		},
		"/admin/backups": map[string]interface{}{
			"get": endpoint("List Backups", "admin", "Get backup history"),
		},
	}
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
