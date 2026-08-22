package queue

import (
	"context"
	"encoding/json"
	"fmt"
	rand "math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Event struct {
	ID             uuid.UUID
	IdempotencyKey string
	Type           string
	Stream         string
	Payload        json.RawMessage
	CreatedAt      time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) EnqueueTx(ctx context.Context, tx pgx.Tx, idempotencyKey, eventType, stream string, payload any) (uuid.UUID, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshal outbox payload: %w", err)
	}
	id := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO job_outbox (id, idempotency_key, event_type, stream_name, payload)
		VALUES ($1, $2, $3, $4, $5::jsonb)
	`, id, idempotencyKey, eventType, stream, string(encoded)); err != nil {
		return uuid.Nil, fmt.Errorf("enqueue outbox event: %w", err)
	}
	return id, nil
}

type Dispatcher struct {
	pool        *pgxpool.Pool
	redis       *redis.Client
	workerID    string
	batchSize   int
	lease       time.Duration
	maxAttempts int
	maxStream   int64
	now         func() time.Time
}

func NewDispatcher(pool *pgxpool.Pool, redisClient *redis.Client, batchSize int, lease time.Duration, maxAttempts int) *Dispatcher {
	return &Dispatcher{
		pool:        pool,
		redis:       redisClient,
		workerID:    uuid.NewString(),
		batchSize:   batchSize,
		lease:       lease,
		maxAttempts: maxAttempts,
		maxStream:   100000,
		now:         time.Now,
	}
}

func (d *Dispatcher) RunOnce(ctx context.Context) (int, error) {
	events, err := d.claim(ctx)
	if err != nil {
		return 0, err
	}
	dispatched := 0
	for _, event := range events {
		if err := d.dispatch(ctx, event); err != nil {
			if markErr := d.markFailure(ctx, event.ID, err); markErr != nil {
				return dispatched, fmt.Errorf("dispatch event %s: %v; mark failure: %w", event.ID, err, markErr)
			}
			continue
		}
		if err := d.markDispatched(ctx, event.ID); err != nil {
			return dispatched, err
		}
		dispatched++
	}
	return dispatched, nil
}

func (d *Dispatcher) claim(ctx context.Context) ([]Event, error) {
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin outbox claim: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		UPDATE job_outbox
		SET status = 'pending', locked_at = NULL, locked_by = NULL,
		    last_error_code = 'DISPATCH_LEASE_EXPIRED',
		    last_error_message = 'Dispatch lease expired before completion.'
		WHERE status = 'dispatching' AND locked_at < now() - ($1 * interval '1 millisecond')
	`, d.lease.Milliseconds()); err != nil {
		return nil, fmt.Errorf("requeue stale outbox events: %w", err)
	}
	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT id
			FROM job_outbox
			WHERE status = 'pending' AND available_at <= now() AND attempts < $1
			ORDER BY available_at, created_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE job_outbox AS outbox
		SET status = 'dispatching', locked_at = now(), locked_by = $3
		FROM candidates
		WHERE outbox.id = candidates.id
		RETURNING outbox.id, outbox.idempotency_key, outbox.event_type,
		          outbox.stream_name, outbox.payload, outbox.created_at
	`, d.maxAttempts, d.batchSize, d.workerID)
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer rows.Close()
	events := make([]Event, 0, d.batchSize)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.IdempotencyKey, &event.Type, &event.Stream, &event.Payload, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit outbox claim: %w", err)
	}
	return events, nil
}

func (d *Dispatcher) dispatch(ctx context.Context, event Event) error {
	_, err := d.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: event.Stream,
		MaxLen: d.maxStream,
		Approx: true,
		Values: map[string]any{
			"job_id":          event.ID.String(),
			"idempotency_key": event.IdempotencyKey,
			"event_type":      event.Type,
			"payload":         string(event.Payload),
			"created_at":      event.CreatedAt.UTC().Format(time.RFC3339Nano),
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("XADD %s: %w", event.Stream, err)
	}
	return nil
}

func (d *Dispatcher) markDispatched(ctx context.Context, id uuid.UUID) error {
	command, err := d.pool.Exec(ctx, `
		UPDATE job_outbox
		SET status = 'dispatched', dispatched_at = now(), locked_at = NULL, locked_by = NULL,
		    last_error_code = NULL, last_error_message = NULL
		WHERE id = $1 AND status = 'dispatching' AND locked_by = $2
	`, id, d.workerID)
	if err != nil {
		return fmt.Errorf("mark outbox event dispatched: %w", err)
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("mark outbox event dispatched: lease lost")
	}
	return nil
}

func (d *Dispatcher) markFailure(ctx context.Context, id uuid.UUID, dispatchErr error) error {
	message := strings.TrimSpace(dispatchErr.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	jitterMilliseconds := rand.IntN(1001)
	var attempts int
	if err := d.pool.QueryRow(ctx, `
		UPDATE job_outbox
		SET attempts = attempts + 1,
		    status = CASE WHEN attempts + 1 >= $3 THEN 'failed'::outbox_status ELSE 'pending'::outbox_status END,
		    available_at = now() + (LEAST(300, power(2, attempts + 1)) * interval '1 second')
		                   + ($5 * interval '1 millisecond'),
		    locked_at = NULL, locked_by = NULL,
		    last_error_code = 'REDIS_DISPATCH_FAILED', last_error_message = $4
		WHERE id = $1 AND status = 'dispatching' AND locked_by = $2
		RETURNING attempts
	`, id, d.workerID, d.maxAttempts, message, jitterMilliseconds).Scan(&attempts); err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	return nil
}
