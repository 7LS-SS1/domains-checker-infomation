package drive

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotConfigured = errors.New("Google Drive OAuth is not configured")
	ErrNotConnected  = errors.New("Google Drive is not connected")
	ErrOAuthState    = errors.New("Google OAuth state is invalid or expired")
	ErrUnavailable   = errors.New("Google Drive is unavailable")
	ErrValidation    = errors.New("Google Drive request is invalid")
)

type Config struct {
	APIBase, AuthorizationURL, TokenURL, UserInfoURL string
	ClientID, ClientSecret, RedirectURL              string
	EncryptionKey                                    string
	Scopes                                           []string
	Timeout                                          time.Duration
	MaxBytes                                         int64
}

type Actor struct {
	UserID    uuid.UUID
	RequestID string
}

type Connection struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	GoogleEmail  string     `json:"google_email,omitempty"`
	Scopes       []string   `json:"scopes"`
	Status       string     `json:"status"`
	TokenExpires *time.Time `json:"token_expires_at,omitempty"`
	ConnectedAt  time.Time  `json:"connected_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Authorization struct {
	AuthorizationURL string    `json:"authorization_url"`
	ExpiresAt        time.Time `json:"expires_at"`
}

type File struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	MimeType     string    `json:"mime_type"`
	ModifiedTime time.Time `json:"modified_time,omitempty"`
	WebViewLink  string    `json:"web_view_link,omitempty"`
}

type FilePage struct {
	Items         []File `json:"items"`
	NextPageToken string `json:"next_page_token,omitempty"`
}

type Service struct {
	pool       *pgxpool.Pool
	audit      *audit.Store
	http       *http.Client
	config     Config
	aead       cipher.AEAD
	configured bool
	now        func() time.Time
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store, client *http.Client, config Config) *Service {
	if client == nil {
		client = &http.Client{}
	}
	if config.Timeout <= 0 {
		config.Timeout = 20 * time.Second
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = 2 << 20
	}
	service := &Service{pool: pool, audit: auditStore, http: client, config: config, now: time.Now}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(config.EncryptionKey))
	if err == nil && len(key) == 32 {
		block, blockErr := aes.NewCipher(key)
		if blockErr == nil {
			service.aead, _ = cipher.NewGCM(block)
		}
	}
	service.configured = strings.TrimSpace(config.ClientID) != "" && strings.TrimSpace(config.ClientSecret) != "" &&
		strings.TrimSpace(config.RedirectURL) != "" && service.aead != nil
	return service
}

func (s *Service) IsConfigured() bool { return s != nil && s.configured }

func (s *Service) Begin(ctx context.Context, actor Actor) (Authorization, error) {
	if !s.IsConfigured() {
		return Authorization{}, ErrNotConfigured
	}
	state, err := randomURLSafe(32)
	if err != nil {
		return Authorization{}, err
	}
	verifier, err := randomURLSafe(48)
	if err != nil {
		return Authorization{}, err
	}
	ciphertext, nonce, err := s.encrypt([]byte(verifier))
	if err != nil {
		return Authorization{}, err
	}
	digest := sha256.Sum256([]byte(state))
	expires := s.now().UTC().Add(10 * time.Minute)
	_, _ = s.pool.Exec(ctx, `DELETE FROM google_oauth_states WHERE expires_at < now() - interval '1 day'`)
	if _, err := s.pool.Exec(ctx, `INSERT INTO google_oauth_states (user_id,state_hash,verifier_ciphertext,verifier_nonce,expires_at) VALUES ($1,$2,$3,$4,$5)`, actor.UserID, digest[:], ciphertext, nonce, expires); err != nil {
		return Authorization{}, err
	}
	challenge := sha256.Sum256([]byte(verifier))
	authURL, err := url.Parse(s.config.AuthorizationURL)
	if err != nil {
		return Authorization{}, ErrNotConfigured
	}
	query := authURL.Query()
	query.Set("client_id", s.config.ClientID)
	query.Set("redirect_uri", s.config.RedirectURL)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(s.config.Scopes, " "))
	query.Set("access_type", "offline")
	query.Set("include_granted_scopes", "true")
	query.Set("prompt", "consent")
	query.Set("state", state)
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	query.Set("code_challenge_method", "S256")
	authURL.RawQuery = query.Encode()
	return Authorization{AuthorizationURL: authURL.String(), ExpiresAt: expires}, nil
}

func (s *Service) Complete(ctx context.Context, actor Actor, state, code string) (Connection, error) {
	if !s.IsConfigured() {
		return Connection{}, ErrNotConfigured
	}
	state, code = strings.TrimSpace(state), strings.TrimSpace(code)
	if len(state) < 32 || len(state) > 512 || len(code) < 4 || len(code) > 4096 {
		return Connection{}, ErrOAuthState
	}
	digest := sha256.Sum256([]byte(state))
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Connection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var verifierCiphertext, verifierNonce []byte
	err = tx.QueryRow(ctx, `UPDATE google_oauth_states SET consumed_at=now() WHERE state_hash=$1 AND user_id=$2 AND consumed_at IS NULL AND expires_at>now() RETURNING verifier_ciphertext,verifier_nonce`, digest[:], actor.UserID).Scan(&verifierCiphertext, &verifierNonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return Connection{}, ErrOAuthState
	}
	if err != nil {
		return Connection{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Connection{}, err
	}
	verifier, err := s.decrypt(verifierCiphertext, verifierNonce)
	if err != nil {
		return Connection{}, ErrOAuthState
	}
	token, err := s.exchangeCode(ctx, code, string(verifier))
	if err != nil {
		return Connection{}, err
	}
	email, err := s.userEmail(ctx, token.AccessToken)
	if err != nil {
		return Connection{}, err
	}
	accessCiphertext, accessNonce, err := s.encrypt([]byte(token.AccessToken))
	if err != nil {
		return Connection{}, err
	}
	var refreshCiphertext, refreshNonce []byte
	if token.RefreshToken != "" {
		refreshCiphertext, refreshNonce, err = s.encrypt([]byte(token.RefreshToken))
		if err != nil {
			return Connection{}, err
		}
	}
	expires := s.now().UTC().Add(time.Duration(max(token.ExpiresIn, 120)) * time.Second)
	connectionID := uuid.New()
	tx, err = s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Connection{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var result Connection
	err = tx.QueryRow(ctx, `
		INSERT INTO google_drive_connections (id,user_id,google_email,access_token_ciphertext,access_token_nonce,refresh_token_ciphertext,refresh_token_nonce,token_expires_at,scopes,status,connected_at,revoked_at)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,'active',now(),NULL)
		ON CONFLICT (user_id) DO UPDATE SET google_email=EXCLUDED.google_email,access_token_ciphertext=EXCLUDED.access_token_ciphertext,access_token_nonce=EXCLUDED.access_token_nonce,
		refresh_token_ciphertext=COALESCE(EXCLUDED.refresh_token_ciphertext,google_drive_connections.refresh_token_ciphertext),refresh_token_nonce=COALESCE(EXCLUDED.refresh_token_nonce,google_drive_connections.refresh_token_nonce),
		token_expires_at=EXCLUDED.token_expires_at,scopes=EXCLUDED.scopes,status='active',last_error_code=NULL,connected_at=now(),revoked_at=NULL,version=google_drive_connections.version+1
		RETURNING id,user_id,COALESCE(google_email::text,''),scopes,status,token_expires_at,connected_at,updated_at
	`, connectionID, actor.UserID, email, accessCiphertext, accessNonce, nullableBytes(refreshCiphertext), nullableBytes(refreshNonce), expires, s.config.Scopes).Scan(&result.ID, &result.UserID, &result.GoogleEmail, &result.Scopes, &result.Status, &result.TokenExpires, &result.ConnectedAt, &result.UpdatedAt)
	if err != nil {
		return Connection{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "GOOGLE_DRIVE_CONNECTED", ResourceType: "google_drive_connection", ResourceID: &result.ID, RequestID: actor.RequestID, After: result}); err != nil {
		return Connection{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Connection{}, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, userID uuid.UUID) (Connection, error) {
	var result Connection
	err := s.pool.QueryRow(ctx, `SELECT id,user_id,COALESCE(google_email::text,''),scopes,status,token_expires_at,connected_at,updated_at FROM google_drive_connections WHERE user_id=$1`, userID).Scan(&result.ID, &result.UserID, &result.GoogleEmail, &result.Scopes, &result.Status, &result.TokenExpires, &result.ConnectedAt, &result.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Connection{}, ErrNotConnected
	}
	return result, err
}

func (s *Service) Disconnect(ctx context.Context, actor Actor, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrValidation
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id uuid.UUID
	err = tx.QueryRow(ctx, `UPDATE google_drive_connections SET status='revoked',revoked_at=now(),access_token_ciphertext=NULL,access_token_nonce=NULL,refresh_token_ciphertext=NULL,refresh_token_nonce=NULL,token_expires_at=NULL,version=version+1 WHERE user_id=$1 AND status='active' RETURNING id`, actor.UserID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotConnected
	}
	if err != nil {
		return err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "GOOGLE_DRIVE_DISCONNECTED", ResourceType: "google_drive_connection", ResourceID: &id, RequestID: actor.RequestID, Reason: strings.TrimSpace(reason)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) AccessToken(ctx context.Context, connectionID uuid.UUID) (string, error) {
	if !s.IsConfigured() {
		return "", ErrNotConfigured
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var accessCiphertext, accessNonce, refreshCiphertext, refreshNonce []byte
	var expiry *time.Time
	err = tx.QueryRow(ctx, `SELECT access_token_ciphertext,access_token_nonce,refresh_token_ciphertext,refresh_token_nonce,token_expires_at FROM google_drive_connections WHERE id=$1 AND status='active' FOR UPDATE`, connectionID).Scan(&accessCiphertext, &accessNonce, &refreshCiphertext, &refreshNonce, &expiry)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotConnected
	}
	if err != nil {
		return "", err
	}
	if expiry != nil && expiry.After(s.now().UTC().Add(2*time.Minute)) {
		plain, decryptErr := s.decrypt(accessCiphertext, accessNonce)
		if decryptErr != nil {
			return "", ErrNotConnected
		}
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
		return string(plain), nil
	}
	refresh, err := s.decrypt(refreshCiphertext, refreshNonce)
	if err != nil || len(refresh) == 0 {
		return "", ErrNotConnected
	}
	token, err := s.refresh(ctx, string(refresh))
	if err != nil {
		_, _ = tx.Exec(ctx, `UPDATE google_drive_connections SET status='error',last_error_code='TOKEN_REFRESH_FAILED',revoked_at=now(),version=version+1 WHERE id=$1`, connectionID)
		_ = tx.Commit(ctx)
		return "", err
	}
	ciphertext, nonce, err := s.encrypt([]byte(token.AccessToken))
	if err != nil {
		return "", err
	}
	newExpiry := s.now().UTC().Add(time.Duration(max(token.ExpiresIn, 120)) * time.Second)
	if _, err := tx.Exec(ctx, `UPDATE google_drive_connections SET access_token_ciphertext=$2,access_token_nonce=$3,token_expires_at=$4,last_error_code=NULL,version=version+1 WHERE id=$1`, connectionID, ciphertext, nonce, newExpiry); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func (s *Service) ListFiles(ctx context.Context, userID uuid.UUID, pageToken string) (FilePage, error) {
	connection, err := s.Get(ctx, userID)
	if err != nil || connection.Status != "active" {
		return FilePage{}, ErrNotConnected
	}
	token, err := s.AccessToken(ctx, connection.ID)
	if err != nil {
		return FilePage{}, err
	}
	base, err := url.Parse(strings.TrimRight(s.config.APIBase, "/") + "/v3/files")
	if err != nil {
		return FilePage{}, ErrNotConfigured
	}
	query := base.Query()
	query.Set("pageSize", "100")
	query.Set("orderBy", "modifiedTime desc")
	query.Set("fields", "nextPageToken,files(id,name,mimeType,modifiedTime,webViewLink)")
	query.Set("q", "trashed = false and mimeType = 'application/vnd.google-apps.spreadsheet'")
	if strings.TrimSpace(pageToken) != "" {
		if len(pageToken) > 2048 {
			return FilePage{}, ErrValidation
		}
		query.Set("pageToken", pageToken)
	}
	base.RawQuery = query.Encode()
	var payload struct {
		NextPageToken string `json:"nextPageToken"`
		Files         []struct {
			ID, Name, MimeType, ModifiedTime, WebViewLink string
		} `json:"files"`
	}
	if err := s.getJSON(ctx, base.String(), token, &payload); err != nil {
		return FilePage{}, err
	}
	result := FilePage{Items: []File{}, NextPageToken: payload.NextPageToken}
	for _, item := range payload.Files {
		modified, _ := time.Parse(time.RFC3339, item.ModifiedTime)
		result.Items = append(result.Items, File{ID: item.ID, Name: item.Name, MimeType: item.MimeType, ModifiedTime: modified, WebViewLink: item.WebViewLink})
	}
	return result, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *Service) exchangeCode(ctx context.Context, code, verifier string) (tokenResponse, error) {
	return s.postToken(ctx, url.Values{"client_id": {s.config.ClientID}, "client_secret": {s.config.ClientSecret}, "code": {code}, "code_verifier": {verifier}, "grant_type": {"authorization_code"}, "redirect_uri": {s.config.RedirectURL}})
}

func (s *Service) refresh(ctx context.Context, refreshToken string) (tokenResponse, error) {
	return s.postToken(ctx, url.Values{"client_id": {s.config.ClientID}, "client_secret": {s.config.ClientSecret}, "refresh_token": {refreshToken}, "grant_type": {"refresh_token"}})
}

func (s *Service) postToken(ctx context.Context, form url.Values) (tokenResponse, error) {
	requestCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, s.config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.http.Do(request)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("%w: token exchange", ErrUnavailable)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 128<<10))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("%w: token HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var token tokenResponse
	if json.Unmarshal(body, &token) != nil || strings.TrimSpace(token.AccessToken) == "" {
		return tokenResponse{}, fmt.Errorf("%w: invalid token response", ErrUnavailable)
	}
	return token, nil
}

func (s *Service) userEmail(ctx context.Context, token string) (string, error) {
	var payload struct {
		Email string `json:"email"`
	}
	if err := s.getJSON(ctx, s.config.UserInfoURL, token, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Email), nil
}

func (s *Service) getJSON(ctx context.Context, endpoint, token string, destination any) error {
	requestCtx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := s.http.Do(request)
	if err != nil {
		return fmt.Errorf("%w: request failed", ErrUnavailable)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, s.config.MaxBytes+1))
	if err != nil || int64(len(body)) > s.config.MaxBytes || response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("%w: HTTP %d", ErrUnavailable, response.StatusCode)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: invalid JSON", ErrUnavailable)
	}
	return nil
}

func (s *Service) encrypt(plaintext []byte) ([]byte, []byte, error) {
	if s.aead == nil {
		return nil, nil, ErrNotConfigured
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return s.aead.Seal(nil, nonce, plaintext, []byte("domain-monitor-google-oauth-v1")), nonce, nil
}

func (s *Service) decrypt(ciphertext, nonce []byte) ([]byte, error) {
	if s.aead == nil || len(ciphertext) == 0 || len(nonce) != s.aead.NonceSize() {
		return nil, ErrNotConfigured
	}
	return s.aead.Open(nil, nonce, ciphertext, []byte("domain-monitor-google-oauth-v1"))
}

func randomURLSafe(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
