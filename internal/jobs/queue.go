package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hibiken/asynq"
)

const (
	TaskScanLibrary      = "scan:library"
	TaskFingerprint      = "fingerprint:media"
	TaskPhashLibrary     = "phash:library"
	TaskSpritesLibrary   = "sprites:library"
	TaskPreviewsLibrary  = "previews:library"
	TaskGeneratePreview  = "preview:generate"
	TaskMetadataScrape   = "metadata:scrape"
	TaskMetadataRefresh  = "metadata:refresh"
	TaskDetectSegments   = "detect:segments"
)

type Queue struct {
	client    *asynq.Client
	server    *asynq.Server
	mux       *asynq.ServeMux
	inspector *asynq.Inspector
}

func NewQueue(redisAddr string) *Queue {
	redisOpt := asynq.RedisClientOpt{Addr: redisAddr}
	client := asynq.NewClient(redisOpt)
	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 2,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)
	mux := asynq.NewServeMux()
	inspector := asynq.NewInspector(redisOpt)
	return &Queue{client: client, server: server, mux: mux, inspector: inspector}
}

// isTaskConflict checks whether the error indicates a task ID conflict,
// using errors.Is for unwrapped sentinel values and a string fallback.
func isTaskConflict(err error) bool {
	if errors.Is(err, asynq.ErrDuplicateTask) || errors.Is(err, asynq.ErrTaskIDConflict) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "task ID conflicts") || strings.Contains(msg, "duplicate task")
}

// EnqueueUnique enqueues a task with a deterministic TaskID to prevent
// duplicate jobs for the same library/item. If a task with the same ID
// is already pending or active, the enqueue is silently skipped.
// If a completed/archived task with the same ID is lingering in Redis,
// it is deleted first so the new task can be enqueued.
func (q *Queue) EnqueueUnique(taskType string, payload interface{}, uniqueID string, opts ...asynq.Option) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	opts = append(opts, asynq.TaskID(uniqueID))
	task := asynq.NewTask(taskType, data, opts...)
	info, err := q.client.Enqueue(task)
	if err == nil {
		return info.ID, nil
	}

	if !isTaskConflict(err) {
		return "", fmt.Errorf("enqueue: %w", err)
	}

	// Task ID conflict — check if the old task is completed/archived and can be cleared
	cleared := false
	for _, queueName := range []string{"default", "critical", "low"} {
		// Try deleting from completed state
		if delErr := q.inspector.DeleteTask(queueName, uniqueID); delErr == nil {
			log.Printf("Queue: cleared completed/archived task %s from queue %s", uniqueID, queueName)
			cleared = true
			break
		}
	}

	if cleared {
		// Retry enqueue after clearing the stale task
		info, err = q.client.Enqueue(task)
		if err == nil {
			return info.ID, nil
		}
	}

	// If we still can't enqueue, the task is likely actively running — that's OK
	if isTaskConflict(err) {
		log.Printf("Queue: task %s (%s) is already active, skipping", taskType, uniqueID)
		return uniqueID, nil
	}
	return "", fmt.Errorf("enqueue: %w", err)
}

func (q *Queue) RegisterHandler(taskType string, handler asynq.Handler) {
	q.mux.Handle(taskType, handler)
}

func (q *Queue) Enqueue(taskType string, payload interface{}, opts ...asynq.Option) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(taskType, data, opts...)
	info, err := q.client.Enqueue(task)
	if err != nil {
		return "", fmt.Errorf("enqueue: %w", err)
	}
	return info.ID, nil
}

func (q *Queue) Start(ctx context.Context) error {
	log.Println("Job queue worker starting...")
	return q.server.Start(q.mux)
}

func (q *Queue) Stop() {
	q.server.Shutdown()
	q.client.Close()
	q.inspector.Close()
}

func (q *Queue) Client() *asynq.Client {
	return q.client
}
