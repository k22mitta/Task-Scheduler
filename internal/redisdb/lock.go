package redisdb

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

var refreshScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

type Lock struct {
	client  *redis.Client
	key     string
	ownerID string
	ttl     time.Duration
}

func NewLock(client *redis.Client, key string, ownerID string, ttl time.Duration) *Lock {
	return &Lock{client: client, key: key, ownerID: ownerID, ttl: ttl}
}

func (l *Lock) Acquire(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.ownerID, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire lock: %w", err)
	}
	return ok, nil
}

func (l *Lock) Release(ctx context.Context) error {
	result, err := releaseScript.Run(ctx, l.client, []string{l.key}, l.ownerID).Int()
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	if result == 0 {
		return fmt.Errorf("lock not owned by %s", l.ownerID)
	}
	return nil
}

func (l *Lock) Refresh(ctx context.Context) error {
	ttlMs := l.ttl.Milliseconds()
	result, err := refreshScript.Run(ctx, l.client, []string{l.key}, l.ownerID, ttlMs).Int()
	if err != nil {
		return fmt.Errorf("refresh lock: %w", err)
	}
	if result == 0 {
		return fmt.Errorf("lock not owned by %s", l.ownerID)
	}
	return nil
}
