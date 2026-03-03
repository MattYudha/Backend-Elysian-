package mq

import (
	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/hibiken/asynq"
)

type AsynqClient struct {
	client *asynq.Client
}

func NewAsynqClient(cfg *config.Config) *AsynqClient {
	redisConnOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	client := asynq.NewClient(redisConnOpt)
	return &AsynqClient{client: client}
}

func (a *AsynqClient) EnqueueTask(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	return a.client.Enqueue(task, opts...)
}

func (a *AsynqClient) Close() error {
	return a.client.Close()
}
