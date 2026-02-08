package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hibiken/asynq"
)

const (
	TaskScanLibrary    = "scan:library"
	TaskFingerprint    = "fingerprint:media"
	TaskPhashLibrary   = "phash:library"
	TaskGeneratePreview = "preview:generate"
	TaskMetadataScrape = "metadata:scrape"
)

type Queue struct {
	client *asynq.Client
	server *asynq.Server
	mux    *asynq.ServeMux
}

func NewQueue(redisAddr string) *Queue {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)
	mux := asynq.NewServeMux()
	return &Queue{client: client, server: server, mux: mux}
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
}

func (q *Queue) Client() *asynq.Client {
	return q.client
}
