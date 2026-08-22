//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"domainmonitor/internal/app"
	"domainmonitor/internal/audit"
	"domainmonitor/internal/auth"
	"domainmonitor/internal/classification"
	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/httpcheck"
	"domainmonitor/internal/monitor"
	"domainmonitor/internal/netcheck"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/platform/rediscache"
	"domainmonitor/internal/queue"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPhase3DurableMonitoringWorkflow(t *testing.T) {
	unique := strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
	stream := "integration:phase3:" + unique
	t.Setenv("OUTBOX_STREAM", stream)
	t.Setenv("MONITOR_RUN_TIMEOUT", "30s")
	t.Setenv("SCHEDULER_BATCH_SIZE", "10")
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

	email := "phase3-" + unique + "@example.internal"
	password := "integration-password-" + unique
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, password_hash, locale)
		VALUES ($1,$2,'Phase 3 Admin',$3,'th')
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
	domainName := "phase3-" + unique + ".com"
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
		_, _ = redisClient.Del(context.Background(), stream).Result()
	}()

	var scheduleCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM monitor_schedules WHERE domain_id = $1 AND enabled = true`, domainID).Scan(&scheduleCount); err != nil {
		t.Fatal(err)
	}
	if scheduleCount != 1 {
		t.Fatalf("enabled monitor schedules = %d, want 1", scheduleCount)
	}

	missingKeyResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/domains/%s/check", domainID), map[string]any{}, cookies, headers)
	if missingKeyResponse.Code != http.StatusUnprocessableEntity || !strings.Contains(missingKeyResponse.Body.String(), `"messages":{"th":`) {
		t.Fatalf("missing idempotency response = %d, body = %s", missingKeyResponse.Code, missingKeyResponse.Body.String())
	}
	manualHeaders := map[string]string{
		"X-CSRF-Token": loginBody.Data.CSRFToken, "Accept-Language": "en", "Idempotency-Key": "phase3-" + unique,
	}
	manualResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/domains/%s/check", domainID), map[string]any{}, cookies, manualHeaders)
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
	if !manualBody.Data.Created || manualBody.Data.Run.Status != "queued" {
		t.Fatalf("manual run = %#v", manualBody.Data)
	}
	repeatedResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/domains/%s/check", domainID), map[string]any{}, cookies, manualHeaders)
	var repeatedBody struct {
		Data struct {
			Run     monitor.Run `json:"run"`
			Created bool        `json:"created"`
		} `json:"data"`
	}
	decodeResponse(t, repeatedResponse, &repeatedBody)
	if repeatedBody.Data.Created || repeatedBody.Data.Run.ID != manualBody.Data.Run.ID {
		t.Fatalf("idempotent run mismatch: first=%#v repeated=%#v", manualBody.Data, repeatedBody.Data)
	}

	dispatched, err := queue.NewDispatcher(pool, redisClient, 10, 5*time.Second, 3).RunOnce(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dispatched < 1 {
		t.Fatalf("dispatched jobs = %d", dispatched)
	}
	consumed := make(chan struct{}, 1)
	consumerCtx, consumerCancel := context.WithCancel(ctx)
	consumer := queue.NewConsumer(redisClient, stream, "phase3-group-"+unique, "phase3-consumer-"+unique, 1, time.Second, 100*time.Millisecond, func(err error) {
		t.Logf("consumer error: %v", err)
	})
	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- consumer.Run(consumerCtx, func(_ context.Context, values map[string]any) error {
			var job monitor.Job
			if err := json.Unmarshal([]byte(fmt.Sprint(values["payload"])), &job); err != nil {
				return err
			}
			if job.RunID != manualBody.Data.Run.ID.String() {
				return fmt.Errorf("run_id = %s", job.RunID)
			}
			consumed <- struct{}{}
			return nil
		})
	}()
	select {
	case <-consumed:
		consumerCancel()
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := <-consumerDone; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE monitoring_runs SET status = 'cancelled', cancelled_at = now() WHERE id = $1`, manualBody.Data.Run.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `UPDATE monitor_schedules SET next_due_at = now() - interval '1 second' WHERE domain_id = $1`, domainID); err != nil {
		t.Fatal(err)
	}
	service := monitor.NewService(pool, audit.NewStore(), cfg)
	scheduled, err := service.ScheduleDue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if scheduled < 1 {
		t.Fatalf("scheduled runs = %d, want at least 1", scheduled)
	}
	var scheduledRunID uuid.UUID
	if err := pool.QueryRow(ctx, `
		SELECT id FROM monitoring_runs WHERE domain_id = $1 AND trigger_type = 'scheduled'
		ORDER BY created_at DESC LIMIT 1
	`, domainID).Scan(&scheduledRunID); err != nil {
		t.Fatal(err)
	}
	execution, err := service.ClaimExecution(ctx, scheduledRunID, "integration-worker")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	localDNS := []dnscheck.Result{{
		Resolver: dnscheck.ResolverLocalSystem, Endpoint: "fixture", QueryName: domainName + ".", QueryType: dnscheck.TypeA,
		Attempt: 1, RCode: "NOERROR", Answers: []dnscheck.Answer{{Name: domainName + ".", Type: dnscheck.TypeA, Value: "203.0.113.10", TTL: 60}}, CheckedAt: now,
	}}
	alternateDNS := []dnscheck.Result{{
		Resolver: dnscheck.ResolverCloudflareDoH, Endpoint: "https://fixture.invalid/dns-query", QueryName: domainName + ".", QueryType: dnscheck.TypeA,
		Attempt: 1, RCode: "NOERROR", Answers: []dnscheck.Answer{{Name: domainName + ".", Type: dnscheck.TypeA, Value: "203.0.113.10", TTL: 60}}, CheckedAt: now,
	}}
	bodyHash := sha256.Sum256([]byte("phase 3 deterministic content"))
	origin := httpcheck.OriginResult{HTTPS: []httpcheck.Attempt{{
		Attempt: 1, RequestURL: "https://" + domainName + "/", EffectiveURL: "https://" + domainName + "/",
		Protocol: "HTTP/2.0", InitialStatusCode: 200, FinalStatusCode: 200, Redirects: []httpcheck.RedirectHop{},
		Timing:  httpcheck.Timing{Total: 125 * time.Millisecond},
		Content: httpcheck.ContentEvidence{Status: "VALID_HTML", ContentType: "text/html", DeclaredContentLength: 128, BodySize: 128, BodySHA256: bodyHash[:], HashComplete: true, Title: "Phase 3"},
		Headers: map[string]string{"Content-Type": "text/html"}, CheckedAt: now,
	}}}
	decision := classification.Classify(localDNS, alternateDNS, origin)
	if err := service.CompleteRun(ctx, execution, monitor.Evidence{
		LocalDNS: localDNS, AlternateDNS: alternateDNS, HTTP: origin, Decision: decision, CheckedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	var runStatus, availability string
	var resultCount, dnsCount, httpCount, historyCount int
	if err := pool.QueryRow(ctx, `SELECT status::text FROM monitoring_runs WHERE id = $1`, scheduledRunID).Scan(&runStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT current_availability_status::text FROM domains WHERE id = $1`, domainID).Scan(&availability); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM monitoring_results WHERE monitoring_run_id = $1`, scheduledRunID).Scan(&resultCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM dns_checks WHERE monitoring_result_id IN (SELECT id FROM monitoring_results WHERE monitoring_run_id = $1)`, scheduledRunID).Scan(&dnsCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM http_checks WHERE monitoring_result_id IN (SELECT id FROM monitoring_results WHERE monitoring_run_id = $1)`, scheduledRunID).Scan(&httpCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM domain_status_history WHERE domain_id = $1`, domainID).Scan(&historyCount); err != nil {
		t.Fatal(err)
	}
	if runStatus != "completed" || availability != "ACTIVE" || resultCount != 1 || dnsCount != 2 || httpCount != 1 || historyCount == 0 {
		t.Fatalf("persistence run=%s availability=%s result=%d dns=%d http=%d history=%d", runStatus, availability, resultCount, dnsCount, httpCount, historyCount)
	}

	for index := 0; index < 3; index++ {
		failureOrigin := httpcheck.OriginResult{HTTPS: []httpcheck.Attempt{{
			Attempt: 1, RequestURL: "https://" + domainName + "/", EffectiveURL: "https://" + domainName + "/",
			Protocol: "HTTP/2.0", InitialStatusCode: 500, FinalStatusCode: 500, Redirects: []httpcheck.RedirectHop{},
			Timing:  httpcheck.Timing{Total: 100 * time.Millisecond},
			Content: httpcheck.ContentEvidence{Status: "VALID_HTML", ContentType: "text/html", DeclaredContentLength: 128, BodySize: 128, BodySHA256: bodyHash[:], HashComplete: true},
			Headers: map[string]string{"Content-Type": "text/html"}, ErrorCode: netcheck.HTTPServerError,
			ErrorMessage: "HTTP server returned a 5xx response", CheckedAt: time.Now().UTC(),
		}}}
		executeSyntheticScheduledRun(t, ctx, pool, service, domainID, localDNS, alternateDNS, failureOrigin)
	}
	var incidentID uuid.UUID
	var incidentStatus string
	var openFailureCount int
	if err := pool.QueryRow(ctx, `
		SELECT id, status::text, open_failure_count FROM incidents
		WHERE domain_id = $1 ORDER BY opened_at DESC LIMIT 1
	`, domainID).Scan(&incidentID, &incidentStatus, &openFailureCount); err != nil {
		t.Fatal(err)
	}
	if incidentStatus != "open" || openFailureCount != 3 {
		t.Fatalf("opened incident status=%s failures=%d", incidentStatus, openFailureCount)
	}
	for index := 0; index < 2; index++ {
		recoveryOrigin := origin
		recoveryOrigin.HTTPS[0].CheckedAt = time.Now().UTC()
		executeSyntheticScheduledRun(t, ctx, pool, service, domainID, localDNS, alternateDNS, recoveryOrigin)
	}
	var closeSuccessCount int
	var closedAt *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status::text, close_success_count, closed_at FROM incidents WHERE id = $1
	`, incidentID).Scan(&incidentStatus, &closeSuccessCount, &closedAt); err != nil {
		t.Fatal(err)
	}
	if incidentStatus != "closed" || closeSuccessCount != 2 || closedAt == nil {
		t.Fatalf("closed incident status=%s successes=%d closed_at=%v", incidentStatus, closeSuccessCount, closedAt)
	}

	runResponse := performJSON(t, handler, http.MethodGet, "/api/v1/monitoring-runs/"+scheduledRunID.String(), nil, cookies, map[string]string{"Accept-Language": "en"})
	if runResponse.Code != http.StatusOK || !strings.Contains(runResponse.Body.String(), `"dns_checks"`) || !strings.Contains(runResponse.Body.String(), `"VALID_HTML"`) {
		t.Fatalf("run detail status = %d, body = %s", runResponse.Code, runResponse.Body.String())
	}
	historyResponse := performJSON(t, handler, http.MethodGet, fmt.Sprintf("/api/v1/domains/%s/monitoring-history?window=24h", domainID), nil, cookies, nil)
	if historyResponse.Code != http.StatusOK || !strings.Contains(historyResponse.Body.String(), `"uptime_percentage"`) {
		t.Fatalf("history status = %d, body = %s", historyResponse.Code, historyResponse.Body.String())
	}
}

func executeSyntheticScheduledRun(
	t *testing.T,
	ctx context.Context,
	pool interface {
		Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	},
	service *monitor.Service,
	domainID uuid.UUID,
	localDNS []dnscheck.Result,
	alternateDNS []dnscheck.Result,
	origin httpcheck.OriginResult,
) {
	t.Helper()
	runID := uuid.New()
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO monitoring_runs (
			id, domain_id, trigger_type, status, priority, deduplication_key,
			policy_version, policy_snapshot, scheduled_for, deadline_at
		) VALUES ($1,$2,'scheduled','queued','normal',$3,'monitor-2026-08-v1','{}'::jsonb,$4,$5)
	`, runID, domainID, "integration-hysteresis:"+runID.String(), now, now.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	execution, err := service.ClaimExecution(ctx, runID, "integration-hysteresis-worker")
	if err != nil {
		t.Fatal(err)
	}
	decision := classification.Classify(localDNS, alternateDNS, origin)
	if err := service.CompleteRun(ctx, execution, monitor.Evidence{
		LocalDNS: localDNS, AlternateDNS: alternateDNS, HTTP: origin, Decision: decision, CheckedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
}
