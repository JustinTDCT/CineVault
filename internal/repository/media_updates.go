package repository

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// MarkUnavailable sets a media item as unavailable (file deleted from disk).
func (r *MediaRepository) MarkUnavailable(filePath string) error {
	query := `DELETE FROM media_items WHERE file_path = $1`
	_, err := r.db.Exec(query, filePath)
	return err
}

// UpdateParentMediaID sets the parent_media_id for an extras item.
func (r *MediaRepository) UpdateParentMediaID(id, parentID uuid.UUID) error {
	query := `UPDATE media_items SET parent_media_id = $1 WHERE id = $2`
	_, err := r.db.Exec(query, parentID, id)
	return err
}

func (r *MediaRepository) UpdateMetadata(id uuid.UUID, title string, year *int, description *string, rating *float64, posterPath *string, contentRating *string) error {
	query := `UPDATE media_items SET title = $1, year = $2, description = $3, rating = $4,
		poster_path = $5, content_rating = $6,
		generated_poster = CASE WHEN ($5::text) IS NOT NULL THEN false ELSE generated_poster END,
		updated_at = CURRENT_TIMESTAMP WHERE id = $7`
	_, err := r.db.Exec(query, title, year, description, rating, posterPath, contentRating, id)
	return err
}

// UpdateMetadataWithLocks updates metadata fields but respects per-field locks.
// Locked fields retain their current database value.
func (r *MediaRepository) UpdateMetadataWithLocks(id uuid.UUID, title string, year *int, description *string, rating *float64, posterPath *string, contentRating *string, lockedFields pq.StringArray) error {
	query := `UPDATE media_items SET
		title = CASE WHEN $8::text[] IS NOT NULL AND ('title' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN title ELSE $1 END,
		year = CASE WHEN $8::text[] IS NOT NULL AND ('year' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN year ELSE $2 END,
		description = CASE WHEN $8::text[] IS NOT NULL AND ('description' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN description ELSE $3 END,
		rating = CASE WHEN $8::text[] IS NOT NULL AND ('rating' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN rating ELSE $4 END,
		poster_path = CASE WHEN $8::text[] IS NOT NULL AND ('poster_path' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN poster_path ELSE $5 END,
		content_rating = CASE WHEN $8::text[] IS NOT NULL AND ('content_rating' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN content_rating ELSE $6 END,
		generated_poster = CASE
			WHEN $8::text[] IS NOT NULL AND ('poster_path' = ANY($8::text[]) OR '*' = ANY($8::text[])) THEN generated_poster
			WHEN ($5::text) IS NOT NULL THEN false
			ELSE generated_poster END,
		updated_at = CURRENT_TIMESTAMP WHERE id = $7`
	_, err := r.db.Exec(query, title, year, description, rating, posterPath, contentRating, id, lockedFields)
	return err
}

func (r *MediaRepository) UpdateRatings(id uuid.UUID, imdbRating *float64, rtRating *int, audienceScore *int) error {
	query := `UPDATE media_items SET imdb_rating = $1, rt_rating = $2, audience_score = $3,
		updated_at = CURRENT_TIMESTAMP WHERE id = $4`
	_, err := r.db.Exec(query, imdbRating, rtRating, audienceScore, id)
	return err
}

// UpdateRatingsWithLocks updates rating fields but respects per-field locks.
func (r *MediaRepository) UpdateRatingsWithLocks(id uuid.UUID, imdbRating *float64, rtRating *int, audienceScore *int, lockedFields pq.StringArray) error {
	query := `UPDATE media_items SET
		imdb_rating = CASE WHEN $5::text[] IS NOT NULL AND ('imdb_rating' = ANY($5::text[]) OR '*' = ANY($5::text[])) THEN imdb_rating ELSE $1 END,
		rt_rating = CASE WHEN $5::text[] IS NOT NULL AND ('rt_rating' = ANY($5::text[]) OR '*' = ANY($5::text[])) THEN rt_rating ELSE $2 END,
		audience_score = CASE WHEN $5::text[] IS NOT NULL AND ('audience_score' = ANY($5::text[]) OR '*' = ANY($5::text[])) THEN audience_score ELSE $3 END,
		updated_at = CURRENT_TIMESTAMP WHERE id = $4`
	_, err := r.db.Exec(query, imdbRating, rtRating, audienceScore, id, lockedFields)
	return err
}

