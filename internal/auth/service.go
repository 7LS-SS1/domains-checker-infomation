package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrInvalidCSRF        = errors.New("invalid CSRF token")
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	DisplayName  string    `json:"display_name"`
	Locale       string    `json:"locale"`
	Status       string    `json:"status"`
	PasswordHash string    `json:"-"`
	Roles        []string  `json:"roles"`
}

type Principal struct {
	UserID      uuid.UUID
	Email       string
	DisplayName string
	Locale      string
	Roles       map[string]struct{}
	SessionID   uuid.UUID
	TokenHash   []byte
	CSRFHash    []byte
}

func (p Principal) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if _, ok := p.Roles[role]; ok {
			return true
		}
	}
	return false
}

type Session struct {
	User      User
	Token     string
	CSRFToken string
	ExpiresAt time.Time
}

type Service struct {
	pool     *pgxpool.Pool
	audit    *audit.Store
	ttl      time.Duration
	now      func() time.Time
	newToken func() (string, error)
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store, ttl time.Duration) *Service {
	return &Service{
		pool:     pool,
		audit:    auditStore,
		ttl:      ttl,
		now:      time.Now,
		newToken: randomToken,
	}
}

func (s *Service) Login(ctx context.Context, email, password, requestID string) (Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return Session{}, ErrInvalidCredentials
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Session{}, fmt.Errorf("begin login transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	user, err := loadUserByEmail(ctx, tx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			burnPasswordVerification(password)
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	passwordValid := VerifyPassword(password, user.PasswordHash)
	if !passwordValid || user.Status != "active" {
		return Session{}, ErrInvalidCredentials
	}

	token, err := s.newToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate session token: %w", err)
	}
	csrfToken, err := s.newToken()
	if err != nil {
		return Session{}, fmt.Errorf("generate CSRF token: %w", err)
	}
	tokenHash := sha256.Sum256([]byte(token))
	csrfHash := sha256.Sum256([]byte(csrfToken))
	now := s.now().UTC()
	expiresAt := now.Add(s.ttl)
	sessionID := uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`, sessionID, user.ID, tokenHash[:], csrfHash[:], expiresAt, now); err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{
		ActorUserID:  &user.ID,
		Action:       "SESSION_CREATED",
		ResourceType: "session",
		ResourceID:   &sessionID,
		RequestID:    requestID,
		After: map[string]any{
			"user_id":    user.ID,
			"expires_at": expiresAt,
		},
	}); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit login transaction: %w", err)
	}
	return Session{User: user, Token: token, CSRFToken: csrfToken, ExpiresAt: expiresAt}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrInvalidSession
	}
	tokenHash := sha256.Sum256([]byte(token))
	var principal Principal
	var roles []string
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.email::text, u.display_name, u.locale, s.id, s.token_hash, s.csrf_token_hash,
		       COALESCE(array_agg(r.code ORDER BY r.code) FILTER (WHERE r.code IS NOT NULL), '{}')
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE s.token_hash = $1
		  AND s.revoked_at IS NULL
		  AND s.expires_at > now()
		  AND u.status = 'active'
		GROUP BY u.id, s.id
	`, tokenHash[:]).Scan(
		&principal.UserID, &principal.Email, &principal.DisplayName, &principal.Locale,
		&principal.SessionID, &principal.TokenHash, &principal.CSRFHash, &roles,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Principal{}, ErrInvalidSession
		}
		return Principal{}, fmt.Errorf("load session: %w", err)
	}
	principal.Roles = make(map[string]struct{}, len(roles))
	for _, role := range roles {
		principal.Roles[role] = struct{}{}
	}
	return principal, nil
}

func (s *Service) ValidateCSRF(principal Principal, rawToken string) error {
	actual := sha256.Sum256([]byte(rawToken))
	if len(principal.CSRFHash) != len(actual) || !equalBytes(principal.CSRFHash, actual[:]) {
		return ErrInvalidCSRF
	}
	return nil
}

func (s *Service) Logout(ctx context.Context, principal Principal, requestID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin logout transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	command, err := tx.Exec(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE id = $1 AND revoked_at IS NULL
	`, principal.SessionID)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrInvalidSession
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{
		ActorUserID:  &principal.UserID,
		Action:       "SESSION_REVOKED",
		ResourceType: "session",
		ResourceID:   &principal.SessionID,
		RequestID:    requestID,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit logout transaction: %w", err)
	}
	return nil
}

func loadUserByEmail(ctx context.Context, tx pgx.Tx, email string) (User, error) {
	var user User
	err := tx.QueryRow(ctx, `
		SELECT u.id, u.email::text, u.display_name, u.locale, u.status, u.password_hash,
		       COALESCE(array_agg(r.code ORDER BY r.code) FILTER (WHERE r.code IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE u.email = $1
		GROUP BY u.id
	`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Locale, &user.Status, &user.PasswordHash, &user.Roles)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func randomToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for i := range left {
		difference |= left[i] ^ right[i]
	}
	return difference == 0
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}
