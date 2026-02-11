package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type AnalyticsRepository struct {
	db *sql.DB
}

func NewAnalyticsRepository(db *sql.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// ──────────────────── Stream Sessions ────────────────────

func (r *AnalyticsRepository) CreateStreamSession(s *models.StreamSession) error {
	s.ID = uuid.New()
	s.StartedAt = time.Now()
	s.IsActive = true
	query := `INSERT INTO stream_sessions (id, user_id, media_item_id, playback_type, quality, codec, resolution, container, client_info, started_at, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	_, err := r.db.Exec(query, s.ID, s.UserID, s.MediaItemID, s.PlaybackType, s.Quality, s.Codec, s.Resolution, s.Container, s.ClientInfo, s.StartedAt, s.IsActive)
	return err
}

func (r *AnalyticsRepository) EndStreamSession(sessionID uuid.UUID, bytesServed int64, durationSeconds int) error {
	now := time.Now()
	query := `UPDATE stream_sessions SET ended_at=$1, is_active=false, bytes_served=$2, duration_seconds=$3 WHERE id=$4`
	_, err := r.db.Exec(query, now, bytesServed, durationSeconds, sessionID)
	return err
}

func (r *AnalyticsRepository) GetActiveStreams() ([]*models.StreamSession, error) {
	query := `SELECT ss.id, ss.user_id, ss.media_item_id, ss.playback_type, ss.quality, ss.codec, ss.resolution,
		ss.bytes_served, ss.duration_seconds, ss.started_at, ss.is_active,
		u.username, m.title
		FROM stream_sessions ss
		JOIN users u ON u.id = ss.user_id
		JOIN media_items m ON m.id = ss.media_item_id
		WHERE ss.is_active = true
		ORDER BY ss.started_at DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.StreamSession
	for rows.Next() {
		s := &models.StreamSession{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.MediaItemID, &s.PlaybackType, &s.Quality, &s.Codec, &s.Resolution,
			&s.BytesServed, &s.DurationSeconds, &s.StartedAt, &s.IsActive,
			&s.Username, &s.MediaTitle); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func (r *AnalyticsRepository) GetRecentStreams(limit int) ([]*models.StreamSession, error) {
	query := `SELECT ss.id, ss.user_id, ss.media_item_id, ss.playback_type, ss.quality, ss.resolution,
		ss.bytes_served, ss.duration_seconds, ss.started_at, ss.ended_at, ss.is_active,
		u.username, m.title
		FROM stream_sessions ss
		JOIN users u ON u.id = ss.user_id
		JOIN media_items m ON m.id = ss.media_item_id
		ORDER BY ss.started_at DESC LIMIT $1`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.StreamSession
	for rows.Next() {
		s := &models.StreamSession{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.MediaItemID, &s.PlaybackType, &s.Quality, &s.Resolution,
			&s.BytesServed, &s.DurationSeconds, &s.StartedAt, &s.EndedAt, &s.IsActive,
			&s.Username, &s.MediaTitle); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func (r *AnalyticsRepository) CountActiveStreams() (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM stream_sessions WHERE is_active = true`).Scan(&count)
	return count, err
}

// ──────────────────── Transcode History ────────────────────

func (r *AnalyticsRepository) RecordTranscode(rec *models.TranscodeHistoryRecord) error {
	rec.ID = uuid.New()
	now := time.Now()
	rec.CompletedAt = &now
	query := `INSERT INTO transcode_history (id, media_item_id, user_id, input_codec, output_codec, input_resolution, output_resolution, hw_accel, quality, duration_seconds, file_size_bytes, success, error_message, started_at, completed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`
	_, err := r.db.Exec(query, rec.ID, rec.MediaItemID, rec.UserID, rec.InputCodec, rec.OutputCodec,
		rec.InputResolution, rec.OutputResolution, rec.HWAccel, rec.Quality,
		rec.DurationSeconds, rec.FileSizeBytes, rec.Success, rec.ErrorMessage, rec.StartedAt, rec.CompletedAt)
	return err
}

func (r *AnalyticsRepository) GetTranscodeStats(days int) (*models.TranscodeStats, error) {
	since := time.Now().AddDate(0, 0, -days)
	stats := &models.TranscodeStats{}

	err := r.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN success THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN NOT success THEN 1 ELSE 0 END),0),
		COALESCE(AVG(duration_seconds),0),
		COALESCE(SUM(CASE WHEN hw_accel IS NOT NULL AND hw_accel != 'libx264' THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*),0) * 100, 0)
		FROM transcode_history WHERE started_at >= $1`, since).
		Scan(&stats.TotalTranscodes, &stats.SuccessCount, &stats.FailureCount, &stats.AvgDurationSecs, &stats.HWAccelPercent)
	if err != nil {
		return nil, err
	}
	if stats.TotalTranscodes > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalTranscodes) * 100
	}

	// Codec distribution
	rows, err := r.db.Query(`SELECT COALESCE(output_codec,'unknown'), COUNT(*) FROM transcode_history WHERE started_at >= $1 GROUP BY output_codec ORDER BY COUNT(*) DESC LIMIT 10`, since)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			nc := models.NameCount{}
			rows.Scan(&nc.Name, &nc.Count)
			stats.CodecDistribution = append(stats.CodecDistribution, nc)
		}
	}
	return stats, nil
}

func (r *AnalyticsRepository) GetRecentTranscodes(limit int) ([]*models.TranscodeHistoryRecord, error) {
	query := `SELECT th.id, th.media_item_id, th.user_id, th.input_codec, th.output_codec, th.output_resolution, th.hw_accel, th.quality, th.duration_seconds, th.success, th.error_message, th.started_at, th.completed_at, m.title
		FROM transcode_history th
		JOIN media_items m ON m.id = th.media_item_id
		ORDER BY th.started_at DESC LIMIT $1`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.TranscodeHistoryRecord
	for rows.Next() {
		rec := &models.TranscodeHistoryRecord{}
		if err := rows.Scan(&rec.ID, &rec.MediaItemID, &rec.UserID, &rec.InputCodec, &rec.OutputCodec, &rec.OutputResolution, &rec.HWAccel, &rec.Quality, &rec.DurationSeconds, &rec.Success, &rec.ErrorMessage, &rec.StartedAt, &rec.CompletedAt, &rec.MediaTitle); err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

// ──────────────────── System Metrics ────────────────────

func (r *AnalyticsRepository) RecordSystemMetrics(m *models.SystemMetric) error {
	m.ID = uuid.New()
	m.RecordedAt = time.Now()
	query := `INSERT INTO system_metrics (id, cpu_percent, memory_percent, memory_used_mb, gpu_encoder_percent, gpu_memory_percent, gpu_temp_celsius, disk_total_gb, disk_used_gb, disk_free_gb, active_streams, active_transcodes, recorded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	_, err := r.db.Exec(query, m.ID, m.CPUPercent, m.MemoryPercent, m.MemoryUsedMB,
		m.GPUEncoderPercent, m.GPUMemoryPercent, m.GPUTempCelsius,
		m.DiskTotalGB, m.DiskUsedGB, m.DiskFreeGB, m.ActiveStreams, m.ActiveTranscodes, m.RecordedAt)
	return err
}

func (r *AnalyticsRepository) GetLatestMetrics() (*models.SystemMetric, error) {
	m := &models.SystemMetric{}
	err := r.db.QueryRow(`SELECT id, cpu_percent, memory_percent, memory_used_mb, gpu_encoder_percent, gpu_memory_percent, gpu_temp_celsius, disk_total_gb, disk_used_gb, disk_free_gb, active_streams, active_transcodes, recorded_at
		FROM system_metrics ORDER BY recorded_at DESC LIMIT 1`).
		Scan(&m.ID, &m.CPUPercent, &m.MemoryPercent, &m.MemoryUsedMB, &m.GPUEncoderPercent, &m.GPUMemoryPercent, &m.GPUTempCelsius, &m.DiskTotalGB, &m.DiskUsedGB, &m.DiskFreeGB, &m.ActiveStreams, &m.ActiveTranscodes, &m.RecordedAt)
	if err == sql.ErrNoRows {
		return &models.SystemMetric{}, nil
	}
	return m, err
}

func (r *AnalyticsRepository) GetMetricsHistory(hours int) ([]*models.SystemMetric, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	query := `SELECT id, cpu_percent, memory_percent, memory_used_mb, gpu_encoder_percent, gpu_memory_percent, gpu_temp_celsius, disk_total_gb, disk_used_gb, disk_free_gb, active_streams, active_transcodes, recorded_at
		FROM system_metrics WHERE recorded_at >= $1 ORDER BY recorded_at ASC`
	rows, err := r.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.SystemMetric
	for rows.Next() {
		m := &models.SystemMetric{}
		if err := rows.Scan(&m.ID, &m.CPUPercent, &m.MemoryPercent, &m.MemoryUsedMB, &m.GPUEncoderPercent, &m.GPUMemoryPercent, &m.GPUTempCelsius, &m.DiskTotalGB, &m.DiskUsedGB, &m.DiskFreeGB, &m.ActiveStreams, &m.ActiveTranscodes, &m.RecordedAt); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

func (r *AnalyticsRepository) CleanupOldMetrics(maxDays int) error {
	cutoff := time.Now().AddDate(0, 0, -maxDays)
	_, err := r.db.Exec(`DELETE FROM system_metrics WHERE recorded_at < $1`, cutoff)
	return err
}

// ──────────────────── Analytics Overview ────────────────────

func (r *AnalyticsRepository) GetAnalyticsOverview() (*models.AnalyticsOverview, error) {
	o := &models.AnalyticsOverview{}
	today := time.Now().Truncate(24 * time.Hour)
	weekAgo := today.AddDate(0, 0, -7)

	// Active streams
	r.db.QueryRow(`SELECT COUNT(*) FROM stream_sessions WHERE is_active = true`).Scan(&o.ActiveStreams)

	// Today's plays
	r.db.QueryRow(`SELECT COUNT(*) FROM stream_sessions WHERE started_at >= $1`, today).Scan(&o.TotalPlaysToday)

	// Week's plays
	r.db.QueryRow(`SELECT COUNT(*) FROM stream_sessions WHERE started_at >= $1`, weekAgo).Scan(&o.TotalPlaysWeek)

	// Unique users today
	r.db.QueryRow(`SELECT COUNT(DISTINCT user_id) FROM stream_sessions WHERE started_at >= $1`, today).Scan(&o.UniqueUsersToday)

	// Bandwidth today
	r.db.QueryRow(`SELECT COALESCE(SUM(bytes_served),0) FROM stream_sessions WHERE started_at >= $1`, today).Scan(&o.BandwidthToday)

	// Library size
	r.db.QueryRow(`SELECT COUNT(*) FROM media_items`).Scan(&o.LibrarySize)

	// Storage
	r.db.QueryRow(`SELECT COALESCE(SUM(file_size),0) FROM media_items`).Scan(&o.StorageUsedBytes)

	// Transcodes today
	r.db.QueryRow(`SELECT COUNT(*) FROM stream_sessions WHERE started_at >= $1 AND playback_type = 'transcode'`, today).Scan(&o.TranscodesToday)

	// Direct plays today
	r.db.QueryRow(`SELECT COUNT(*) FROM stream_sessions WHERE started_at >= $1 AND playback_type = 'direct_play'`, today).Scan(&o.DirectPlaysToday)

	// Failures today
	r.db.QueryRow(`SELECT COUNT(*) FROM transcode_history WHERE started_at >= $1 AND success = false`, today).Scan(&o.FailuresToday)

	// Latest system metrics
	latest, err := r.GetLatestMetrics()
	if err == nil {
		o.CPUPercent = latest.CPUPercent
		o.MemoryPercent = latest.MemoryPercent
		o.DiskFreeGB = latest.DiskFreeGB
	}

	return o, nil
}

// ──────────────────── Stream Type Breakdown ────────────────────

func (r *AnalyticsRepository) GetStreamTypeBreakdown(days int) (*models.StreamTypeBreakdown, error) {
	since := time.Now().AddDate(0, 0, -days)
	b := &models.StreamTypeBreakdown{}
	r.db.QueryRow(`SELECT COALESCE(SUM(CASE WHEN playback_type='direct_play' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN playback_type='direct_stream' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN playback_type='transcode' THEN 1 ELSE 0 END),0),
		COUNT(*)
		FROM stream_sessions WHERE started_at >= $1`, since).
		Scan(&b.DirectPlays, &b.DirectStreams, &b.Transcodes, &b.Total)
	return b, nil
}

// ──────────────────── Watch Activity (Who Watched What) ────────────────────

func (r *AnalyticsRepository) GetWatchActivity(userID *uuid.UUID, from, to *time.Time, mediaType string, limit int) ([]*models.WatchActivityEntry, error) {
	query := `SELECT wh.user_id, u.username, wh.media_item_id, m.title, COALESCE(ss.playback_type,'direct_play'),
		wh.last_watched_at, COALESCE(wh.progress_seconds/60,0), wh.completed, m.poster_path
		FROM watch_history wh
		JOIN users u ON u.id = wh.user_id
		JOIN media_items m ON m.id = wh.media_item_id
		LEFT JOIN LATERAL (SELECT playback_type FROM stream_sessions WHERE user_id = wh.user_id AND media_item_id = wh.media_item_id ORDER BY started_at DESC LIMIT 1) ss ON true
		WHERE 1=1`
	args := []interface{}{}
	paramIdx := 1

	if userID != nil {
		query += fmt.Sprintf(" AND wh.user_id = $%d", paramIdx)
		args = append(args, *userID)
		paramIdx++
	}
	if from != nil {
		query += fmt.Sprintf(" AND wh.last_watched_at >= $%d", paramIdx)
		args = append(args, *from)
		paramIdx++
	}
	if to != nil {
		query += fmt.Sprintf(" AND wh.last_watched_at <= $%d", paramIdx)
		args = append(args, *to)
		paramIdx++
	}
	if mediaType != "" {
		query += fmt.Sprintf(" AND m.media_type = $%d", paramIdx)
		args = append(args, mediaType)
		paramIdx++
	}
	query += fmt.Sprintf(" ORDER BY wh.last_watched_at DESC LIMIT $%d", paramIdx)
	args = append(args, limit)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.WatchActivityEntry
	for rows.Next() {
		e := &models.WatchActivityEntry{}
		if err := rows.Scan(&e.UserID, &e.Username, &e.MediaItemID, &e.MediaTitle, &e.PlaybackType,
			&e.WatchedAt, &e.DurationMins, &e.Completed, &e.PosterPath); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// ──────────────────── User Activity Summary ────────────────────

func (r *AnalyticsRepository) GetUserActivitySummaries() ([]*models.UserActivitySummary, error) {
	query := `SELECT u.id, u.username,
		COUNT(wh.id) AS total_plays,
		COALESCE(SUM(wh.progress_seconds)/60, 0) AS total_mins,
		COALESCE(MAX(wh.last_watched_at), u.created_at) AS last_active
		FROM users u
		LEFT JOIN watch_history wh ON wh.user_id = u.id
		WHERE u.is_active = true
		GROUP BY u.id, u.username, u.created_at
		ORDER BY total_plays DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.UserActivitySummary
	for rows.Next() {
		s := &models.UserActivitySummary{}
		if err := rows.Scan(&s.UserID, &s.Username, &s.TotalPlays, &s.TotalWatchMins, &s.LastActive); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// ──────────────────── Storage Stats ────────────────────

func (r *AnalyticsRepository) GetStorageStats() ([]*models.StorageInfo, error) {
	query := `SELECT l.id, l.name, COUNT(m.id), COALESCE(SUM(m.file_size),0)
		FROM libraries l
		LEFT JOIN media_items m ON m.library_id = l.id
		GROUP BY l.id, l.name ORDER BY SUM(m.file_size) DESC NULLS LAST`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.StorageInfo
	for rows.Next() {
		si := &models.StorageInfo{}
		if err := rows.Scan(&si.LibraryID, &si.LibraryName, &si.FileCount, &si.TotalBytes); err != nil {
			return nil, err
		}
		results = append(results, si)
	}
	return results, rows.Err()
}

// ──────────────────── Library Health ────────────────────

func (r *AnalyticsRepository) GetLibraryHealth() ([]*models.LibraryHealthReport, error) {
	query := `SELECT l.id, l.name,
		COUNT(m.id) AS total,
		COALESCE(SUM(CASE WHEN m.description IS NULL OR m.description = '' THEN 1 ELSE 0 END),0) AS missing_meta,
		COALESCE(SUM(CASE WHEN m.poster_path IS NULL THEN 1 ELSE 0 END),0) AS missing_poster,
		COALESCE(SUM(CASE WHEN m.hdr_format IS NOT NULL AND m.hdr_format != '' THEN 1 ELSE 0 END),0) AS hdr_count,
		COALESCE(SUM(CASE WHEN m.audio_format IS NOT NULL AND LOWER(m.audio_format) LIKE '%atmos%' THEN 1 ELSE 0 END),0) AS atmos_count
		FROM libraries l
		LEFT JOIN media_items m ON m.library_id = l.id
		GROUP BY l.id, l.name`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []*models.LibraryHealthReport
	for rows.Next() {
		h := &models.LibraryHealthReport{}
		if err := rows.Scan(&h.LibraryID, &h.LibraryName, &h.TotalItems, &h.MissingMetadata, &h.MissingPoster, &h.HDRCount, &h.AtmosCount); err != nil {
			return nil, err
		}
		if h.TotalItems > 0 {
			h.MetadataPercent = float64(h.TotalItems-h.MissingMetadata) / float64(h.TotalItems) * 100
		}
		reports = append(reports, h)
	}

	// Fill codec + resolution distribution per library
	for _, h := range reports {
		codecRows, err := r.db.Query(`SELECT COALESCE(codec,'unknown'), COUNT(*) FROM media_items WHERE library_id=$1 GROUP BY codec ORDER BY COUNT(*) DESC LIMIT 10`, h.LibraryID)
		if err == nil {
			for codecRows.Next() {
				nc := models.NameCount{}
				codecRows.Scan(&nc.Name, &nc.Count)
				h.CodecDistribution = append(h.CodecDistribution, nc)
			}
			codecRows.Close()
		}
		resRows, err := r.db.Query(`SELECT COALESCE(resolution,'unknown'), COUNT(*) FROM media_items WHERE library_id=$1 GROUP BY resolution ORDER BY COUNT(*) DESC LIMIT 10`, h.LibraryID)
		if err == nil {
			for resRows.Next() {
				nc := models.NameCount{}
				resRows.Scan(&nc.Name, &nc.Count)
				h.ResolutionDist = append(h.ResolutionDist, nc)
			}
			resRows.Close()
		}
	}

	return reports, nil
}

// ──────────────────── Daily Trends ────────────────────

func (r *AnalyticsRepository) GetDailyTrends(from, to time.Time) ([]*models.DailyStat, error) {
	query := `SELECT id, stat_date, total_plays, unique_users, total_watch_minutes, total_bytes_served,
		transcodes, direct_plays, direct_streams, transcode_failures, new_media_added, library_size_total, storage_used_bytes
		FROM daily_stats WHERE stat_date >= $1 AND stat_date <= $2 ORDER BY stat_date ASC`
	rows, err := r.db.Query(query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []*models.DailyStat
	for rows.Next() {
		d := &models.DailyStat{}
		if err := rows.Scan(&d.ID, &d.StatDate, &d.TotalPlays, &d.UniqueUsers, &d.TotalWatchMinutes, &d.TotalBytesServed,
			&d.Transcodes, &d.DirectPlays, &d.DirectStreams, &d.TranscodeFailures, &d.NewMediaAdded, &d.LibrarySizeTotal, &d.StorageUsedBytes); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

func (r *AnalyticsRepository) UpsertDailyStat(d *models.DailyStat) error {
	d.ID = uuid.New()
	query := `INSERT INTO daily_stats (id, stat_date, total_plays, unique_users, total_watch_minutes, total_bytes_served, transcodes, direct_plays, direct_streams, transcode_failures, new_media_added, library_size_total, storage_used_bytes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (stat_date) DO UPDATE SET
			total_plays=$3, unique_users=$4, total_watch_minutes=$5, total_bytes_served=$6,
			transcodes=$7, direct_plays=$8, direct_streams=$9, transcode_failures=$10,
			new_media_added=$11, library_size_total=$12, storage_used_bytes=$13`
	_, err := r.db.Exec(query, d.ID, d.StatDate, d.TotalPlays, d.UniqueUsers, d.TotalWatchMinutes, d.TotalBytesServed,
		d.Transcodes, d.DirectPlays, d.DirectStreams, d.TranscodeFailures, d.NewMediaAdded, d.LibrarySizeTotal, d.StorageUsedBytes)
	return err
}

// ──────────────────── Rollup Helper ────────────────────

func (r *AnalyticsRepository) ComputeDailyRollup(date time.Time) (*models.DailyStat, error) {
	dayStart := date.Truncate(24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)

	d := &models.DailyStat{StatDate: dayStart}

	// Stream session stats
	r.db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT user_id), COALESCE(SUM(duration_seconds)/60,0), COALESCE(SUM(bytes_served),0),
		COALESCE(SUM(CASE WHEN playback_type='transcode' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN playback_type='direct_play' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN playback_type='direct_stream' THEN 1 ELSE 0 END),0)
		FROM stream_sessions WHERE started_at >= $1 AND started_at < $2`, dayStart, dayEnd).
		Scan(&d.TotalPlays, &d.UniqueUsers, &d.TotalWatchMinutes, &d.TotalBytesServed,
			&d.Transcodes, &d.DirectPlays, &d.DirectStreams)

	// Transcode failures
	r.db.QueryRow(`SELECT COUNT(*) FROM transcode_history WHERE started_at >= $1 AND started_at < $2 AND success = false`, dayStart, dayEnd).
		Scan(&d.TranscodeFailures)

	// New media added
	r.db.QueryRow(`SELECT COUNT(*) FROM media_items WHERE added_at >= $1 AND added_at < $2`, dayStart, dayEnd).
		Scan(&d.NewMediaAdded)

	// Library totals
	r.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(file_size),0) FROM media_items`).
		Scan(&d.LibrarySizeTotal, &d.StorageUsedBytes)

	return d, nil
}
