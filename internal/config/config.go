package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment              string
	HTTPAddress              string
	DatabaseURL              string
	RedisAddress             string
	RedisPassword            string
	RedisDB                  int
	DefaultLocale            string
	CookieName               string
	CookieSecure             bool
	SessionTTL               time.Duration
	ShutdownTimeout          time.Duration
	ReadHeaderTimeout        time.Duration
	ReadTimeout              time.Duration
	WriteTimeout             time.Duration
	IdleTimeout              time.Duration
	DBMaxConnections         int32
	DBMinConnections         int32
	DBMaxConnLifetime        time.Duration
	DBMaxConnIdleTime        time.Duration
	AllowUnknownTLD          bool
	APIMaxBodyBytes          int64
	OutboxStream             string
	OutboxBatchSize          int
	OutboxInterval           time.Duration
	OutboxLease              time.Duration
	OutboxMaxAttempts        int
	LocalDNSAddress          string
	DoHEndpoint              string
	DoHMaxBytes              int64
	DNSAttemptTimeout        time.Duration
	HTTPConnectTimeout       time.Duration
	TLSHandshakeTimeout      time.Duration
	HTTPHeaderTimeout        time.Duration
	HTTPChainTimeout         time.Duration
	HTTPMaxRedirects         int
	HTTPMaxBodyBytes         int64
	HTTPExcerptBytes         int
	HTTPHeaderBytes          int
	HTTPMinBodyBytes         int
	MonitorMaxAttempts       int
	RetryBaseDelay           time.Duration
	RetryMaxDelay            time.Duration
	TLSExpiringSoon          time.Duration
	MonitorUserAgent         string
	MonitorPolicyVersion     string
	MonitorRunTimeout        time.Duration
	MonitorWorkers           int
	MonitorQueueGroup        string
	MonitorQueueLease        time.Duration
	MonitorQueueBlock        time.Duration
	SchedulerInterval        time.Duration
	SchedulerBatchSize       int
	IncidentOpenFailures     int
	IncidentCloseSuccess     int
	ProbeTokenTTL            time.Duration
	ProbeChallengeTTL        time.Duration
	ProbeJobTTL              time.Duration
	ProbeJobLease            time.Duration
	ProbeStaleAfter          time.Duration
	ProbeMaxClockSkew        time.Duration
	ProbeEvidenceFresh       time.Duration
	ProbeDispatchBatch       int
	ProbeRequiredRegion      string
	ProbeMaxPayloadBytes     int64
	RDAPBootstrapURL         string
	RDAPMaxBytes             int64
	RDAPRequestTimeout       time.Duration
	RDAPBootstrapTTL         time.Duration
	RDAPBootstrapMaxStale    time.Duration
	RDAPDomainTTL            time.Duration
	RDAPMinInterval          time.Duration
	FinanceFXMaxAge          time.Duration
	GoogleSheetsAPIBase      string
	GoogleSheetsAPIKey       string
	GoogleSheetsAccessToken  string
	GoogleCredentialsFile    string
	GoogleSheetsMaxBytes     int64
	GoogleSheetsTimeout      time.Duration
	GoogleDriveAPIBase       string
	GoogleOAuthAuthURL       string
	GoogleOAuthTokenURL      string
	GoogleOAuthUserInfoURL   string
	GoogleOAuthClientID      string
	GoogleOAuthClientSecret  string
	GoogleOAuthRedirectURL   string
	GoogleOAuthEncryptionKey string
	GoogleOAuthScopes        string
	ExcelImportMaxBytes      int64
	ExcelUnzipMaxBytes       int64
	ExcelImportMaxRows       int
	ExcelImportMaxColumns    int
}