func (r *MediaRepository) UpdateMediaFields(id uuid.UUID, title string, sortTitle, originalTitle, description *string, year *int, releaseDate *string, rating *float64, editionType *string) error {
	query := `UPDATE media_items SET
		title = $1, sort_title = $2, original_title = $3, description = $4,
		year = $5, release_date = $6, rating = $7, edition_type = COALESCE($8, edition_type),
		metadata_locked = true, updated_at = CURRENT_TIMESTAMP
		WHERE id = $9`
	_, err := r.db.Exec(query, title, sortTitle, originalTitle, description, year, releaseDate, rating, editionType, id)
	return err
}

// ResetMetadataLock clears the metadata_locked flag so the next scan/auto-match can overwrite.
func (r *MediaRepository) ResetMetadataLock(id uuid.UUID) error {
	_, err := r.db.Exec(`UPDATE media_items SET metadata_locked = false, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	return err
}

func (r *MediaRepository) UpdateLastScanned(id uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE media_items SET last_scanned_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	return err
}

func (r *MediaRepository) IncrementPlayCount(id uuid.UUID) error {
	_, err := r.db.Exec(
		`UPDATE media_items SET play_count = COALESCE(play_count, 0) + 1, last_played_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id)
	return err
}

// UpdateFileHash sets the MD5 hash for a media item.
func (r *MediaRepository) UpdateFileHash(id uuid.UUID, hash string) error {
	_, err := r.db.Exec(`UPDATE media_items SET file_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, hash, id)
	return err
}

// UpdateLoudness stores the measured LUFS loudness and computed gain for a media item.
func (r *MediaRepository) UpdateLoudness(id uuid.UUID, lufs, gainDB float64) error {
	_, err := r.db.Exec(`UPDATE media_items SET loudness_lufs = $1, loudness_gain_db = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`,
		lufs, gainDB, id)
	return err
}

func (r *MediaRepository) UpdatePhash(id uuid.UUID, phash string) error {
	_, err := r.db.Exec(`UPDATE media_items SET phash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, phash, id)
	return err
}

