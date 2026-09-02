//go:build integration

package integration

import (
	"bytes"
	"context"
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
	"domainmonitor/internal/finance"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/platform/rediscache"
	"domainmonitor/internal/report"
	"github.com/google/uuid"
)

// TestReportDashboardAndPDFWorkflow exercises the visual report feature end
// to end: the live GET /reports/dashboard view, and creating + downloading a
// PDF snapshot of the same data (status distributions, 30-day incident
// trend, top-10 domains needing attention) through POST /reports.
func TestReportDashboardAndPDFWorkflow(t *testing.T) {
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

	userID := uuid.New()
	passwordHash, err := auth.HashPassword("report-dashboard-" + unique)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,display_name,password_hash,locale) VALUES ($1,$2,'Report Dashboard Admin',$3,'th')`, userID, "report-dashboard-"+unique+"@example.internal", passwordHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id,role_id,granted_by) SELECT $1,id,$1 FROM roles WHERE code='ADMIN'`, userID); err != nil {
		t.Fatal(err)
	}

	activeDomain := "report-dash-active-" + unique + ".com"
	blockedDomain := "report-dash-blocked-" + unique + ".com"
	var activeDomainID, blockedDomainID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO domains (original_input,domain_ascii,domain_unicode,registrable_domain,source_type,expected_content_mode,
			current_availability_status,current_dns_status,current_http_status,current_redirect_status,current_isp_status,current_tls_status)
		VALUES ($1,$1,$1,$1,'manual','HTML','ACTIVE','OK','OK','NONE','NOT_DETECTED','VALID') RETURNING id
	`, activeDomain).Scan(&activeDomainID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO domains (original_input,domain_ascii,domain_unicode,registrable_domain,source_type,expected_content_mode,
			current_availability_status,current_dns_status,current_http_status,current_redirect_status,current_isp_status,current_tls_status)
		VALUES ($1,$1,$1,$1,'manual','HTML','UNAVAILABLE','OK','CONNECTION_ERROR','NONE','HIGH_CONFIDENCE_BLOCK','UNKNOWN') RETURNING id
	`, blockedDomain).Scan(&blockedDomainID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO domain_costs (domain_id,cost_type,amount,currency_code,price_source,effective_from)
		VALUES ($1,'renewal',1234.565,'THB','manual',current_date)
	`, blockedDomainID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `UPDATE monitor_schedules SET enabled=false WHERE domain_id IN ($1,$2)`, activeDomainID, blockedDomainID)
	}()

	var runID uuid.UUID
	now := time.Now().UTC()
	if err := pool.QueryRow(ctx, `
		INSERT INTO monitoring_runs (domain_id,trigger_type,status,priority,deduplication_key,policy_version,policy_snapshot,scheduled_for,deadline_at,started_at,completed_at)
		VALUES ($1,'scheduled','completed','normal',$2,$3,'{}'::jsonb,$4,$5,$4,$4) RETURNING id
	`, blockedDomainID, "report-dash-run:"+unique, cfg.MonitorPolicyVersion, now.Add(-time.Minute), now.Add(time.Minute)).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO incidents (domain_id,status,open_failure_count,opened_at,opened_by_run_id)
		VALUES ($1,'open',3,now(),$2)
	`, blockedDomainID, runID); err != nil {
		t.Fatal(err)
	}

	auditStore := audit.NewStore()
	financeService := finance.NewService(pool, auditStore, 48*time.Hour)
	reportService := report.NewService(pool, auditStore, financeService)

	dashboard, err := reportService.Dashboard(ctx, "THB")
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.TotalDomains < 2 {
		t.Fatalf("dashboard total domains = %d, want >= 2", dashboard.TotalDomains)
	}
	if !hasStatusCount(dashboard.AvailabilityDistribution, "ACTIVE") || !hasStatusCount(dashboard.AvailabilityDistribution, "UNAVAILABLE") {
		t.Fatalf("availability distribution missing expected statuses: %#v", dashboard.AvailabilityDistribution)
	}
	if !hasStatusCount(dashboard.ISPDistribution, "HIGH_CONFIDENCE_BLOCK") {
		t.Fatalf("isp distribution missing HIGH_CONFIDENCE_BLOCK: %#v", dashboard.ISPDistribution)
	}
	if len(dashboard.IncidentTrend30d) != 30 {
		t.Fatalf("incident trend length = %d, want 30 (zero-filled)", len(dashboard.IncidentTrend30d))
	}
	if dashboard.IncidentTrend30d[29].Count < 1 {
		t.Fatalf("incident trend today = %#v, want at least the incident just opened", dashboard.IncidentTrend30d[29])
	}
	foundBlockedInTop := false
	for _, item := range dashboard.TopDomains {
		if item.Domain == blockedDomain {
			foundBlockedInTop = true
			if item.RenewalCost == nil || *item.RenewalCost != "1234.565000" {
				t.Fatalf("top domain renewal cost = %v, want 1234.565000", item.RenewalCost)
			}
		}
	}
	if !foundBlockedInTop {
		t.Fatalf("top domains should surface the unavailable+blocked domain: %#v", dashboard.TopDomains)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := app.NewAPIHandler(cfg, logger, pool, redisClient)
	loginResponse := performJSON(t, handler, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email": "report-dashboard-" + unique + "@example.internal", "password": "report-dashboard-" + unique,
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

	dashboardResponse := performJSON(t, handler, http.MethodGet, "/api/v1/reports/dashboard?reporting_currency=THB", nil, cookies, map[string]string{"Accept-Language": "en"})
	if dashboardResponse.Code != http.StatusOK {
		t.Fatalf("dashboard endpoint status = %d, body = %s", dashboardResponse.Code, dashboardResponse.Body.String())
	}
	if !strings.Contains(dashboardResponse.Body.String(), `"top_domains"`) || !strings.Contains(dashboardResponse.Body.String(), `"incident_trend_30d"`) {
		t.Fatalf("dashboard response missing expected fields: %s", dashboardResponse.Body.String())
	}

	createHeaders := map[string]string{"X-CSRF-Token": loginBody.Data.CSRFToken, "Idempotency-Key": "report-pdf-" + unique}
	createResponse := performJSON(t, handler, http.MethodPost, "/api/v1/reports", map[string]any{"format": "pdf", "reporting_currency": "THB"}, cookies, createHeaders)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create pdf report status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createdBody struct {
		Data struct {
			ID         uuid.UUID `json:"id"`
			ReportType string    `json:"report_type"`
			Format     string    `json:"format"`
		} `json:"data"`
	}
	decodeResponse(t, createResponse, &createdBody)
	if createdBody.Data.Format != "pdf" || createdBody.Data.ReportType != "dashboard" {
		t.Fatalf("created report = %#v", createdBody.Data)
	}

	downloadRequest := performJSON(t, handler, http.MethodGet, fmt.Sprintf("/api/v1/reports/%s/download", createdBody.Data.ID), nil, cookies, map[string]string{"Accept-Language": "en"})
	if downloadRequest.Code != http.StatusOK {
		t.Fatalf("download pdf status = %d", downloadRequest.Code)
	}
	if contentType := downloadRequest.Header().Get("Content-Type"); contentType != "application/pdf" {
		t.Fatalf("download content-type = %q, want application/pdf", contentType)
	}
	disposition := downloadRequest.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "domain-dashboard-") || !strings.HasSuffix(disposition, ".pdf") {
		t.Fatalf("download content-disposition = %q", disposition)
	}
	body := downloadRequest.Body.Bytes()
	if !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("downloaded content does not start with the PDF magic bytes: %q", body[:min(16, len(body))])
	}
	if len(body) < 500 {
		t.Fatalf("downloaded PDF is suspiciously small: %d bytes", len(body))
	}
}

func hasStatusCount(counts []report.StatusCount, status string) bool {
	for _, item := range counts {
		if item.Status == status {
			return true
		}
	}
	return false
}
