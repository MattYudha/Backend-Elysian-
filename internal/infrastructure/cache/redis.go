package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(cfg *config.Config) (Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisDSN(),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
	}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}

	return value, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	err := c.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return nil
}

func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	err := c.client.Del(ctx, keys...).Err()
	if err != nil {
		return fmt.Errorf("failed to delete key %w: ", err)
	}

	return nil
}

func (c *RedisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	count, err := c.client.Exists(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to check if key exist %w: ", err)
	}

	return count, nil
}

func (c *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	err := c.client.Expire(ctx, key, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration key %s: %w", key, err)
	}

	return nil
}

func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return ttl, fmt.Errorf("failed to get TTL key %s: %w", key, err)
	}

	return ttl, nil
}

func (c *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	inc, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return inc, fmt.Errorf("failed to inc key %s: %w", key, err)
	}

	return inc, nil
}

func (c *RedisCache) Decrement(ctx context.Context, key string) (int64, error) {
	dec, err := c.client.Decr(ctx, key).Result()
	if err != nil {
		return dec, fmt.Errorf("failed to dec key %s: %w", key, err)
	}

	return dec, nil
}

func (c *RedisCache) MGet(ctx context.Context, keys ...string) ([]any, error) {
	vals, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple keys: %w", err)
	}

	return vals, nil
}

func (c *RedisCache) MSet(ctx context.Context, pairs map[string]any) error {
	values := make([]any, 0, len(pairs)*2)
	for k, v := range pairs {
		values = append(values, k, v)
	}

	err := c.client.MSet(ctx, values...).Err()
	if err != nil {
		return fmt.Errorf("failed to set multiple keys: %w", err)
	}

	return nil
}

func (c *RedisCache) FlushAll(ctx context.Context) error {
	err := c.client.FlushAll(ctx).Err()
	if err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}

	return nil
}

func (c *RedisCache) Ping(ctx context.Context) error {
	err := c.client.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("failed to flush: %w", err)
	}

	return nil
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

// GetStats returns Redis statistics in a structured format
func (c *RedisCache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	// Get pool stats
	poolStats := c.client.PoolStats()

	// Get Redis server info
	info, err := c.client.Info(ctx, "stats", "memory", "server").Result()
	if err != nil {
		return map[string]interface{}{
			"pool": map[string]interface{}{
				"hits":        poolStats.Hits,
				"misses":      poolStats.Misses,
				"timeouts":    poolStats.Timeouts,
				"total_conns": poolStats.TotalConns,
				"idle_conns":  poolStats.IdleConns,
				"stale_conns": poolStats.StaleConns,
			},
			"server": map[string]interface{}{
				"available": false,
				"error":     err.Error(),
			},
		}, nil
	}

	parsedInfo := parseRedisInfo(info)

	return map[string]any{
		"pool": map[string]any{
			"hits":        poolStats.Hits,
			"misses":      poolStats.Misses,
			"timeouts":    poolStats.Timeouts,
			"total_conns": poolStats.TotalConns,
			"idle_conns":  poolStats.IdleConns,
			"stale_conns": poolStats.StaleConns,
		},
		"server": parsedInfo,
	}, nil
}

func parseRedisInfo(info string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(info, "\r\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}

	return result
}

func (c *RedisCache) GetClient() *redis.Client {
	return c.client
}
