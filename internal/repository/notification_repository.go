package repository

import (
	"database/sql"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// ──────────────────── Channels ────────────────────

func (r *NotificationRepository) CreateChannel(ch *models.NotificationChannel) error {
	ch.ID = uuid.New()
	now := time.Now()
	ch.CreatedAt = now
	ch.UpdatedAt = now
	query := `INSERT INTO notification_channels (id, name, channel_type, webhook_url, is_enabled, events, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := r.db.Exec(query, ch.ID, ch.Name, ch.ChannelType, ch.WebhookURL, ch.IsEnabled, ch.Events, ch.CreatedAt, ch.UpdatedAt)
	return err
}

func (r *NotificationRepository) UpdateChannel(ch *models.NotificationChannel) error {
	ch.UpdatedAt = time.Now()
	query := `UPDATE notification_channels SET name=$1, channel_type=$2, webhook_url=$3, is_enabled=$4, events=$5, updated_at=$6 WHERE id=$7`
	_, err := r.db.Exec(query, ch.Name, ch.ChannelType, ch.WebhookURL, ch.IsEnabled, ch.Events, ch.UpdatedAt, ch.ID)
	return err
}

func (r *NotificationRepository) DeleteChannel(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM notification_channels WHERE id=$1`, id)
	return err
}

