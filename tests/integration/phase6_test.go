//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/auth"
	"domainmonitor/internal/domain"
	"domainmonitor/internal/drive"
	"domainmonitor/internal/finance"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/recommendation"
	"domainmonitor/internal/report"
	"domainmonitor/internal/sheets"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

func TestDriveExcelRecommendationAndReportWorkflow(t *testing.T) {
	cfg := testConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := postgres.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, table := range []string{"google_drive_connections", "google_oauth_states", "report_payloads"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists); err != nil || !exists {
			t.Fatalf("Phase 6 table %s missing: exists=%v err=%v", table, exists, err)
		}
	}

	unique := strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
	userID := uuid.New()
	passwordHash, err := auth.HashPassword("phase6-integration-" + unique)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,display_name,password_hash,locale) VALUES ($1,$2,'Phase 6 Admin',$3,'th')`, userID, "phase6-"+unique+"@example.internal", passwordHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id,role_id,granted_by) SELECT $1,id,$1 FROM roles WHERE code='ADMIN'`, userID); err != nil {
		t.Fatal(err)
	}
	auditStore := audit.NewStore()

	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fixture-access-token", "refresh_token": "fixture-refresh-token", "expires_in": 3600})
		case "/userinfo":
			_ = json.NewEncoder(w).Encode(map[string]any{"email": "owner@example.internal"})
		case "/drive/v3/files":
			if r.Header.Get("Authorization") != "Bearer fixture-access-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"files": []map[string]string{{"id": "sheet-1", "name": "Domains", "mimeType": "application/vnd.google-apps.spreadsheet", "modifiedTime": "2026-08-22T00:00:00Z"}}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer fixture.Close()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	driveService := drive.NewService(pool, auditStore, fixture.Client(), drive.Config{APIBase: fixture.URL + "/drive", AuthorizationURL: fixture.URL + "/auth", TokenURL: fixture.URL + "/token", UserInfoURL: fixture.URL + "/userinfo", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://example.internal/callback", EncryptionKey: key, Scopes: []string{"openid", "email", "drive.file"}, Timeout: 3 * time.Second})
	authorization, err := driveService.Begin(ctx, drive.Actor{UserID: userID, RequestID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	authorizationURL, _ := url.Parse(authorization.AuthorizationURL)
	connection, err := driveService.Complete(ctx, drive.Actor{UserID: userID, RequestID: uuid.NewString()}, authorizationURL.Query().Get("state"), "fixture-code")
	if err != nil || connection.GoogleEmail != "owner@example.internal" {
		t.Fatalf("OAuth complete: %#v err=%v", connection, err)
	}
	var encrypted []byte
	if err := pool.QueryRow(ctx, `SELECT access_token_ciphertext FROM google_drive_connections WHERE id=$1`, connection.ID).Scan(&encrypted); err != nil || bytes.Contains(encrypted, []byte("fixture-access-token")) {
		t.Fatalf("OAuth token was not encrypted: err=%v", err)
	}
	files, err := driveService.ListFiles(ctx, userID, "")
	if err != nil || len(files.Items) != 1 || files.Items[0].ID != "sheet-1" {
		t.Fatalf("Drive files: %#v err=%v", files, err)
	}

	workbook := excelize.NewFile()
	workbook.SetSheetName("Sheet1", "Domains")
	domainName := "phase6" + unique[:12] + ".com"
	_ = workbook.SetSheetRow("Domains", "A1", &[]any{"domain", "renewal_price", "currency", "expiration_date", "business_priority", "active"})
	_ = workbook.SetSheetRow("Domains", "A2", &[]any{domainName, "500.000000", "THB", time.Now().UTC().Add(30 * 24 * time.Hour).Format("2006-01-02"), "high", "true"})
	var workbookData bytes.Buffer
	if err := workbook.Write(&workbookData); err != nil {
		t.Fatal(err)
	}
	_ = workbook.Close()
	sheetService := sheets.NewService(pool, auditStore, nil, domain.Normalizer{AllowUnknownTLD: true})
	preview, err := sheetService.PreviewExcel(ctx, sheets.Actor{UserID: userID, RequestID: uuid.NewString()}, "excel-preview-"+unique, "inventory.xlsx", "phase6-inventory-"+unique, "Domains", nil, workbookData.Bytes(), sheets.ExcelOptions{MaxBytes: 10 << 20, MaxUncompressedBytes: 64 << 20, MaxRows: 100, MaxColumns: 20})
	if err != nil || preview.SourceKind != "excel" || preview.AddedCount != 1 {
		t.Fatalf("Excel preview: %#v err=%v", preview, err)
	}
	applied, err := sheetService.Apply(ctx, sheets.Actor{UserID: userID, RequestID: uuid.NewString()}, preview.ID, "excel-apply-"+unique, "approved Excel fixture")
	if err != nil || applied.ValidRowsApplied != 1 {
		t.Fatalf("Excel apply: %#v err=%v", applied, err)
	}
	var domainID uuid.UUID
	if err := pool.QueryRow(ctx, `UPDATE domains SET current_availability_status='ACTIVE',current_dns_status='OK',current_http_status='OK',current_redirect_status='NONE',current_isp_status='NOT_DETECTED',current_tls_status='VALID',current_confidence_score=90,last_checked_at=now() WHERE domain_ascii=$1 RETURNING id`, domainName).Scan(&domainID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `UPDATE monitor_schedules SET enabled=false WHERE domain_id=$1`, domainID)
	}()

	recommendationService := recommendation.NewService(pool, auditStore)
	recommendationRecord, err := recommendationService.Generate(ctx, recommendation.Actor{UserID: userID, RequestID: uuid.NewString()}, domainID)
	if err != nil || recommendationRecord.Action != "RENEW" || len(recommendationRecord.ReasonsTH) == 0 || len(recommendationRecord.ReasonsEN) == 0 {
		t.Fatalf("recommendation: %#v err=%v", recommendationRecord, err)
	}
	financeService := finance.NewService(pool, auditStore, 48*time.Hour)
	reportService := report.NewService(pool, auditStore, financeService)
	summary, err := reportService.Summary(ctx, "THB")
	if err != nil || summary.TotalDomains < 1 || summary.RecommendedRenew < 1 {
		t.Fatalf("summary: %#v err=%v", summary, err)
	}
	reportRecord, err := reportService.Create(ctx, report.Actor{UserID: userID, RequestID: uuid.NewString()}, "report-"+unique, "json", "THB")
	if err != nil || reportRecord.SHA256 == "" {
		t.Fatalf("report create: %#v err=%v", reportRecord, err)
	}
	idempotentReport, err := reportService.Create(ctx, report.Actor{UserID: userID, RequestID: uuid.NewString()}, "report-"+unique, "csv", "USD")
	if err != nil || idempotentReport.ID != reportRecord.ID {
		t.Fatalf("report idempotency: first=%s second=%s err=%v", reportRecord.ID, idempotentReport.ID, err)
	}
	payload, err := reportService.Download(ctx, reportRecord.ID)
	digest := sha256.Sum256(payload.Content)
	if err != nil || reportRecord.SHA256 != fmtHex(digest[:]) || !json.Valid(payload.Content) {
		t.Fatalf("report payload invalid: err=%v", err)
	}
}

func fmtHex(value []byte) string {
	const alphabet = "0123456789abcdef"
	encoded := make([]byte, len(value)*2)
	for i, b := range value {
		encoded[i*2], encoded[i*2+1] = alphabet[b>>4], alphabet[b&15]
	}
	return string(encoded)
}
