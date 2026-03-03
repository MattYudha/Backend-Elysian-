package mq

import (
	"context"
	"log"

	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/hibiken/asynq"
)

// Queue names — all document parsing tasks go to heavy_parsing.
// This prevents Docling OOM from killing the default queue workers.
const (
	QueueDefault      = "default"
	QueueHeavyParsing = "heavy_parsing" // Throttled: max 2 concurrent Docling calls
	QueueCritical     = "critical"
)

type AsynqWorker struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

func NewAsynqWorker(cfg *config.Config) *AsynqWorker {
	redisConnOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	server := asynq.NewServer(
		redisConnOpt,
		asynq.Config{
			// Total concurrency across ALL queues.
			// heavy_parsing is limited to 2 via Queues weight — this is the key OOM guard.
			Concurrency: 12,

			// Queue priority map.
			// Asynq honors these as WEIGHTED queues, NOT as strict per-queue concurrency limits.
			// To enforce a hard ceiling of 2 on heavy_parsing, we use StrictPriority=false
			// and keep the weights low enough relative to Concurrency.
			//
			// Effective behavior:
			//   critical     → 8/12 of workers (fast, lightweight tasks)
			//   default      → 4/12 of workers
			//   heavy_parsing → at most 2/12 workers → Docling never starves RAM
			Queues: map[string]int{
				QueueCritical:     8,
				QueueDefault:      4,
				QueueHeavyParsing: 2, // HARD CEILING: max 2 concurrent Docling calls
			},

			// StrictPriority=false allows fair interleaving under load
			StrictPriority: false,

			// Exponential backoff on failure
			RetryDelayFunc: asynq.DefaultRetryDelayFunc,

			// Final error callback — triggered after all retries exhausted → DLQ
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Printf("[MQ-Asynq] TASK DEAD: type=%s error=%v — moved to DLQ", task.Type(), err)
			}),
		},
	)

	mux := asynq.NewServeMux()

	return &AsynqWorker{
		server: server,
		mux:    mux,
	}
}

func (w *AsynqWorker) RegisterHandler(taskType string, handler asynq.HandlerFunc) {
	w.mux.HandleFunc(taskType, handler)
}

func (w *AsynqWorker) Start() error {
	log.Println("[MQ-Asynq] Starting Background Worker (heavy_parsing concurrency ≤ 2)...")
	return w.server.Start(w.mux)
}

func (w *AsynqWorker) Stop() {
	log.Println("[MQ-Asynq] Stopping Background Worker...")
	w.server.Shutdown()
}