// ClearStalePhashes nullifies phash values that don't match the expected length,
// indicating they were computed with an old/incompatible algorithm.
func (r *MediaRepository) ClearStalePhashes(libraryID uuid.UUID, expectedLen int) (int64, error) {
	res, err := r.db.Exec(`UPDATE media_items SET phash = NULL
		WHERE library_id = $1 AND phash IS NOT NULL AND LENGTH(phash) != $2`, libraryID, expectedLen)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdatePosterPath sets the poster image path for a media item.
func (r *MediaRepository) UpdatePosterPath(id uuid.UUID, posterPath string) error {
	_, err := r.db.Exec(`UPDATE media_items SET poster_path = $1, generated_poster = false, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, posterPath, id)
	return err
}

// UpdateBackdropPath sets the backdrop image path for a media item.
func (r *MediaRepository) UpdateBackdropPath(id uuid.UUID, backdropPath string) error {
	_, err := r.db.Exec(`UPDATE media_items SET backdrop_path = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, backdropPath, id)
	return err
}

// SetGeneratedPoster marks a media item's poster as generated from a video screenshot.
func (r *MediaRepository) SetGeneratedPoster(id uuid.UUID, generated bool) error {
	_, err := r.db.Exec(`UPDATE media_items SET generated_poster = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, generated, id)
	return err
}

// UpdatePreviewPath sets the preview clip path for a media item.
func (r *MediaRepository) UpdatePreviewPath(id uuid.UUID, previewPath string) error {
	_, err := r.db.Exec(`UPDATE media_items SET preview_path = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, previewPath, id)
	return err
}

// UpdateSpritePath sets the timeline thumbnail sprite sheet path for a media item.
func (r *MediaRepository) UpdateSpritePath(id uuid.UUID, spritePath string) error {
	_, err := r.db.Exec(`UPDATE media_items SET sprite_path = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, spritePath, id)
	return err
}

// UpdateExternalIDs stores the external source IDs JSON for a media item.
func (r *MediaRepository) UpdateExternalIDs(id uuid.UUID, externalIDsJSON string) error {
	_, err := r.db.Exec(`UPDATE media_items SET external_ids = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, externalIDsJSON, id)
	return err
}

// UpdateContentRating sets the content rating (e.g. PG-13, R) for a media item.
func (r *MediaRepository) UpdateContentRating(id uuid.UUID, contentRating string) error {
	_, err := r.db.Exec(`UPDATE media_items SET content_rating = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, contentRating, id)
	return err
}

// UpdateMetacriticScore sets the Metacritic score for a media item.
func (r *MediaRepository) UpdateMetacriticScore(id uuid.UUID, score int) error {
	_, err := r.db.Exec(`UPDATE media_items SET metacritic_score = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, score, id)
	return err
}

// UpdateContentRatingsJSON stores the full multi-country content ratings JSON.
func (r *MediaRepository) UpdateContentRatingsJSON(id uuid.UUID, ratingsJSON string) error {
	_, err := r.db.Exec(`UPDATE media_items SET content_ratings_json = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, ratingsJSON, id)
	return err
}

// UpdateField updates a single TEXT column by name (whitelist-checked).
func (r *MediaRepository) UpdateField(id uuid.UUID, field, value string) error {
	allowed := map[string]bool{
		"taglines_json": true, "trailers_json": true, "descriptions_json": true,
		"content_ratings_json": true, "keywords": true,
	}
	if !allowed[field] {
		return fmt.Errorf("field %q not allowed in UpdateField", field)
	}
	query := fmt.Sprintf(`UPDATE media_items SET %s = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, field)
	_, err := r.db.Exec(query, value, id)
	return err
}

// UpdateExtendedMetadata sets extended metadata fields (tagline, language, country, trailer, logo).
// Only non-nil fields are updated; nil values are left unchanged.
func (r *MediaRepository) UpdateExtendedMetadata(id uuid.UUID, tagline, originalLang, country, trailerURL, logoPath *string) error {
	return r.UpdateExtendedMetadataFull(id, &ExtendedMetadataUpdate{
		Tagline:          tagline,
		OriginalLanguage: originalLang,
		Country:          country,
		TrailerURL:       trailerURL,
		LogoPath:         logoPath,
	})
}

// UpdateExtendedMetadataFull sets all extended metadata fields from an update struct.
// Only non-nil fields are updated; nil values are left unchanged.
func (r *MediaRepository) UpdateExtendedMetadataFull(id uuid.UUID, u *ExtendedMetadataUpdate) error {
	setClauses := []string{}
	args := []interface{}{}
	idx := 1

	if u.Tagline != nil {
		setClauses = append(setClauses, fmt.Sprintf("tagline = $%d", idx))
		args = append(args, *u.Tagline)
		idx++
	}
	if u.OriginalLanguage != nil {
		setClauses = append(setClauses, fmt.Sprintf("original_language = $%d", idx))
		args = append(args, *u.OriginalLanguage)
		idx++
	}
	if u.Country != nil {
		setClauses = append(setClauses, fmt.Sprintf("country = $%d", idx))
		args = append(args, *u.Country)
		idx++
	}
	if u.TrailerURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("trailer_url = $%d", idx))
		args = append(args, *u.TrailerURL)
		idx++
	}
	if u.LogoPath != nil {
		setClauses = append(setClauses, fmt.Sprintf("logo_path = $%d", idx))
		args = append(args, *u.LogoPath)
		idx++
	}
	if u.OriginalTitle != nil {
		setClauses = append(setClauses, fmt.Sprintf("original_title = $%d", idx))
		args = append(args, *u.OriginalTitle)
		idx++
	}
	if u.SortTitle != nil {
		setClauses = append(setClauses, fmt.Sprintf("sort_title = $%d", idx))
		args = append(args, *u.SortTitle)
		idx++
	}
	if u.ReleaseDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("release_date = $%d", idx))
		args = append(args, *u.ReleaseDate)
		idx++
	}
	if u.BannerPath != nil {
		setClauses = append(setClauses, fmt.Sprintf("banner_path = $%d", idx))
		args = append(args, *u.BannerPath)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE media_items SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), idx)
	args = append(args, id)
	_, err := r.db.Exec(query, args...)
	return err
}

