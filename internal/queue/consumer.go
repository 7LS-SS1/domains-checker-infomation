package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type MessageHandler func(context.Context, map[string]any) error

type Consumer struct {
	redis      *redis.Client
	stream     string
	group      string
	consumerID string
	workers    int
	lease      time.Duration
	block      time.Duration
	onError    func(error)
}

func NewConsumer(redisClient *redis.Client, stream, group, consumerID string, workers int, lease, block time.Duration, onError func(error)) *Consumer {
	if workers < 1 {
		workers = 1
	}
	if lease <= 0 {
		lease = 90 * time.Second
	}
	if block <= 0 {
		block = 2 * time.Second
	}
	if onError == nil {
		onError = func(error) {}
	}
	return &Consumer{redis: redisClient, stream: stream, group: group, consumerID: consumerID, workers: workers, lease: lease, block: block, onError: onError}
}

func (c *Consumer) Run(ctx context.Context, handler MessageHandler) error {
	if err := c.redis.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err(); err != nil && !stringsContains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("create Redis consumer group: %w", err)
	}
	jobs := make(chan redis.XMessage, c.workers*2)
	var workers sync.WaitGroup
	for range c.workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for message := range jobs {
				if err := handler(ctx, message.Values); err != nil {
					c.onError(fmt.Errorf("process Redis message %s: %w", message.ID, err))
					continue
				}
				if err := c.redis.XAck(ctx, c.stream, c.group, message.ID).Err(); err != nil && ctx.Err() == nil {
					c.onError(fmt.Errorf("ack Redis message %s: %w", message.ID, err))
				}
			}
		}()
	}
	defer func() {
		close(jobs)
		workers.Wait()
	}()

	lastClaim := time.Time{}
	claimCursor := "0-0"
	for ctx.Err() == nil {
		if lastClaim.IsZero() || time.Since(lastClaim) >= c.lease/2 {
			messages, next, err := c.redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{
				Stream: c.stream, Group: c.group, Consumer: c.consumerID,
				MinIdle: c.lease, Start: claimCursor, Count: int64(c.workers),
			}).Result()
			if err != nil && !errors.Is(err, redis.Nil) && ctx.Err() == nil {
				c.onError(fmt.Errorf("reclaim Redis messages: %w", err))
			} else {
				claimCursor = next
				if claimCursor == "0-0" {
					lastClaim = time.Now()
				}
				for _, message := range messages {
					if !sendMessage(ctx, jobs, message) {
						return ctx.Err()
					}
				}
			}
		}
		streams, err := c.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: c.group, Consumer: c.consumerID, Streams: []string{c.stream, ">"},
			Count: int64(c.workers), Block: c.block,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			if ctx.Err() != nil {
				break
			}
			c.onError(fmt.Errorf("read Redis stream: %w", err))
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				if !sendMessage(ctx, jobs, message) {
					return ctx.Err()
				}
			}
		}
	}
	return ctx.Err()
}

func sendMessage(ctx context.Context, jobs chan<- redis.XMessage, message redis.XMessage) bool {
	select {
	case jobs <- message:
		return true
	case <-ctx.Done():
		return false
	}
}

func stringsContains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
