package repository

import (
	"database/sql"
	"fmt"

	"github.com/JustinTDCT/CineVault/internal/models"
	"github.com/google/uuid"
)

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Create(job *models.JobRecord) error {
	query := `INSERT INTO job_history (id, job_type, status, progress, started_by)
		VALUES ($1, $2, $3, $4, $5) RETURNING started_at, updated_at`
	return r.db.QueryRow(query, job.ID, job.JobType, job.Status, job.Progress, job.StartedBy).
		Scan(&job.StartedAt, &job.UpdatedAt)
}

func (r *JobRepository) UpdateStatus(id uuid.UUID, status models.JobStatus, progress int, errMsg *string) error {
	query := `UPDATE job_history SET status = $1, progress = $2, error_message = $3, updated_at = CURRENT_TIMESTAMP`
	if status == models.JobCompleted || status == models.JobFailed {
		query += `, completed_at = CURRENT_TIMESTAMP`
	}
	query += ` WHERE id = $4`
	_, err := r.db.Exec(query, status, progress, errMsg, id)
	return err
}

func (r *JobRepository) GetByID(id uuid.UUID) (*models.JobRecord, error) {
	job := &models.JobRecord{}
	query := `SELECT id, job_type, status, progress, error_message, started_by, started_at, completed_at, updated_at
		FROM job_history WHERE id = $1`
	err := r.db.QueryRow(query, id).Scan(&job.ID, &job.JobType, &job.Status, &job.Progress,
		&job.ErrorMessage, &job.StartedBy, &job.StartedAt, &job.CompletedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	return job, err
}

func (r *JobRepository) ListRecent(limit int) ([]*models.JobRecord, error) {
	query := `SELECT id, job_type, status, progress, error_message, started_by, started_at, completed_at, updated_at
		FROM job_history ORDER BY started_at DESC LIMIT $1`
	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []*models.JobRecord
	for rows.Next() {
		job := &models.JobRecord{}
		if err := rows.Scan(&job.ID, &job.JobType, &job.Status, &job.Progress,
			&job.ErrorMessage, &job.StartedBy, &job.StartedAt, &job.CompletedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