// UpdateDuplicateStatus sets the duplicate_status flag on a media item.
func (r *MediaRepository) UpdateDuplicateStatus(id uuid.UUID, status string) error {
	_, err := r.db.Exec(`UPDATE media_items SET duplicate_status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, status, id)
	return err
}

// ClearLibraryPreviews NULLs out preview_path for all items in a library and returns
// the old preview paths so the caller can delete the files on disk.
func (r *MediaRepository) ClearLibraryPreviews(libraryID uuid.UUID) ([]string, error) {
	rows, err := r.db.Query(`SELECT preview_path FROM media_items WHERE library_id = $1 AND preview_path IS NOT NULL AND preview_path != ''`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	_, err = r.db.Exec(`UPDATE media_items SET preview_path = NULL, updated_at = CURRENT_TIMESTAMP WHERE library_id = $1`, libraryID)
	return paths, err
}

// ClearItemMetadata resets all enriched metadata fields for a single item back to a
// clean state. Technical metadata (resolution, codec, duration, etc.) is preserved.
// The title is reset to the provided fileTitle (derived from the filename).
func (r *MediaRepository) ClearItemMetadata(id uuid.UUID, fileTitle string) error {
	query := `UPDATE media_items SET
		title = $1, sort_title = NULL, original_title = NULL, description = NULL,
		year = NULL, release_date = NULL, rating = NULL,
		poster_path = NULL, thumbnail_path = NULL, backdrop_path = NULL,
		generated_poster = false, imdb_rating = NULL, rt_rating = NULL, audience_score = NULL,
		content_rating = NULL, external_ids = NULL,
		updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`
	_, err := r.db.Exec(query, fileTitle, id)
	return err
}

// ClearItemMetadataWithLocks resets enriched metadata fields but preserves any
// per-field locked values. Extended metadata fields (tagline, language, etc.)
// are also cleared unless locked.
func (r *MediaRepository) ClearItemMetadataWithLocks(id uuid.UUID, fileTitle string, lockedFields pq.StringArray) error {
	query := `UPDATE media_items SET
		title = CASE WHEN $3::text[] IS NOT NULL AND ('title' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN title ELSE $1 END,
		sort_title = CASE WHEN $3::text[] IS NOT NULL AND ('title' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN sort_title ELSE NULL END,
		original_title = CASE WHEN $3::text[] IS NOT NULL AND ('title' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN original_title ELSE NULL END,
		description = CASE WHEN $3::text[] IS NOT NULL AND ('description' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN description ELSE NULL END,
		year = CASE WHEN $3::text[] IS NOT NULL AND ('year' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN year ELSE NULL END,
		release_date = CASE WHEN $3::text[] IS NOT NULL AND ('year' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN release_date ELSE NULL END,
		rating = CASE WHEN $3::text[] IS NOT NULL AND ('rating' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN rating ELSE NULL END,
		poster_path = CASE WHEN $3::text[] IS NOT NULL AND ('poster_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN poster_path ELSE NULL END,
		thumbnail_path = CASE WHEN $3::text[] IS NOT NULL AND ('poster_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN thumbnail_path ELSE NULL END,
		backdrop_path = CASE WHEN $3::text[] IS NOT NULL AND ('backdrop_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN backdrop_path ELSE NULL END,
		generated_poster = CASE WHEN $3::text[] IS NOT NULL AND ('poster_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN generated_poster ELSE false END,
		imdb_rating = CASE WHEN $3::text[] IS NOT NULL AND ('imdb_rating' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN imdb_rating ELSE NULL END,
		rt_rating = CASE WHEN $3::text[] IS NOT NULL AND ('rt_rating' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN rt_rating ELSE NULL END,
		audience_score = CASE WHEN $3::text[] IS NOT NULL AND ('audience_score' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN audience_score ELSE NULL END,
		content_rating = CASE WHEN $3::text[] IS NOT NULL AND ('content_rating' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN content_rating ELSE NULL END,
		external_ids = CASE WHEN $3::text[] IS NOT NULL AND ('external_ids' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN external_ids ELSE NULL END,
		tagline = CASE WHEN $3::text[] IS NOT NULL AND ('tagline' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN tagline ELSE NULL END,
		original_language = CASE WHEN $3::text[] IS NOT NULL AND ('original_language' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN original_language ELSE NULL END,
		country = CASE WHEN $3::text[] IS NOT NULL AND ('country' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN country ELSE NULL END,
		trailer_url = CASE WHEN $3::text[] IS NOT NULL AND ('trailer_url' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN trailer_url ELSE NULL END,
		logo_path = CASE WHEN $3::text[] IS NOT NULL AND ('logo_path' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN logo_path ELSE NULL END,
		custom_notes = CASE WHEN $3::text[] IS NOT NULL AND ('custom_notes' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN custom_notes ELSE NULL END,
		custom_tags = CASE WHEN $3::text[] IS NOT NULL AND ('custom_tags' = ANY($3::text[]) OR '*' = ANY($3::text[])) THEN custom_tags ELSE '{}' END,
		updated_at = CURRENT_TIMESTAMP
		WHERE id = $2`
	_, err := r.db.Exec(query, fileTitle, id, lockedFields)
	return err
}

// RemoveAllMediaTags removes all tag links for a media item.
func (r *MediaRepository) RemoveAllMediaTags(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM media_tags WHERE media_item_id = $1`, id)
	return err
}

// RemoveAllMediaPerformers removes all performer links for a media item.
func (r *MediaRepository) RemoveAllMediaPerformers(id uuid.UUID) error {
	_, err := r.db.Exec(`DELETE FROM media_performers WHERE media_item_id = $1`, id)
	return err
}

// UpdateTechnicalMetadata sets source type, HDR format, and dynamic range for a media item.
func (r *MediaRepository) UpdateTechnicalMetadata(id uuid.UUID, sourceType, hdrFormat, dynamicRange *string) error {
	setClauses := []string{}
	args := []interface{}{}
	idx := 1

	if sourceType != nil {
		setClauses = append(setClauses, fmt.Sprintf("source_type = $%d", idx))
		args = append(args, *sourceType)
		idx++
	}
	if hdrFormat != nil {
		setClauses = append(setClauses, fmt.Sprintf("hdr_format = $%d", idx))
		args = append(args, *hdrFormat)
		idx++
	}
	if dynamicRange != nil {
		setClauses = append(setClauses, fmt.Sprintf("dynamic_range = $%d", idx))
		args = append(args, *dynamicRange)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE media_items SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), idx)
	args = append(args, id)
	_, err := r.db.Exec(query, args...)
	return err
}

// UpdateCustomNotes sets the custom_notes text for a media item.
func (r *MediaRepository) UpdateCustomNotes(id uuid.UUID, notes *string) error {
	_, err := r.db.Exec(`UPDATE media_items SET custom_notes = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, notes, id)
	return err
}

// UpdateCustomTags sets the custom_tags JSON for a media item.
func (r *MediaRepository) UpdateCustomTags(id uuid.UUID, tagsJSON string) error {
	_, err := r.db.Exec(`UPDATE media_items SET custom_tags = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, tagsJSON, id)
	return err
}

// BulkUpdateFields updates specific fields across multiple media items in a single transaction.
// The fields map should contain column names mapped to their new values.
func (r *MediaRepository) BulkUpdateFields(ids []uuid.UUID, fields map[string]interface{}) error {
	if len(ids) == 0 || len(fields) == 0 {
		return nil
	}

	// Whitelist of allowed columns
	allowed := map[string]bool{
		"rating": true, "edition_type": true, "custom_notes": true, "custom_tags": true,
	}

	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	for col, val := range fields {
		if !allowed[col] {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "metadata_locked = true", "updated_at = CURRENT_TIMESTAMP")

	// Build ID placeholders
	idPlaceholders := make([]string, len(ids))
	for i, id := range ids {
		idPlaceholders[i] = fmt.Sprintf("$%d", argIdx)
		args = append(args, id)
		argIdx++
	}

	query := fmt.Sprintf("UPDATE media_items SET %s WHERE id IN (%s)",
		strings.Join(setClauses, ", "),
		strings.Join(idPlaceholders, ", "))

	_, err := r.db.Exec(query, args...)
	return err
}
