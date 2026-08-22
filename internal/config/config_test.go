package config

import (
	"testing"
	"time"
)

func TestLoadDefaultLocale(t *testing.T) {
	t.Setenv("DEFAULT_LOCALE", "en")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DefaultLocale != "en" {
		t.Fatalf("DefaultLocale = %q, want en", cfg.DefaultLocale)
	}
}

func TestLoadRejectsUnsupportedLocale(t *testing.T) {
	t.Setenv("DEFAULT_LOCALE", "ja")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}

func TestLoadRejectsInvalidBoolean(t *testing.T) {
	t.Setenv("SESSION_COOKIE_SECURE", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}

func TestLoadRejectsInvalidInteger(t *testing.T) {
	t.Setenv("DB_MAX_CONNECTIONS", "many")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}

func TestLoadMonitoringDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DoHEndpoint != "https://cloudflare-dns.com/dns-query" || cfg.DNSAttemptTimeout != 3*time.Second || cfg.HTTPMaxRedirects != 10 || cfg.HTTPMaxBodyBytes != 2<<20 || cfg.MonitorMaxAttempts != 3 {
		t.Fatalf("unexpected monitoring defaults: %#v", cfg)
	}
	if cfg.MonitorPolicyVersion != "monitor-2026-08-v2" || cfg.MonitorWorkers != 10 || cfg.IncidentOpenFailures != 3 || cfg.IncidentCloseSuccess != 2 {
		t.Fatalf("unexpected Phase 4 defaults: %#v", cfg)
	}
	if cfg.ProbeRequiredRegion != "SG" || cfg.ProbeTokenTTL != 5*time.Minute || cfg.ProbeJobTTL != 2*time.Minute || cfg.ProbeMaxClockSkew != 2*time.Minute {
		t.Fatalf("unexpected remote probe defaults: %#v", cfg)
	}
	if cfg.RDAPBootstrapURL != "https://data.iana.org/rdap/dns.json" || cfg.RDAPDomainTTL != 6*time.Hour || cfg.FinanceFXMaxAge != 48*time.Hour || cfg.GoogleSheetsAPIBase != "https://sheets.googleapis.com" {
		t.Fatalf("unexpected Phase 5 defaults: %#v", cfg)
	}
	if cfg.GoogleDriveAPIBase != "https://www.googleapis.com/drive" || cfg.ExcelImportMaxBytes != 10<<20 || cfg.ExcelUnzipMaxBytes != 64<<20 || cfg.ExcelImportMaxRows != 20000 || cfg.ExcelImportMaxColumns != 100 {
		t.Fatalf("unexpected Drive/Excel defaults: %#v", cfg)
	}
}

func TestLoadRejectsIncompleteOrInvalidOAuthSecrets(t *testing.T) {
	t.Setenv("GOOGLE_OAUTH_CLIENT_ID", "client")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected partial OAuth configuration to fail")
	}
	t.Setenv("GOOGLE_OAUTH_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_OAUTH_REDIRECT_URL", "https://example.internal/callback")
	t.Setenv("GOOGLE_OAUTH_TOKEN_ENCRYPTION_KEY", "not-base64")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an invalid OAuth encryption key to fail")
	}
}

func TestLoadRejectsUnsafeDoHEndpoint(t *testing.T) {
	t.Setenv("CLOUDFLARE_DOH_ENDPOINT", "http://resolver.example/dns-query")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}

func TestLoadRejectsInvalidMonitoringBounds(t *testing.T) {
	t.Setenv("MONITOR_EXCERPT_BYTES", "70000")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}
