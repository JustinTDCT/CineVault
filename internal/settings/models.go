package settings

import "time"

const (
	KeyRegion             = "region"
	KeyDuplicatesEnabled  = "duplicates_enabled"
	KeySkipIntro          = "skip_app_intro"
	KeySilentIntro        = "silent_app_intro"
	KeyDefaultQuality     = "default_video_quality"
	KeyAutoSkipIntro      = "auto_skip_intro"
	KeyAutoSkipCredits    = "auto_skip_credits"
	KeyAutoSkipRecaps     = "auto_skip_recaps"
	KeyTranscoderType     = "transcoder_type"
	KeyMaxTranscodes      = "max_transcodes"
	KeyCacheServerEnabled = "cache_server_enabled"
	KeyCacheServerURL     = "cache_server_url"
	KeyCacheServerKey     = "cache_server_api_key"
	KeyAutomatchMinPct    = "automatch_min_pct"
	KeyManualMinPct       = "manual_min_pct"
	KeyManualMaxResults   = "manual_max_results"
	KeyHTTPSEnabled       = "https_enabled"
	KeyHTTPSCert          = "https_cert_path"
	KeyHTTPSKey           = "https_key_path"
	KeyFastUserSwitch     = "fast_user_switch"
	KeyMinPINLength       = "min_pin_length"
	KeyPasswordMinLength  = "password_min_length"
	KeyPasswordComplexity = "password_complexity"
)

type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
