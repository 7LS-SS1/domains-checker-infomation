//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/auth"
	"domainmonitor/internal/domain"
	"domainmonitor/internal/finance"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/rdap"
	"domainmonitor/internal/sheets"
	"github.com/google/uuid"
)

type fixtureSheetSource struct{ snapshot sheets.Snapshot }

func (source *fixtureSheetSource) Fetch(context.Context, sheets.Config) (sheets.Snapshot, error) {
	return source.snapshot, nil
}

func TestPhase5AssetIntelligenceWorkflow(t *testing.T) {
	cfg := testConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := postgres.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	for _, table := range []string{"rdap_bootstrap_cache", "google_sheet_configs", "domain_field_provenance", "provenance_conflicts"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists); err != nil || !exists {
			t.Fatalf("Phase 5 table %s missing: exists=%v err=%v", table, exists, err)
		}
	}
	unique := strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
	userID := uuid.New()
	passwordHash, err := auth.HashPassword("integration-password-" + unique)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,display_name,password_hash,locale) VALUES ($1,$2,'Phase 5 Admin',$3,'th')`, userID, "phase5-"+unique+"@example.internal", passwordHash); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id,role_id,granted_by) SELECT $1,id,$1 FROM roles WHERE code='ADMIN'`, userID); err != nil {
		t.Fatal(err)
	}
	actor := sheets.Actor{UserID: userID, RequestID: uuid.NewString()}
	auditStore := audit.NewStore()
	normalizer := domain.Normalizer{AllowUnknownTLD: true}
	domainA := "phase5-a-" + unique + ".com"
	domainB := "phase5-b-" + unique + ".com"
	domainC := "phase5-c-" + unique + ".com"
	duplicate := "phase5-duplicate-" + unique + ".com"
	source := &fixtureSheetSource{snapshot: sheets.Snapshot{Revision: "rev-1", Values: [][]string{
		{"domain", "registrar", "purchase_price", "renewal_price", "currency", "tax_rate", "purchase_date", "expiration_date", "business_priority", "notes", "active"},
		{domainA, "Fixture Registrar", "250.00", "100.00", "THB", "0.07", "2025-01-01", "2026-09-01", "high", "first", "true"},
		{domainB, "Fixture Registrar", "50.00", "80.00", "THB", "0.07", "2025-01-01", "2026-10-01", "medium", "second", "true"},
		{duplicate, "", "", "", "", "", "", "", "medium", "duplicate one", "true"},
		{strings.ToUpper(duplicate), "", "", "", "", "", "", "", "medium", "duplicate two", "true"},
		{"bad domain", "", "not-money", "", "", "", "bad-date", "", "wrong", "invalid", "maybe"},
	}}}
	sheetService := sheets.NewService(pool, auditStore, source, normalizer)
	version := int64(0)
	if existing, getErr := sheetService.GetConfig(ctx); getErr == nil {
		version = existing.Version
	}
	config, err := sheetService.SaveConfig(ctx, actor, sheets.ConfigInput{SpreadsheetID: "fixture-" + unique, SheetName: "Domains", Range: "A:K", Enabled: false, SyncIntervalMinutes: 60, Version: version, Reason: "Phase 5 integration"})
	if err != nil || config.SpreadsheetID == "" {
		t.Fatalf("save sheet config: %#v err=%v", config, err)
	}
	preview1, err := sheetService.Preview(ctx, actor, "preview-1-"+unique, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if preview1.AddedCount != 2 || preview1.InvalidCount != 3 || preview1.ModifiedCount != 0 {
		t.Fatalf("unexpected initial preview counts: %#v", preview1)
	}
	applied1, err := sheetService.Apply(ctx, actor, preview1.ID, "apply-1-"+unique, "approved integration import")
	if err != nil || applied1.Status != "applied" || applied1.ValidRowsApplied != 2 {
		t.Fatalf("initial apply: %#v err=%v", applied1, err)
	}
	var domainAID, domainBID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM domains WHERE domain_ascii=$1`, domainA).Scan(&domainAID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM domains WHERE domain_ascii=$1`, domainB).Scan(&domainBID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `UPDATE monitor_schedules SET enabled=false WHERE domain_id IN ($1,$2) OR domain_id IN (SELECT id FROM domains WHERE domain_ascii=$3)`, domainAID, domainBID, domainC)
	}()

	financeService := finance.NewService(pool, auditStore, 48*time.Hour)
	overrideValue := []byte(`"125.000000"`)
	override, err := financeService.CreateOverride(ctx, finance.Actor{UserID: userID, RequestID: uuid.NewString()}, domainAID, "renewal_price", overrideValue, "contracted manual renewal", nil)
	if err != nil || string(override.OriginalValue) != `"100.000000"` {
		t.Fatalf("manual override: %#v err=%v", override, err)
	}

	source.snapshot = sheets.Snapshot{Revision: "rev-2", Values: [][]string{
		{"domain", "registrar", "purchase_price", "renewal_price", "currency", "tax_rate", "purchase_date", "expiration_date", "business_priority", "notes", "active"},
		{domainA, "Fixture Registrar", "250.00", "110.00", "THB", "0.07", "2025-01-01", "2026-09-15", "critical", "modified", "true"},
		{domainC, "Fixture Registrar", "75.00", "90.00", "THB", "", "2025-02-01", "2026-11-01", "low", "added second sync", "false"},
	}}
	preview2, err := sheetService.Preview(ctx, actor, "preview-2-"+unique, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if preview2.AddedCount != 1 || preview2.ModifiedCount != 1 || preview2.MissingCount != 1 || preview2.InvalidCount != 0 {
		t.Fatalf("unexpected second preview counts: %#v", preview2)
	}
	idempotentPreview, err := sheetService.Preview(ctx, actor, "preview-2-"+unique, "manual")
	if err != nil || idempotentPreview.ID != preview2.ID {
		t.Fatalf("preview idempotency failed: id=%s err=%v", idempotentPreview.ID, err)
	}
	applied2, err := sheetService.Apply(ctx, actor, preview2.ID, "apply-2-"+unique, "approved second import")
	if err != nil || applied2.Status != "applied" || applied2.ValidRowsApplied != 3 {
		t.Fatalf("second apply: %#v err=%v", applied2, err)
	}
	idempotentApply, err := sheetService.Apply(ctx, actor, preview2.ID, "apply-2-"+unique, "approved second import")
	if err != nil || idempotentApply.ID != preview2.ID {
		t.Fatalf("apply idempotency failed: %#v err=%v", idempotentApply, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE google_sheet_configs SET enabled=true,next_sync_at=now()-interval '1 second' WHERE id=$1`, config.ID); err != nil {
		t.Fatal(err)
	}
	scheduled, err := sheetService.ScheduleDue(ctx)
	if err != nil || scheduled != 1 {
		t.Fatalf("scheduled Sheet preview: count=%d err=%v", scheduled, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE google_sheet_configs SET enabled=false,next_sync_at=NULL WHERE id=$1`, config.ID); err != nil {
		t.Fatal(err)
	}
	var scheduledCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM google_sheet_imports WHERE config_id=$1 AND trigger_type='scheduled'`, config.ID).Scan(&scheduledCount); err != nil || scheduledCount < 1 {
		t.Fatalf("scheduled preview history missing: count=%d err=%v", scheduledCount, err)
	}
	var sourceStatus string
	if err := pool.QueryRow(ctx, `SELECT source_status::text FROM domains WHERE id=$1`, domainBID).Scan(&sourceStatus); err != nil || sourceStatus != "missing_from_source" {
		t.Fatalf("missing domain was deleted or status wrong: %s err=%v", sourceStatus, err)
	}
	var domainCCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM domains WHERE domain_ascii=$1`, domainC).Scan(&domainCCount); err != nil || domainCCount != 1 {
		t.Fatalf("second sync add failed: count=%d err=%v", domainCCount, err)
	}

	costs, err := financeService.ListCosts(ctx, domainAID)
	if err != nil || len(costs) < 4 {
		t.Fatalf("sheet cost history unavailable: count=%d err=%v", len(costs), err)
	}
	rate, err := financeService.AddRate(ctx, finance.Actor{UserID: userID, RequestID: uuid.NewString()}, finance.AddRateInput{BaseCurrency: "USD", QuoteCurrency: "THB", Rate: "35.5000000000", Source: "integration", ObservedAt: time.Now().UTC(), Reason: "integration FX"})
	if err != nil || rate.Rate != "35.5000000000" {
		t.Fatalf("exchange rate: %#v err=%v", rate, err)
	}
	summary, err := financeService.BudgetSummary(ctx, "THB")
	if err != nil || summary.TotalRenewalCost == "" || summary.TotalAnnualBudget == "" {
		t.Fatalf("finance summary: %#v err=%v", summary, err)
	}

	var rdapServer *httptest.Server
	rdapServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		if request.URL.Path == "/bootstrap" {
			_, _ = fmt.Fprintf(w, `{"services":[[["com"],[%q]]]}`, rdapServer.URL)
			return
		}
		if strings.HasPrefix(request.URL.Path, "/domain/") {
			_, _ = fmt.Fprint(w, `{"objectClassName":"domain","status":["active"],"events":[{"eventAction":"registration","eventDate":"2025-01-01T00:00:00Z"},{"eventAction":"expiration","eventDate":"2027-01-01T00:00:00Z"}],"nameservers":[{"ldhName":"ns1.example."}],"secureDNS":{"delegationSigned":true},"entities":[{"roles":["registrar"],"vcardArray":["vcard",[["fn",{},"text","Different Registrar"]]],"publicIds":[{"type":"IANA Registrar ID","identifier":"123"}]}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer rdapServer.Close()
	rdapClient := rdap.NewClient(rdapServer.Client(), rdap.Config{BootstrapURL: rdapServer.URL + "/bootstrap", MinInterval: time.Nanosecond, RequestTimeout: 3 * time.Second})
	rdapService := rdap.NewService(pool, auditStore, rdapClient, rdap.ServiceConfig{BootstrapTTL: time.Hour, BootstrapMaxStale: time.Hour, DomainTTL: time.Hour})
	result, err := rdapService.Check(ctx, rdap.Actor{UserID: userID, RequestID: uuid.NewString()}, domainAID, true)
	if err != nil || result.SourceStatus != "available" || len(result.Conflicts) < 2 {
		t.Fatalf("RDAP result: %#v err=%v", result, err)
	}
	var conflictCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM provenance_conflicts WHERE domain_id=$1 AND status='open'`, domainAID).Scan(&conflictCount); err != nil || conflictCount < 2 {
		t.Fatalf("RDAP conflicts not persisted: count=%d err=%v", conflictCount, err)
	}
}
