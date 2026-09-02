//go:build integration

package integration

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"domainmonitor/internal/app"
	"domainmonitor/internal/auth"
	"domainmonitor/internal/monitor"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/platform/rediscache"
	"github.com/google/uuid"
)

// TestForceISPCheckWorkflow exercises the "force ISP check" button's backend
// path: POST /domains/{id}/isp-check queues a fresh manual monitoring run
// exactly like the plain recheck endpoint, but is idempotent independently of
// it and is recorded in the hash-chained audit log under a distinct action
// (DOMAIN_ISP_CHECK_FORCED) so operators can tell the two apart.
func TestForceISPCheckWorkflow(t *testing.T) {
	unique := strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
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

	email := "isp-check-" + unique + "@example.internal"
	password := "integration-password-" + unique
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, password_hash, locale)
		VALUES ($1,$2,'ISP Check Admin',$3,'th')
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
	cookies := loginResponse.Result().Cookies()
	headers := map[string]string{"X-CSRF-Token": loginBody.Data.CSRFToken, "Accept-Language": "th"}
	domainName := "isp-check-" + unique + ".com"
	createResponse := performJSON(t, handler, http.MethodPost, "/api/v1/domains", map[string]any{
		"domain": domainName, "monitoring_enabled": true,
	}, cookies, headers)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create domain status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	var created struct {
		Data struct {
			ID uuid.UUID `json:"id"`
		} `json:"data"`
	}
	decodeResponse(t, createResponse, &created)
	domainID := created.Data.ID
	defer func() {
		_, _ = pool.Exec(context.Background(), `UPDATE monitor_schedules SET enabled = false WHERE domain_id = $1`, domainID)
	}()

	missingKeyResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/domains/%s/isp-check", domainID), map[string]any{}, cookies, headers)
	if missingKeyResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing idempotency response = %d, body = %s", missingKeyResponse.Code, missingKeyResponse.Body.String())
	}

	ispHeaders := map[string]string{
		"X-CSRF-Token": loginBody.Data.CSRFToken, "Accept-Language": "en", "Idempotency-Key": "isp-check-" + unique,
	}
	ispResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/domains/%s/isp-check", domainID), map[string]any{}, cookies, ispHeaders)
	if ispResponse.Code != http.StatusAccepted {
		t.Fatalf("isp check status = %d, body = %s", ispResponse.Code, ispResponse.Body.String())
	}
	var ispBody struct {
		Data struct {
			Run     monitor.Run `json:"run"`
			Created bool        `json:"created"`
		} `json:"data"`
	}
	decodeResponse(t, ispResponse, &ispBody)
	if !ispBody.Data.Created || ispBody.Data.Run.Status != "queued" {
		t.Fatalf("isp check run = %#v", ispBody.Data)
	}

	// Repeating the same Idempotency-Key returns the same run without creating a second one.
	repeatedResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/domains/%s/isp-check", domainID), map[string]any{}, cookies, ispHeaders)
	var repeatedBody struct {
		Data struct {
			Run     monitor.Run `json:"run"`
			Created bool        `json:"created"`
		} `json:"data"`
	}
	decodeResponse(t, repeatedResponse, &repeatedBody)
	if repeatedBody.Data.Created || repeatedBody.Data.Run.ID != ispBody.Data.Run.ID {
		t.Fatalf("idempotent isp check run mismatch: first=%#v repeated=%#v", ispBody.Data, repeatedBody.Data)
	}

	// A distinct Idempotency-Key on the plain manual-check endpoint queues an
	// independent run rather than colliding with the ISP-check run above.
	manualResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/domains/%s/check", domainID), map[string]any{}, cookies, map[string]string{
		"X-CSRF-Token": loginBody.Data.CSRFToken, "Accept-Language": "en", "Idempotency-Key": "manual-check-" + unique,
	})
	if manualResponse.Code != http.StatusAccepted {
		t.Fatalf("manual check status = %d, body = %s", manualResponse.Code, manualResponse.Body.String())
	}
	var manualBody struct {
		Data struct {
			Run     monitor.Run `json:"run"`
			Created bool        `json:"created"`
		} `json:"data"`
	}
	decodeResponse(t, manualResponse, &manualBody)
	if !manualBody.Data.Created || manualBody.Data.Run.ID == ispBody.Data.Run.ID {
		t.Fatalf("manual check run should be independent of the isp check run: %#v", manualBody.Data)
	}

	var ispAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_logs
		WHERE action = 'DOMAIN_ISP_CHECK_FORCED' AND resource_type = 'monitoring_run' AND resource_id = $1
	`, ispBody.Data.Run.ID).Scan(&ispAuditCount); err != nil {
		t.Fatal(err)
	}
	if ispAuditCount != 1 {
		t.Fatalf("DOMAIN_ISP_CHECK_FORCED audit rows for isp check run = %d, want 1", ispAuditCount)
	}
	var manualAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_logs
		WHERE action = 'DOMAIN_MANUAL_CHECK_REQUESTED' AND resource_type = 'monitoring_run' AND resource_id = $1
	`, manualBody.Data.Run.ID).Scan(&manualAuditCount); err != nil {
		t.Fatal(err)
	}
	if manualAuditCount != 1 {
		t.Fatalf("DOMAIN_MANUAL_CHECK_REQUESTED audit rows for manual check run = %d, want 1", manualAuditCount)
	}

	assertAuditChain(t, ctx, pool)

	if _, err := pool.Exec(ctx, `UPDATE monitoring_runs SET status = 'cancelled', cancelled_at = now() WHERE id IN ($1, $2)`, ispBody.Data.Run.ID, manualBody.Data.Run.ID); err != nil {
		t.Fatal(err)
	}
}
