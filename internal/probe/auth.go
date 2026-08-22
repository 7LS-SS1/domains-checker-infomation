package probe

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/probeprotocol"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) CreateRegistrationToken(ctx context.Context, spec RegistrationSpec) (RegistrationToken, error) {
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Region = strings.ToUpper(strings.TrimSpace(spec.Region))
	spec.Country = strings.ToUpper(strings.TrimSpace(spec.Country))
	spec.Network = strings.TrimSpace(spec.Network)
	if spec.Name == "" || len(spec.Name) > 200 || len(spec.Region) < 2 || len(spec.Region) > 16 || len(spec.Country) != 2 {
		return RegistrationToken{}, ErrInvalidRequest
	}
	if spec.TTL <= 0 {
		spec.TTL = 10 * time.Minute
	}
	if spec.TTL > 24*time.Hour {
		return RegistrationToken{}, ErrInvalidRequest
	}
	secret, hash, err := probeprotocol.NewSecret()
	if err != nil {
		return RegistrationToken{}, err
	}
	item := RegistrationToken{ID: uuid.New(), Token: secret, Name: spec.Name, Region: spec.Region, Country: spec.Country, Network: spec.Network, ExpiresAt: s.now().UTC().Add(spec.TTL)}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RegistrationToken{}, fmt.Errorf("begin registration token transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO probe_registration_tokens
		(id, token_hash, name, region_code, country_code, network_name, expires_at, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, item.ID, hash[:], item.Name, item.Region, item.Country, nullableText(item.Network), item.ExpiresAt, spec.CreatedBy)
	if err != nil {
		return RegistrationToken{}, fmt.Errorf("insert probe registration token: %w", err)
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{
		ActorUserID: &spec.CreatedBy, Action: "PROBE_REGISTRATION_TOKEN_CREATED", ResourceType: "probe_registration_token",
		ResourceID: &item.ID, RequestID: spec.RequestID,
		After: map[string]any{"name": item.Name, "region_code": item.Region, "country_code": item.Country, "expires_at": item.ExpiresAt},
	}); err != nil {
		return RegistrationToken{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RegistrationToken{}, fmt.Errorf("commit probe registration token: %w", err)
	}
	return item, nil
}

func (s *Service) Register(ctx context.Context, request probeprotocol.RegisterRequest) (probeprotocol.RegisterResponse, error) {
	hash, err := probeprotocol.HashSecret(request.RegistrationToken)
	if err != nil {
		return probeprotocol.RegisterResponse{}, ErrUnauthorized
	}
	publicKey, err := probeprotocol.DecodePublicKey(request.PublicKey)
	if err != nil {
		return probeprotocol.RegisterResponse{}, errors.Join(ErrInvalidRequest, err)
	}
	request.Name = strings.TrimSpace(request.Name)
	request.RegionCode = strings.ToUpper(strings.TrimSpace(request.RegionCode))
	request.CountryCode = strings.ToUpper(strings.TrimSpace(request.CountryCode))
	request.NetworkName = strings.TrimSpace(request.NetworkName)
	if request.ProtocolVersion != probeprotocol.Version || request.AgentVersion == "" {
		return probeprotocol.RegisterResponse{}, ErrInvalidRequest
	}
	capabilities, err := json.Marshal(request.Capabilities)
	if err != nil {
		return probeprotocol.RegisterResponse{}, ErrInvalidRequest
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return probeprotocol.RegisterResponse{}, fmt.Errorf("begin probe registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var tokenID uuid.UUID
	var name, region, country string
	var network *string
	var expiresAt time.Time
	var usedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, name, region_code, country_code, network_name, expires_at, used_at
		FROM probe_registration_tokens WHERE token_hash = $1 FOR UPDATE
	`, hash[:]).Scan(&tokenID, &name, &region, &country, &network, &expiresAt, &usedAt)
	if err == pgx.ErrNoRows || usedAt != nil {
		return probeprotocol.RegisterResponse{}, ErrUnauthorized
	}
	if err != nil {
		return probeprotocol.RegisterResponse{}, fmt.Errorf("load probe registration token: %w", err)
	}
	now := s.now().UTC()
	if !expiresAt.After(now) {
		return probeprotocol.RegisterResponse{}, ErrExpired
	}
	expectedNetwork := ""
	if network != nil {
		expectedNetwork = *network
	}
	if request.Name != name || request.RegionCode != region || request.CountryCode != country || request.NetworkName != expectedNetwork {
		return probeprotocol.RegisterResponse{}, ErrForbidden
	}
	probeID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO probe_nodes
		(id, name, region_code, country_code, network_name, public_key, status, version, capabilities, registered_at)
		VALUES ($1,$2,$3,$4,$5,$6,'OFFLINE',$7,$8::jsonb,$9)
	`, probeID, name, region, country, network, []byte(publicKey), request.AgentVersion, string(capabilities), now)
	if err != nil {
		return probeprotocol.RegisterResponse{}, errors.Join(ErrConflict, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE probe_registration_tokens SET used_at = $2 WHERE id = $1`, tokenID, now); err != nil {
		return probeprotocol.RegisterResponse{}, fmt.Errorf("consume probe registration token: %w", err)
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{
		Action: "PROBE_REGISTERED", ResourceType: "probe_node", ResourceID: &probeID,
		After: map[string]any{"name": name, "region_code": region, "country_code": country, "version": request.AgentVersion},
	}); err != nil {
		return probeprotocol.RegisterResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return probeprotocol.RegisterResponse{}, fmt.Errorf("commit probe registration: %w", err)
	}
	return probeprotocol.RegisterResponse{ProbeID: probeID, ProtocolVersion: probeprotocol.Version, RegisteredAt: now}, nil
}

func (s *Service) CreateChallenge(ctx context.Context, probeID uuid.UUID) (probeprotocol.TokenChallenge, error) {
	if probeID == uuid.Nil {
		return probeprotocol.TokenChallenge{}, ErrInvalidRequest
	}
	var status string
	var revokedAt *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT status::text, revoked_at FROM probe_nodes WHERE id = $1`, probeID).Scan(&status, &revokedAt); err != nil {
		if err == pgx.ErrNoRows {
			return probeprotocol.TokenChallenge{}, ErrUnauthorized
		}
		return probeprotocol.TokenChallenge{}, fmt.Errorf("load probe for challenge: %w", err)
	}
	if status == "REVOKED" || revokedAt != nil {
		return probeprotocol.TokenChallenge{}, ErrForbidden
	}
	challengeRaw := make([]byte, 32)
	if _, err := rand.Read(challengeRaw); err != nil {
		return probeprotocol.TokenChallenge{}, fmt.Errorf("generate probe challenge: %w", err)
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	item := probeprotocol.TokenChallenge{ChallengeID: uuid.New(), Challenge: base64.RawURLEncoding.EncodeToString(challengeRaw), ExpiresAt: now.Add(s.cfg.ProbeChallengeTTL)}
	item.SigningMessage = probeprotocol.TokenSigningMessage(item.ChallengeID, item.Challenge, item.ExpiresAt)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO probe_auth_challenges (id, probe_node_id, challenge, expires_at)
		VALUES ($1,$2,$3,$4)
	`, item.ChallengeID, probeID, challengeRaw, item.ExpiresAt); err != nil {
		return probeprotocol.TokenChallenge{}, fmt.Errorf("insert probe challenge: %w", err)
	}
	return item, nil
}

func (s *Service) IssueToken(ctx context.Context, request probeprotocol.TokenRequest) (probeprotocol.TokenResponse, error) {
	signature, err := probeprotocol.DecodeSignature(request.Signature)
	if err != nil || request.ProbeID == uuid.Nil || request.ChallengeID == uuid.Nil {
		return probeprotocol.TokenResponse{}, ErrUnauthorized
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return probeprotocol.TokenResponse{}, fmt.Errorf("begin probe token issue: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var challenge []byte
	var expiresAt time.Time
	var usedAt *time.Time
	var publicKey []byte
	var status string
	err = tx.QueryRow(ctx, `
		SELECT challenge.challenge, challenge.expires_at, challenge.used_at, node.public_key, node.status::text
		FROM probe_auth_challenges challenge
		JOIN probe_nodes node ON node.id = challenge.probe_node_id
		WHERE challenge.id = $1 AND challenge.probe_node_id = $2
		FOR UPDATE OF challenge
	`, request.ChallengeID, request.ProbeID).Scan(&challenge, &expiresAt, &usedAt, &publicKey, &status)
	if err == pgx.ErrNoRows {
		return probeprotocol.TokenResponse{}, ErrUnauthorized
	}
	if err != nil {
		return probeprotocol.TokenResponse{}, fmt.Errorf("load probe challenge: %w", err)
	}
	now := s.now().UTC()
	if usedAt != nil {
		return probeprotocol.TokenResponse{}, ErrReplay
	}
	if !expiresAt.After(now) {
		return probeprotocol.TokenResponse{}, ErrExpired
	}
	if status == "REVOKED" || len(publicKey) != ed25519.PublicKeySize {
		return probeprotocol.TokenResponse{}, ErrForbidden
	}
	encodedChallenge := base64.RawURLEncoding.EncodeToString(challenge)
	message := probeprotocol.TokenSigningMessage(request.ChallengeID, encodedChallenge, expiresAt)
	if !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(message), signature) {
		return probeprotocol.TokenResponse{}, ErrSignature
	}
	secret, hash, err := probeprotocol.NewSecret()
	if err != nil {
		return probeprotocol.TokenResponse{}, err
	}
	tokenID := uuid.New()
	tokenExpiresAt := now.Add(s.cfg.ProbeTokenTTL)
	if _, err := tx.Exec(ctx, `UPDATE probe_auth_challenges SET used_at = $2 WHERE id = $1`, request.ChallengeID, now); err != nil {
		return probeprotocol.TokenResponse{}, fmt.Errorf("consume probe challenge: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO probe_access_tokens (id, probe_node_id, token_hash, expires_at)
		VALUES ($1,$2,$3,$4)
	`, tokenID, request.ProbeID, hash[:], tokenExpiresAt); err != nil {
		return probeprotocol.TokenResponse{}, fmt.Errorf("insert probe access token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return probeprotocol.TokenResponse{}, fmt.Errorf("commit probe access token: %w", err)
	}
	return probeprotocol.TokenResponse{AccessToken: secret, TokenType: "Bearer", ExpiresAt: tokenExpiresAt, Scope: []string{"heartbeat", "jobs:claim", "results:submit"}}, nil
}

func (s *Service) Authenticate(ctx context.Context, bearer string) (Principal, error) {
	hash, err := probeprotocol.HashSecret(bearer)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}
	var principal Principal
	var publicKey []byte
	var network *string
	var expiresAt time.Time
	var revokedAt *time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT token.id, node.id, node.name, node.region_code, node.country_code, node.network_name,
		       node.version, node.status::text, node.public_key, token.expires_at, token.revoked_at
		FROM probe_access_tokens token
		JOIN probe_nodes node ON node.id = token.probe_node_id
		WHERE token.token_hash = $1
	`, hash[:]).Scan(&principal.TokenID, &principal.ProbeID, &principal.Name, &principal.RegionCode,
		&principal.CountryCode, &network, &principal.AgentVersion, &principal.Status, &publicKey, &expiresAt, &revokedAt)
	if err == pgx.ErrNoRows {
		return Principal{}, ErrUnauthorized
	}
	if err != nil {
		return Principal{}, fmt.Errorf("load probe access token: %w", err)
	}
	if revokedAt != nil || !expiresAt.After(s.now().UTC()) {
		return Principal{}, ErrUnauthorized
	}
	if principal.Status == "REVOKED" {
		return Principal{}, ErrForbidden
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return Principal{}, ErrForbidden
	}
	if network != nil {
		principal.NetworkName = *network
	}
	principal.PublicKey = ed25519.PublicKey(publicKey)
	return principal, nil
}

func (s *Service) Heartbeat(ctx context.Context, principal Principal, request probeprotocol.HeartbeatRequest) (probeprotocol.HeartbeatResponse, error) {
	if request.AgentVersion == "" || request.QueueCapacity < 0 || request.QueueCapacity > 10000 {
		return probeprotocol.HeartbeatResponse{}, ErrInvalidRequest
	}
	status := "ONLINE"
	if request.ProtocolVersion != probeprotocol.Version {
		status = "UPGRADE_REQUIRED"
	} else if abs(request.ClockOffsetMS) > int(s.cfg.ProbeMaxClockSkew.Milliseconds()) {
		status = "DEGRADED"
	}
	capabilities, err := json.Marshal(request.Capabilities)
	if err != nil {
		return probeprotocol.HeartbeatResponse{}, ErrInvalidRequest
	}
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return probeprotocol.HeartbeatResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	command, err := tx.Exec(ctx, `
		UPDATE probe_nodes SET status = $2, version = $3, capabilities = $4::jsonb,
		last_seen_at = $5, clock_offset_ms = $6
		WHERE id = $1 AND revoked_at IS NULL
	`, principal.ProbeID, status, request.AgentVersion, string(capabilities), now, request.ClockOffsetMS)
	if err != nil {
		return probeprotocol.HeartbeatResponse{}, fmt.Errorf("update probe heartbeat: %w", err)
	}
	if command.RowsAffected() != 1 {
		return probeprotocol.HeartbeatResponse{}, ErrForbidden
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO probe_heartbeats
		(probe_node_id, status, protocol_version, agent_version, clock_offset_ms, queue_capacity, capabilities)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb)
	`, principal.ProbeID, status, request.ProtocolVersion, request.AgentVersion, request.ClockOffsetMS, request.QueueCapacity, string(capabilities)); err != nil {
		return probeprotocol.HeartbeatResponse{}, fmt.Errorf("insert probe heartbeat: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return probeprotocol.HeartbeatResponse{}, err
	}
	return probeprotocol.HeartbeatResponse{Status: status, ServerTime: now, NextAfterMS: 30000}, nil
}

func (s *Service) ListNodes(ctx context.Context) ([]Node, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, region_code, country_code, network_name, status::text, version,
		       capabilities, last_seen_at, clock_offset_ms, registered_at, revoked_at
		FROM probe_nodes ORDER BY region_code, name
	`)
	if err != nil {
		return nil, fmt.Errorf("query probe nodes: %w", err)
	}
	defer rows.Close()
	items := []Node{}
	for rows.Next() {
		var item Node
		if err := rows.Scan(&item.ID, &item.Name, &item.RegionCode, &item.CountryCode, &item.NetworkName,
			&item.Status, &item.Version, &item.Capabilities, &item.LastSeenAt, &item.ClockOffsetMS,
			&item.RegisteredAt, &item.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan probe node: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Revoke(ctx context.Context, probeID, actorID uuid.UUID, requestID, reason string) error {
	reason = strings.TrimSpace(reason)
	if probeID == uuid.Nil || reason == "" || len(reason) > 1000 {
		return ErrInvalidRequest
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	now := s.now().UTC()
	command, err := tx.Exec(ctx, `
		UPDATE probe_nodes SET status = 'REVOKED', revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL
	`, probeID, now)
	if err != nil {
		return fmt.Errorf("revoke probe: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE probe_access_tokens SET revoked_at = $2 WHERE probe_node_id = $1 AND revoked_at IS NULL`, probeID, now); err != nil {
		return fmt.Errorf("revoke probe tokens: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE remote_probe_jobs SET status = 'cancelled', last_error_code = 'PROBE_REVOKED' WHERE probe_node_id = $1 AND status IN ('queued','leased')`, probeID); err != nil {
		return fmt.Errorf("cancel probe jobs: %w", err)
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actorID, Action: "PROBE_REVOKED", ResourceType: "probe_node", ResourceID: &probeID, RequestID: requestID, Reason: reason}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nullableText(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func hashBytes(value []byte) []byte {
	hash := sha256.Sum256(value)
	return hash[:]
}