func (r *NotificationRepository) GetChannel(id uuid.UUID) (*models.NotificationChannel, error) {
	ch := &models.NotificationChannel{}
	err := r.db.QueryRow(`SELECT id, name, channel_type, webhook_url, is_enabled, events, created_at, updated_at
		FROM notification_channels WHERE id=$1`, id).
		Scan(&ch.ID, &ch.Name, &ch.ChannelType, &ch.WebhookURL, &ch.IsEnabled, &ch.Events, &ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (r *NotificationRepository) ListChannels() ([]*models.NotificationChannel, error) {
	rows, err := r.db.Query(`SELECT id, name, channel_type, webhook_url, is_enabled, events, created_at, updated_at
		FROM notification_channels ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.NotificationChannel
	for rows.Next() {
		ch := &models.NotificationChannel{}
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.ChannelType, &ch.WebhookURL, &ch.IsEnabled, &ch.Events, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, ch)
	}
	return results, rows.Err()
}

func (r *NotificationRepository) ListEnabledChannels() ([]*models.NotificationChannel, error) {
	rows, err := r.db.Query(`SELECT id, name, channel_type, webhook_url, is_enabled, events, created_at, updated_at
		FROM notification_channels WHERE is_enabled = true ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.NotificationChannel
	for rows.Next() {
		ch := &models.NotificationChannel{}
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.ChannelType, &ch.WebhookURL, &ch.IsEnabled, &ch.Events, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, ch)
	}
	return results, rows.Err()
}

// ──────────────────── Alert Rules ────────────────────

func (r *NotificationRepository) CreateAlertRule(rule *models.AlertRule) error {
	rule.ID = uuid.New()
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	query := `INSERT INTO alert_rules (id, name, condition_type, threshold, cooldown_minutes, channel_id, is_enabled, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.db.Exec(query, rule.ID, rule.Name, rule.ConditionType, rule.Threshold, rule.CooldownMinutes, rule.ChannelID, rule.IsEnabled, rule.CreatedAt, rule.UpdatedAt)
	return err
}

func (r *NotificationRepository) UpdateAlertRule(rule *models.AlertRule) error {
	rule.UpdatedAt = time.Now()
	query := `UPDATE alert_rules SET name=$1, condition_type=$2, threshold=$3, cooldown_minutes=$4, channel_id=$5, is_enabled=$6, updated_at=$7 WHERE id=$8`
	_, err := r.db.Exec(query, rule.Name, rule.ConditionType, rule.Threshold, rule.CooldownMinutes, rule.ChannelID, rule.IsEnabled, rule.UpdatedAt, rule.ID)
	return err
}

func (r *NotificationRepository) DeleteAlertRule(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM alert_rules WHERE id=$1`, id)
	return err
}

func (r *NotificationRepository) ListAlertRules() ([]*models.AlertRule, error) {
	rows, err := r.db.Query(`SELECT ar.id, ar.name, ar.condition_type, ar.threshold, ar.cooldown_minutes, ar.channel_id, ar.is_enabled, ar.last_triggered_at, ar.created_at, ar.updated_at, nc.name
		FROM alert_rules ar JOIN notification_channels nc ON nc.id = ar.channel_id
		ORDER BY ar.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.AlertRule
	for rows.Next() {
		rule := &models.AlertRule{}
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.ConditionType, &rule.Threshold, &rule.CooldownMinutes, &rule.ChannelID, &rule.IsEnabled, &rule.LastTriggeredAt, &rule.CreatedAt, &rule.UpdatedAt, &rule.ChannelName); err != nil {
			return nil, err
		}
		results = append(results, rule)
	}
	return results, rows.Err()
}

func (r *NotificationRepository) ListEnabledAlertRules() ([]*models.AlertRule, error) {
	rows, err := r.db.Query(`SELECT ar.id, ar.name, ar.condition_type, ar.threshold, ar.cooldown_minutes, ar.channel_id, ar.is_enabled, ar.last_triggered_at, ar.created_at, ar.updated_at, nc.name
		FROM alert_rules ar JOIN notification_channels nc ON nc.id = ar.channel_id
		WHERE ar.is_enabled = true AND nc.is_enabled = true
		ORDER BY ar.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.AlertRule
	for rows.Next() {
		rule := &models.AlertRule{}
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.ConditionType, &rule.Threshold, &rule.CooldownMinutes, &rule.ChannelID, &rule.IsEnabled, &rule.LastTriggeredAt, &rule.CreatedAt, &rule.UpdatedAt, &rule.ChannelName); err != nil {
			return nil, err
		}
		results = append(results, rule)
	}
	return results, rows.Err()
}

func (r *NotificationRepository) UpdateLastTriggered(ruleID uuid.UUID) error {
	_, err := r.db.Exec(`UPDATE alert_rules SET last_triggered_at = CURRENT_TIMESTAMP WHERE id = $1`, ruleID)
	return err
}

// ──────────────────── Alert Log ────────────────────

func (r *NotificationRepository) LogAlert(ruleID, channelID *uuid.UUID, message string, success bool, errorDetail *string) error {
	id := uuid.New()
	_, err := r.db.Exec(`INSERT INTO alert_log (id, rule_id, channel_id, message, success, error_detail, sent_at)
		VALUES ($1,$2,$3,$4,$5,$6,CURRENT_TIMESTAMP)`, id, ruleID, channelID, message, success, errorDetail)
	return err
}

func (r *NotificationRepository) GetRecentAlerts(limit int) ([]*models.AlertLogEntry, error) {
	query := `SELECT al.id, al.rule_id, al.channel_id, al.message, al.success, al.error_detail, al.sent_at,
		COALESCE(ar.name,''), COALESCE(nc.name,'')
		FROM alert_log al
		LEFT JOIN alert_rules ar ON ar.id = al.rule_id
		LEFT JOIN notification_channels nc ON nc.id = al.channel_id
		ORDER BY al.sent_at DESC LIMIT $1`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.AlertLogEntry
	for rows.Next() {
		e := &models.AlertLogEntry{}
		if err := rows.Scan(&e.ID, &e.RuleID, &e.ChannelID, &e.Message, &e.Success, &e.ErrorDetail, &e.SentAt, &e.RuleName, &e.ChannelName); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// TranscodeFailureCount returns number of failed transcodes in the last N minutes.
func (r *NotificationRepository) TranscodeFailureCount(minutes int) (int, error) {
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM transcode_history WHERE success = false AND started_at >= $1`, since).Scan(&count)
	return count, err
}

// StreamErrorCount returns number of stream errors in the last N minutes (sessions < 10s).
func (r *NotificationRepository) StreamErrorCount(minutes int) (int, error) {
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM stream_sessions WHERE started_at >= $1 AND is_active = false AND duration_seconds < 10`, since).Scan(&count)
	return count, err
}

// ── User Notification Preferences ──

// NotificationPreference represents a user's preference for an event type and channel.
type NotificationPreference struct {
	UserID    uuid.UUID
	EventType string
	ChannelID uuid.UUID
	Enabled   bool
}

// GetPreferencesForEvent returns all enabled user preferences for a given event type.
func (r *NotificationRepository) GetPreferencesForEvent(eventType string) ([]NotificationPreference, error) {
	rows, err := r.db.Query(`SELECT user_id, event_type, channel_id, enabled
		FROM user_notification_preferences WHERE event_type = $1 AND enabled = TRUE`, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []NotificationPreference
	for rows.Next() {
		var p NotificationPreference
		if err := rows.Scan(&p.UserID, &p.EventType, &p.ChannelID, &p.Enabled); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

// SetPreference creates or updates a user notification preference.
func (r *NotificationRepository) SetPreference(userID uuid.UUID, eventType string, channelID uuid.UUID, enabled bool) error {
	_, err := r.db.Exec(`INSERT INTO user_notification_preferences (user_id, event_type, channel_id, enabled)
		VALUES ($1, $2, $3, $4) ON CONFLICT (user_id, event_type, channel_id)
		DO UPDATE SET enabled = $4`, userID, eventType, channelID, enabled)
	return err
}

// GetUserPreferences returns all notification preferences for a user.
func (r *NotificationRepository) GetUserPreferences(userID uuid.UUID) ([]NotificationPreference, error) {
	rows, err := r.db.Query(`SELECT user_id, event_type, channel_id, enabled
		FROM user_notification_preferences WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []NotificationPreference
	for rows.Next() {
		var p NotificationPreference
		if err := rows.Scan(&p.UserID, &p.EventType, &p.ChannelID, &p.Enabled); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}
