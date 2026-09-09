// Package queue is a thin wrapper over a Redis list used as a work queue.
// Scheduler pushes monitor IDs; workers block-pop them. Same code in every
// environment — ElastiCache in AWS, a Redis pod in k3s.
package queue

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const checksKey = "pulse:checks"

type Queue struct{ rdb *redis.Client }

func New(url string) (*Queue, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &Queue{rdb: redis.NewClient(opt)}, nil
}

func (q *Queue) Enqueue(ctx context.Context, monitorID string) error {
	return q.rdb.LPush(ctx, checksKey, monitorID).Err()
}

// Dequeue blocks up to timeout waiting for work. Returns "" on timeout.
func (q *Queue) Dequeue(ctx context.Context, timeout time.Duration) (string, error) {
	res, err := q.rdb.BRPop(ctx, timeout, checksKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return res[1], nil
}

func (q *Queue) Depth(ctx context.Context) (int64, error) {
	return q.rdb.LLen(ctx, checksKey).Result()
}
