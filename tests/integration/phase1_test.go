//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"domainmonitor/internal/app"
	"domainmonitor/internal/auth"
	"domainmonitor/internal/config"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/platform/rediscache"
	"domainmonitor/internal/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestPhase1SchemaAndAPI(t *testing.T) {
	cfg := testConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := postgres.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	redisClient, err := rediscache.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = redisClient.Close() }()

	requiredTables := []string{
		"domains", "registrars", "registrar_prices", "domain_costs", "monitoring_runs",
		"monitoring_results", "dns_checks", "http_checks", "tls_checks", "redirect_hops",
		"remote_probe_results", "domain_status_history", "recommendations", "google_sheet_imports",
		"reports", "system_settings", "audit_logs", "job_outbox",
	}
	for _, table := range requiredTables {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Errorf("required table %s does not exist", table)
		}
	}

	unique := strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
	email := "admin-" + unique + "@example.internal"
	password := "integration-password-" + unique
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, password_hash, locale)
		VALUES ($1, $2, 'Integration Admin', $3, 'th')
	`, userID, email, passwordHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, granted_by)
		SELECT $1, id, $1 FROM roles WHERE code = 'ADMIN'
	`, userID); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := app.NewAPIHandler(cfg, logger, pool, redisClient)
	loginResponse := performJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": email, "password": password,
	}, nil, map[string]string{"Accept-Language": "en"})
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	var loginBody struct {
		Data struct {
			CSRFToken string `json:"csrf_token"`
		} `json:"data"`
	}
	decodeResponse(t, loginResponse, &loginBody)
	if loginBody.Data.CSRFToken == "" {
		t.Fatal("login did not return a CSRF token")
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %d", len(cookies))
	}
	headers := map[string]string{"X-CSRF-Token": loginBody.Data.CSRFToken, "Accept-Language": "th"}
	domainName := "integration-" + unique + ".com"
	createResponse := performJSON(t, handler, http.MethodPost, "/api/v1/domains", map[string]any{
		"domain": domainName, "business_priority": "high", "monitoring_enabled": true,
	}, cookies, headers)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createdBody struct {
		Data struct {
			ID              uuid.UUID `json:"id"`
			ASCII           string    `json:"domain_ascii"`
			LifecycleStatus string    `json:"lifecycle_status"`
			Version         int64     `json:"version"`
		} `json:"data"`
	}
	decodeResponse(t, createResponse, &createdBody)
	defer func() {
		_, _ = pool.Exec(context.Background(), `UPDATE monitor_schedules SET enabled = false WHERE domain_id = $1`, createdBody.Data.ID)
	}()
	if createdBody.Data.ASCII != domainName || createdBody.Data.Version != 1 {
		t.Fatalf("unexpected created domain: %#v", createdBody.Data)
	}

	duplicateResponse := performJSON(t, handler, http.MethodPost, "/api/v1/domains", map[string]any{
		"domain": "https://" + strings.ToUpper(domainName) + "/",
	}, cookies, headers)
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicateResponse.Code, duplicateResponse.Body.String())
	}
	if !strings.Contains(duplicateResponse.Body.String(), `"messages":{"th":`) || !strings.Contains(duplicateResponse.Body.String(), `"en":`) {
		t.Fatalf("duplicate error is not bilingual: %s", duplicateResponse.Body.String())
	}

	invalidResponse := performJSON(t, handler, http.MethodPost, "/api/v1/domains", map[string]any{
		"domain": "127.0.0.1",
	}, cookies, headers)
	if invalidResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid domain status = %d, body = %s", invalidResponse.Code, invalidResponse.Body.String())
	}
	invalidBody := invalidResponse.Body.String()
	if !strings.Contains(invalidBody, `"code":"INVALID_DOMAIN"`) ||
		!strings.Contains(invalidBody, `"reason_code":"DOMAIN_INVALID"`) ||
		!strings.Contains(invalidBody, `"messages":{"th":`) ||
		!strings.Contains(invalidBody, `"en":`) {
		t.Fatalf("invalid domain error lacks stable bilingual details: %s", invalidBody)
	}

	archiveResponse := performJSON(t, handler, http.MethodPost,
		fmt.Sprintf("/api/v1/domains/%s/archive", createdBody.Data.ID),
		map[string]any{"version": createdBody.Data.Version, "reason": "integration test"}, cookies, headers)
	if archiveResponse.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body = %s", archiveResponse.Code, archiveResponse.Body.String())
	}
	var archivedBody struct {
		Data struct {
			LifecycleStatus   string `json:"lifecycle_status"`
			MonitoringEnabled bool   `json:"monitoring_enabled"`
			Version           int64  `json:"version"`
		} `json:"data"`
	}
	decodeResponse(t, archiveResponse, &archivedBody)
	if archivedBody.Data.LifecycleStatus != "archived" || archivedBody.Data.MonitoringEnabled {
		t.Fatalf("unexpected archived domain: %#v", archivedBody.Data)
	}

	restoreResponse := performJSON(t, handler, http.MethodPost,
		fmt.Sprintf("/api/v1/domains/%s/restore", createdBody.Data.ID),
		map[string]any{"version": archivedBody.Data.Version, "reason": "integration test restore"}, cookies, headers)
	if restoreResponse.Code != http.StatusOK {
		t.Fatalf("restore status = %d, body = %s", restoreResponse.Code, restoreResponse.Body.String())
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE resource_id = $1`, createdBody.Data.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 3 {
		t.Fatalf("domain audit count = %d, want 3", auditCount)
	}
	assertAuditChain(t, ctx, pool)

	if _, err := pool.Exec(ctx, `UPDATE audit_logs SET action = action WHERE resource_id = $1`, createdBody.Data.ID); err == nil {
		t.Fatal("audit log update succeeded; append-only trigger did not reject mutation")
	}
}

func TestOutboxDispatchesToRedis(t *testing.T) {
	cfg := testConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := postgres.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	redisClient, err := rediscache.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = redisClient.Close() }()

	stream := "integration:outbox:" + uuid.NewString()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	outboxID, err := queue.NewStore(pool).EnqueueTx(ctx, tx, "integration-"+uuid.NewString(), "integration.test", stream, map[string]any{"ok": true})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	dispatched, err := queue.NewDispatcher(pool, redisClient, 10, 5*time.Second, 3).RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched < 1 {
		t.Fatalf("dispatched = %d", dispatched)
	}
	messages, err := redisClient.XRange(ctx, stream, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Values["job_id"] != outboxID.String() {
		t.Fatalf("unexpected stream messages: %#v", messages)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM job_outbox WHERE id = $1`, outboxID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "dispatched" {
		t.Fatalf("outbox status = %q", status)
	}
}

func testConfig(t *testing.T) config.Config {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("REDIS_ADDR") == "" {
		t.Skip("DATABASE_URL and REDIS_ADDR are required for integration tests")
	}
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	t.Setenv("APP_ENV", "test")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func performJSON(t *testing.T, handler http.Handler, method, path string, payload any, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}

func assertAuditChain(t *testing.T, ctx context.Context, pool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT prev_hash, entry_hash FROM audit_logs ORDER BY created_at, id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var previousEntryHash []byte
	index := 0
	for rows.Next() {
		var previousHash, entryHash []byte
		if err := rows.Scan(&previousHash, &entryHash); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(previousHash, previousEntryHash) {
			t.Fatalf("audit chain broken at entry %d", index)
		}
		if len(entryHash) != 32 {
			t.Fatalf("audit entry %d hash length = %d, want 32", index, len(entryHash))
		}
		previousEntryHash = append(previousEntryHash[:0], entryHash...)
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index == 0 {
		t.Fatal("audit chain is empty")
	}
}
