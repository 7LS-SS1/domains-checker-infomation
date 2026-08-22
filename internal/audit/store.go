package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const auditChainLockID int64 = 734624119012337

type Entry struct {
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   *uuid.UUID
	RequestID    string
	Reason       string
	Before       any
	After        any
	Metadata     map[string]any
}

type Store struct{}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) AppendTx(ctx context.Context, tx pgx.Tx, entry Entry) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", auditChainLockID); err != nil {
		return fmt.Errorf("lock audit chain: %w", err)
	}

	var previousHash []byte
	err := tx.QueryRow(ctx, `
		SELECT entry_hash
		FROM audit_logs
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`).Scan(&previousHash)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("read previous audit hash: %w", err)
	}
	if previousHash == nil {
		previousHash = []byte{}
	}

	beforeJSON, err := marshalSnapshot(entry.Before)
	if err != nil {
		return fmt.Errorf("marshal audit before snapshot: %w", err)
	}
	afterJSON, err := marshalSnapshot(entry.After)
	if err != nil {
		return fmt.Errorf("marshal audit after snapshot: %w", err)
	}
	metadataJSON, err := marshalSnapshot(entry.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	id := uuid.New()
	createdAt := time.Now().UTC()
	hashPayload := struct {
		ID           string          `json:"id"`
		PreviousHash string          `json:"previous_hash"`
		ActorUserID  *uuid.UUID      `json:"actor_user_id"`
		Action       string          `json:"action"`
		ResourceType string          `json:"resource_type"`
		ResourceID   *uuid.UUID      `json:"resource_id"`
		RequestID    string          `json:"request_id"`
		Reason       string          `json:"reason"`
		Before       json.RawMessage `json:"before"`
		After        json.RawMessage `json:"after"`
		Metadata     json.RawMessage `json:"metadata"`
		CreatedAt    string          `json:"created_at"`
	}{
		ID:           id.String(),
		PreviousHash: hex.EncodeToString(previousHash),
		ActorUserID:  entry.ActorUserID,
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		RequestID:    entry.RequestID,
		Reason:       entry.Reason,
		Before:       beforeJSON,
		After:        afterJSON,
		Metadata:     metadataJSON,
		CreatedAt:    createdAt.Format(time.RFC3339Nano),
	}
	canonical, err := json.Marshal(hashPayload)
	if err != nil {
		return fmt.Errorf("marshal audit hash payload: %w", err)
	}
	entryHash := sha256.Sum256(canonical)

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs (
			id, actor_user_id, action, resource_type, resource_id, request_id,
			reason, before_redacted, after_redacted, metadata, prev_hash, entry_hash, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::jsonb, $11, $12, $13
		)
	`, id, entry.ActorUserID, entry.Action, entry.ResourceType, entry.ResourceID, entry.RequestID,
		entry.Reason, string(beforeJSON), string(afterJSON), string(metadataJSON), previousHash, entryHash[:], createdAt)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func marshalSnapshot(value any) (json.RawMessage, error) {
	if value == nil {
		return json.RawMessage("null"), nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}