func Load() (Config, error) {
	cfg := Config{
		Environment:              env("APP_ENV", "development"),
		HTTPAddress:              env("HTTP_ADDR", ":8080"),
		DatabaseURL:              env("DATABASE_URL", "postgres://domainmonitor:domainmonitor@localhost:55432/domainmonitor?sslmode=disable"),
		RedisAddress:             env("REDIS_ADDR", "localhost:56379"),
		RedisPassword:            os.Getenv("REDIS_PASSWORD"),
		DefaultLocale:            strings.ToLower(env("DEFAULT_LOCALE", "th")),
		CookieName:               env("SESSION_COOKIE_NAME", "domainintel_session"),
		OutboxStream:             env("OUTBOX_STREAM", "domain-monitor:jobs"),
		LocalDNSAddress:          strings.TrimSpace(os.Getenv("LOCAL_DNS_ADDR")),
		DoHEndpoint:              env("CLOUDFLARE_DOH_ENDPOINT", "https://cloudflare-dns.com/dns-query"),
		MonitorUserAgent:         env("MONITOR_USER_AGENT", "DomainMonitor/phase5"),
		MonitorPolicyVersion:     env("MONITOR_POLICY_VERSION", "monitor-2026-08-v2"),
		MonitorQueueGroup:        env("MONITOR_QUEUE_GROUP", "domain-monitor-workers"),
		ProbeRequiredRegion:      strings.ToUpper(env("PROBE_REQUIRED_REGION", "SG")),
		RDAPBootstrapURL:         env("RDAP_BOOTSTRAP_URL", "https://data.iana.org/rdap/dns.json"),
		GoogleSheetsAPIBase:      env("GOOGLE_SHEETS_API_BASE", "https://sheets.googleapis.com"),
		GoogleSheetsAPIKey:       strings.TrimSpace(os.Getenv("GOOGLE_SHEETS_API_KEY")),
		GoogleSheetsAccessToken:  strings.TrimSpace(os.Getenv("GOOGLE_SHEETS_ACCESS_TOKEN")),
		GoogleCredentialsFile:    strings.TrimSpace(os.Getenv("GOOGLE_SERVICE_ACCOUNT_CREDENTIALS_FILE")),
		GoogleDriveAPIBase:       env("GOOGLE_DRIVE_API_BASE", "https://www.googleapis.com/drive"),
		GoogleOAuthAuthURL:       env("GOOGLE_OAUTH_AUTH_URL", "https://accounts.google.com/o/oauth2/v2/auth"),
		GoogleOAuthTokenURL:      env("GOOGLE_OAUTH_TOKEN_URL", "https://oauth2.googleapis.com/token"),
		GoogleOAuthUserInfoURL:   env("GOOGLE_OAUTH_USERINFO_URL", "https://openidconnect.googleapis.com/v1/userinfo"),
		GoogleOAuthClientID:      strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_ID")),
		GoogleOAuthClientSecret:  strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET")),
		GoogleOAuthRedirectURL:   strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_REDIRECT_URL")),
		GoogleOAuthEncryptionKey: strings.TrimSpace(os.Getenv("GOOGLE_OAUTH_TOKEN_ENCRYPTION_KEY")),
		GoogleOAuthScopes:        env("GOOGLE_OAUTH_SCOPES", "openid email https://www.googleapis.com/auth/drive.file https://www.googleapis.com/auth/spreadsheets.readonly"),
	}

	var err error
	var integerValue int
	if cfg.RedisDB, err = parseInt("REDIS_DB", 0); err != nil {
		return Config{}, err
	}
	if integerValue, err = parseInt("DB_MAX_CONNECTIONS", 20); err != nil {
		return Config{}, err
	}
	cfg.DBMaxConnections = int32(integerValue)
	if integerValue, err = parseInt("DB_MIN_CONNECTIONS", 2); err != nil {
		return Config{}, err
	}
	cfg.DBMinConnections = int32(integerValue)
	if integerValue, err = parseInt("API_MAX_BODY_BYTES", 1<<20); err != nil {
		return Config{}, err
	}
	cfg.APIMaxBodyBytes = int64(integerValue)
	if cfg.OutboxBatchSize, err = parseInt("OUTBOX_BATCH_SIZE", 100); err != nil {
		return Config{}, err
	}
	if cfg.OutboxMaxAttempts, err = parseInt("OUTBOX_MAX_ATTEMPTS", 10); err != nil {
		return Config{}, err
	}
	if integerValue, err = parseInt("DOH_MAX_RESPONSE_BYTES", 64<<10); err != nil {
		return Config{}, err
	}
	cfg.DoHMaxBytes = int64(integerValue)
	if cfg.HTTPMaxRedirects, err = parseInt("MONITOR_MAX_REDIRECTS", 10); err != nil {
		return Config{}, err
	}
	if integerValue, err = parseInt("MONITOR_MAX_BODY_BYTES", 2<<20); err != nil {
		return Config{}, err
	}
	cfg.HTTPMaxBodyBytes = int64(integerValue)
	if cfg.HTTPExcerptBytes, err = parseInt("MONITOR_EXCERPT_BYTES", 32<<10); err != nil {
		return Config{}, err
	}
	if cfg.HTTPHeaderBytes, err = parseInt("MONITOR_HEADER_BYTES", 16<<10); err != nil {
		return Config{}, err
	}
	if cfg.HTTPMinBodyBytes, err = parseInt("MONITOR_MIN_BODY_BYTES", 64); err != nil {
		return Config{}, err
	}
	if cfg.MonitorMaxAttempts, err = parseInt("MONITOR_MAX_ATTEMPTS", 3); err != nil {
		return Config{}, err
	}
	if cfg.MonitorWorkers, err = parseInt("MONITOR_WORKERS", 10); err != nil {
		return Config{}, err
	}
	if cfg.SchedulerBatchSize, err = parseInt("SCHEDULER_BATCH_SIZE", 100); err != nil {
		return Config{}, err
	}
	if cfg.IncidentOpenFailures, err = parseInt("INCIDENT_OPEN_FAILURES", 3); err != nil {
		return Config{}, err
	}
	if cfg.IncidentCloseSuccess, err = parseInt("INCIDENT_CLOSE_SUCCESSES", 2); err != nil {
		return Config{}, err
	}
	if cfg.ProbeDispatchBatch, err = parseInt("PROBE_DISPATCH_BATCH", 100); err != nil {
		return Config{}, err
	}
	if integerValue, err = parseInt("PROBE_MAX_PAYLOAD_BYTES", 1<<20); err != nil {
		return Config{}, err
	}
	cfg.ProbeMaxPayloadBytes = int64(integerValue)
	if integerValue, err = parseInt("RDAP_MAX_RESPONSE_BYTES", 1<<20); err != nil {
		return Config{}, err
	}
	cfg.RDAPMaxBytes = int64(integerValue)
	if integerValue, err = parseInt("GOOGLE_SHEETS_MAX_RESPONSE_BYTES", 4<<20); err != nil {
		return Config{}, err
	}
	cfg.GoogleSheetsMaxBytes = int64(integerValue)
	if integerValue, err = parseInt("EXCEL_IMPORT_MAX_BYTES", 10<<20); err != nil {
		return Config{}, err
	}
	cfg.ExcelImportMaxBytes = int64(integerValue)
	if integerValue, err = parseInt("EXCEL_IMPORT_MAX_UNCOMPRESSED_BYTES", 64<<20); err != nil {
		return Config{}, err
	}
	cfg.ExcelUnzipMaxBytes = int64(integerValue)
	if cfg.ExcelImportMaxRows, err = parseInt("EXCEL_IMPORT_MAX_ROWS", 20000); err != nil {
		return Config{}, err
	}
	if cfg.ExcelImportMaxColumns, err = parseInt("EXCEL_IMPORT_MAX_COLUMNS", 100); err != nil {
		return Config{}, err
	}
	if cfg.CookieSecure, err = parseBool("SESSION_COOKIE_SECURE", true); err != nil {
		return Config{}, err
	}
	if cfg.AllowUnknownTLD, err = parseBool("ALLOW_UNKNOWN_TLD", false); err != nil {
		return Config{}, err
	}
	if cfg.SessionTTL, err = parseDuration("SESSION_TTL", 8*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = parseDuration("SHUTDOWN_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ReadHeaderTimeout, err = parseDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ReadTimeout, err = parseDuration("HTTP_READ_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = parseDuration("HTTP_WRITE_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = parseDuration("HTTP_IDLE_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxConnLifetime, err = parseDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxConnIdleTime, err = parseDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.OutboxInterval, err = parseDuration("OUTBOX_INTERVAL", time.Second); err != nil {
		return Config{}, err
	}
	if cfg.OutboxLease, err = parseDuration("OUTBOX_LEASE", 30*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.DNSAttemptTimeout, err = parseDuration("MONITOR_DNS_TIMEOUT", 3*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.HTTPConnectTimeout, err = parseDuration("MONITOR_CONNECT_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.TLSHandshakeTimeout, err = parseDuration("MONITOR_TLS_TIMEOUT", 7*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.HTTPHeaderTimeout, err = parseDuration("MONITOR_RESPONSE_HEADER_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.HTTPChainTimeout, err = parseDuration("MONITOR_HTTP_CHAIN_TIMEOUT", 25*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.RetryBaseDelay, err = parseDuration("MONITOR_RETRY_BASE_DELAY", 100*time.Millisecond); err != nil {
		return Config{}, err
	}
	if cfg.RetryMaxDelay, err = parseDuration("MONITOR_RETRY_MAX_DELAY", 2*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.TLSExpiringSoon, err = parseDuration("MONITOR_TLS_EXPIRING_SOON", 30*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.MonitorRunTimeout, err = parseDuration("MONITOR_RUN_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.MonitorQueueLease, err = parseDuration("MONITOR_QUEUE_LEASE", 90*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.MonitorQueueBlock, err = parseDuration("MONITOR_QUEUE_BLOCK", 2*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.SchedulerInterval, err = parseDuration("SCHEDULER_INTERVAL", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ProbeTokenTTL, err = parseDuration("PROBE_TOKEN_TTL", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.ProbeChallengeTTL, err = parseDuration("PROBE_CHALLENGE_TTL", time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.ProbeJobTTL, err = parseDuration("PROBE_JOB_TTL", 2*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.ProbeJobLease, err = parseDuration("PROBE_JOB_LEASE", 90*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ProbeStaleAfter, err = parseDuration("PROBE_STALE_AFTER", 90*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ProbeMaxClockSkew, err = parseDuration("PROBE_MAX_CLOCK_SKEW", 2*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.ProbeEvidenceFresh, err = parseDuration("PROBE_EVIDENCE_FRESHNESS", 24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.RDAPRequestTimeout, err = parseDuration("RDAP_REQUEST_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.RDAPBootstrapTTL, err = parseDuration("RDAP_BOOTSTRAP_TTL", 24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.RDAPBootstrapMaxStale, err = parseDuration("RDAP_BOOTSTRAP_MAX_STALE", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.RDAPDomainTTL, err = parseDuration("RDAP_DOMAIN_CACHE_TTL", 6*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.RDAPMinInterval, err = parseDuration("RDAP_MIN_REQUEST_INTERVAL", 200*time.Millisecond); err != nil {
		return Config{}, err
	}
	if cfg.FinanceFXMaxAge, err = parseDuration("FINANCE_FX_MAX_AGE", 48*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.GoogleSheetsTimeout, err = parseDuration("GOOGLE_SHEETS_TIMEOUT", 20*time.Second); err != nil {
		return Config{}, err
	}

	if cfg.DefaultLocale != "th" && cfg.DefaultLocale != "en" {
		return Config{}, fmt.Errorf("DEFAULT_LOCALE must be th or en")
	}
	if cfg.DBMaxConnections <= 0 || cfg.DBMinConnections < 0 || cfg.DBMinConnections > cfg.DBMaxConnections {
		return Config{}, errors.New("invalid PostgreSQL connection pool limits")
	}
	if cfg.APIMaxBodyBytes < 1024 || cfg.APIMaxBodyBytes > 10<<20 {
		return Config{}, errors.New("API_MAX_BODY_BYTES must be between 1024 and 10485760")
	}
	if cfg.OutboxBatchSize < 1 || cfg.OutboxBatchSize > 1000 {
		return Config{}, errors.New("OUTBOX_BATCH_SIZE must be between 1 and 1000")
	}
	if cfg.OutboxMaxAttempts < 1 || cfg.OutboxMaxAttempts > 100 {
		return Config{}, errors.New("OUTBOX_MAX_ATTEMPTS must be between 1 and 100")
	}
	parsedDoH, err := url.Parse(cfg.DoHEndpoint)
	if err != nil || parsedDoH.Scheme != "https" || parsedDoH.Hostname() == "" {
		return Config{}, errors.New("CLOUDFLARE_DOH_ENDPOINT must be an absolute HTTPS URL")
	}
	if cfg.DoHMaxBytes < 4096 || cfg.DoHMaxBytes > 1<<20 {
		return Config{}, errors.New("DOH_MAX_RESPONSE_BYTES must be between 4096 and 1048576")
	}
	if cfg.HTTPMaxRedirects < 1 || cfg.HTTPMaxRedirects > 20 {
		return Config{}, errors.New("MONITOR_MAX_REDIRECTS must be between 1 and 20")
	}
	if cfg.HTTPMaxBodyBytes < 1024 || cfg.HTTPMaxBodyBytes > 16<<20 {
		return Config{}, errors.New("MONITOR_MAX_BODY_BYTES must be between 1024 and 16777216")
	}
	if cfg.HTTPExcerptBytes < 0 || cfg.HTTPExcerptBytes > 64<<10 || int64(cfg.HTTPExcerptBytes) > cfg.HTTPMaxBodyBytes {
		return Config{}, errors.New("MONITOR_EXCERPT_BYTES must be between 0 and 65536 and not exceed MONITOR_MAX_BODY_BYTES")
	}
	if cfg.HTTPHeaderBytes < 1024 || cfg.HTTPHeaderBytes > 64<<10 {
		return Config{}, errors.New("MONITOR_HEADER_BYTES must be between 1024 and 65536")
	}
	if cfg.HTTPMinBodyBytes < 1 || int64(cfg.HTTPMinBodyBytes) > cfg.HTTPMaxBodyBytes {
		return Config{}, errors.New("MONITOR_MIN_BODY_BYTES must be positive and not exceed MONITOR_MAX_BODY_BYTES")
	}
	if cfg.MonitorMaxAttempts < 1 || cfg.MonitorMaxAttempts > 20 {
		return Config{}, errors.New("MONITOR_MAX_ATTEMPTS must be between 1 and 20")
	}
	if cfg.RetryMaxDelay < cfg.RetryBaseDelay {
		return Config{}, errors.New("MONITOR_RETRY_MAX_DELAY must be greater than or equal to MONITOR_RETRY_BASE_DELAY")
	}
	if strings.TrimSpace(cfg.MonitorUserAgent) == "" || len(cfg.MonitorUserAgent) > 200 {
		return Config{}, errors.New("MONITOR_USER_AGENT must be between 1 and 200 characters")
	}
	if strings.TrimSpace(cfg.MonitorPolicyVersion) == "" || len(cfg.MonitorPolicyVersion) > 100 {
		return Config{}, errors.New("MONITOR_POLICY_VERSION must be between 1 and 100 characters")
	}
	if strings.TrimSpace(cfg.MonitorQueueGroup) == "" || len(cfg.MonitorQueueGroup) > 200 {
		return Config{}, errors.New("MONITOR_QUEUE_GROUP must be between 1 and 200 characters")
	}
	if cfg.MonitorWorkers < 1 || cfg.MonitorWorkers > 500 {
		return Config{}, errors.New("MONITOR_WORKERS must be between 1 and 500")
	}
	if cfg.SchedulerBatchSize < 1 || cfg.SchedulerBatchSize > 1000 {
		return Config{}, errors.New("SCHEDULER_BATCH_SIZE must be between 1 and 1000")
	}
	if cfg.IncidentOpenFailures < 1 || cfg.IncidentOpenFailures > 100 {
		return Config{}, errors.New("INCIDENT_OPEN_FAILURES must be between 1 and 100")
	}
	if cfg.IncidentCloseSuccess < 1 || cfg.IncidentCloseSuccess > 100 {
		return Config{}, errors.New("INCIDENT_CLOSE_SUCCESSES must be between 1 and 100")
	}
	if cfg.ProbeDispatchBatch < 1 || cfg.ProbeDispatchBatch > 1000 {
		return Config{}, errors.New("PROBE_DISPATCH_BATCH must be between 1 and 1000")
	}
	if len(cfg.ProbeRequiredRegion) < 2 || len(cfg.ProbeRequiredRegion) > 16 {
		return Config{}, errors.New("PROBE_REQUIRED_REGION must be between 2 and 16 characters")
	}
	if cfg.ProbeMaxPayloadBytes < 4096 || cfg.ProbeMaxPayloadBytes > 10<<20 {
		return Config{}, errors.New("PROBE_MAX_PAYLOAD_BYTES must be between 4096 and 10485760")
	}
	if cfg.ProbeJobLease >= cfg.ProbeJobTTL {
		return Config{}, errors.New("PROBE_JOB_LEASE must be shorter than PROBE_JOB_TTL")
	}
	parsedRDAP, err := url.Parse(cfg.RDAPBootstrapURL)
	if err != nil || parsedRDAP.Scheme != "https" || parsedRDAP.Hostname() == "" || parsedRDAP.User != nil {
		return Config{}, errors.New("RDAP_BOOTSTRAP_URL must be an absolute HTTPS URL")
	}
	parsedSheets, err := url.Parse(cfg.GoogleSheetsAPIBase)
	if err != nil || parsedSheets.Scheme != "https" || parsedSheets.Hostname() == "" || parsedSheets.User != nil {
		return Config{}, errors.New("GOOGLE_SHEETS_API_BASE must be an absolute HTTPS URL")
	}
	if cfg.RDAPMaxBytes < 4096 || cfg.RDAPMaxBytes > 4<<20 {
		return Config{}, errors.New("RDAP_MAX_RESPONSE_BYTES must be between 4096 and 4194304")
	}
	if cfg.GoogleSheetsMaxBytes < 4096 || cfg.GoogleSheetsMaxBytes > 16<<20 {
		return Config{}, errors.New("GOOGLE_SHEETS_MAX_RESPONSE_BYTES must be between 4096 and 16777216")
	}
	parsedDrive, err := url.Parse(cfg.GoogleDriveAPIBase)
	if err != nil || parsedDrive.Scheme != "https" || parsedDrive.Hostname() == "" || parsedDrive.User != nil {
		return Config{}, errors.New("GOOGLE_DRIVE_API_BASE must be an absolute HTTPS URL")
	}
	if cfg.ExcelImportMaxBytes < 1<<20 || cfg.ExcelImportMaxBytes > 50<<20 {
		return Config{}, errors.New("EXCEL_IMPORT_MAX_BYTES must be between 1048576 and 52428800")
	}
	if cfg.ExcelUnzipMaxBytes < cfg.ExcelImportMaxBytes || cfg.ExcelUnzipMaxBytes > 256<<20 {
		return Config{}, errors.New("EXCEL_IMPORT_MAX_UNCOMPRESSED_BYTES must be at least EXCEL_IMPORT_MAX_BYTES and at most 268435456")
	}
	if cfg.ExcelImportMaxRows < 1 || cfg.ExcelImportMaxRows > 100000 || cfg.ExcelImportMaxColumns < 1 || cfg.ExcelImportMaxColumns > 500 {
		return Config{}, errors.New("invalid Excel import row or column limit")
	}
	oauthValues := []string{cfg.GoogleOAuthClientID, cfg.GoogleOAuthClientSecret, cfg.GoogleOAuthRedirectURL, cfg.GoogleOAuthEncryptionKey}
	oauthConfigured := false
	for _, value := range oauthValues {
		oauthConfigured = oauthConfigured || strings.TrimSpace(value) != ""
	}
	if oauthConfigured {
		for _, value := range oauthValues {
			if strings.TrimSpace(value) == "" {
				return Config{}, errors.New("Google OAuth client ID, secret, redirect URL and encryption key must be configured together")
			}
		}
		for name, raw := range map[string]string{"GOOGLE_OAUTH_AUTH_URL": cfg.GoogleOAuthAuthURL, "GOOGLE_OAUTH_TOKEN_URL": cfg.GoogleOAuthTokenURL, "GOOGLE_OAUTH_USERINFO_URL": cfg.GoogleOAuthUserInfoURL} {
			parsed, parseErr := url.Parse(raw)
			if parseErr != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
				return Config{}, fmt.Errorf("%s must be an absolute HTTPS URL", name)
			}
		}
		redirect, parseErr := url.Parse(cfg.GoogleOAuthRedirectURL)
		if parseErr != nil || redirect.Hostname() == "" || (redirect.Scheme != "https" && !(redirect.Scheme == "http" && (redirect.Hostname() == "localhost" || redirect.Hostname() == "127.0.0.1"))) {
			return Config{}, errors.New("GOOGLE_OAUTH_REDIRECT_URL must use HTTPS, except localhost development")
		}
		if len(strings.Fields(cfg.GoogleOAuthScopes)) == 0 {
			return Config{}, errors.New("GOOGLE_OAUTH_SCOPES must not be empty")
		}
		key, decodeErr := base64.StdEncoding.DecodeString(cfg.GoogleOAuthEncryptionKey)
		if decodeErr != nil || len(key) != 32 {
			return Config{}, errors.New("GOOGLE_OAUTH_TOKEN_ENCRYPTION_KEY must be standard base64 for exactly 32 bytes")
		}
	}
	if cfg.RDAPBootstrapMaxStale < cfg.RDAPBootstrapTTL {
		return Config{}, errors.New("RDAP_BOOTSTRAP_MAX_STALE must be greater than or equal to RDAP_BOOTSTRAP_TTL")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

func parseBool(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false: %w", key, err)
	}
	return value, nil
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a Go duration: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return value, nil
}
