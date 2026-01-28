package cache

import (
	"context"
	"time"
)

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from cache
	Get(ctx context.Context, key string) (string, error)

	// Set stores a value in cache with optional TTL
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete removes a key from cache
	Delete(ctx context.Context, keys ...string) error

	// Exists checks if a key exists
	Exists(ctx context.Context, keys ...string) (int64, error)

	// Expire sets an expiration on a key
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// TTL returns the remaining time to live
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Increment increments a key's value by 1
	Increment(ctx context.Context, key string) (int64, error)

	// Decrement decrements a key's value by 1
	Decrement(ctx context.Context, key string) (int64, error)

	// MGet retrieves multiple values
	MGet(ctx context.Context, keys ...string) ([]any, error)

	// MSet sets multiple key-value pairs
	MSet(ctx context.Context, pairs map[string]any) error

	// FlushAll clears all keys (use with caution!)
	FlushAll(ctx context.Context) error

	// Ping checks if cache is reachable
	Ping(ctx context.Context) error

	// Close closes the cache connection
	Close() error
}
