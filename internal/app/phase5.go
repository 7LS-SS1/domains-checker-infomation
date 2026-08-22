package app

import (
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/config"
	"domainmonitor/internal/domain"
	"domainmonitor/internal/drive"
	"domainmonitor/internal/finance"
	"domainmonitor/internal/rdap"
	"domainmonitor/internal/safedial"
	"domainmonitor/internal/sheets"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Phase5Services struct {
	RDAP    *rdap.Service
	Finance *finance.Service
	Sheets  *sheets.Service
	Drive   *drive.Service
}

func NewPhase5Services(cfg config.Config, pool *pgxpool.Pool, auditStore *audit.Store) Phase5Services {
	dialer := safedial.New(net.DefaultResolver, safedial.Policy{AllowedPorts: map[uint16]struct{}{443: {}}, ConnectTimeout: cfg.HTTPConnectTimeout})
	transport := &http.Transport{
		Proxy: nil, DialContext: dialer.DialContext, ForceAttemptHTTP2: true,
		MaxIdleConns: 100, MaxIdleConnsPerHost: 10, IdleConnTimeout: 90 * time.Second,
		TLSHandshakeTimeout: cfg.TLSHandshakeTimeout, ResponseHeaderTimeout: max(cfg.RDAPRequestTimeout, cfg.GoogleSheetsTimeout),
		ExpectContinueTimeout: time.Second, MaxResponseHeaderBytes: 32 << 10,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	httpClient := &http.Client{Transport: transport, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 3 || request.URL.Scheme != "https" {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	rdapClient := rdap.NewClient(httpClient, rdap.Config{BootstrapURL: cfg.RDAPBootstrapURL, UserAgent: cfg.MonitorUserAgent, MaxBytes: cfg.RDAPMaxBytes, RequestTimeout: cfg.RDAPRequestTimeout, RetryMaxDelay: cfg.RetryMaxDelay, MinInterval: cfg.RDAPMinInterval})
	googleClient := sheets.NewGoogleClient(httpClient, sheets.GoogleClientConfig{APIBase: cfg.GoogleSheetsAPIBase, APIKey: cfg.GoogleSheetsAPIKey, AccessToken: cfg.GoogleSheetsAccessToken, CredentialsFile: cfg.GoogleCredentialsFile, MaxBytes: cfg.GoogleSheetsMaxBytes, Timeout: cfg.GoogleSheetsTimeout})
	driveService := drive.NewService(pool, auditStore, httpClient, drive.Config{APIBase: cfg.GoogleDriveAPIBase, AuthorizationURL: cfg.GoogleOAuthAuthURL, TokenURL: cfg.GoogleOAuthTokenURL, UserInfoURL: cfg.GoogleOAuthUserInfoURL, ClientID: cfg.GoogleOAuthClientID, ClientSecret: cfg.GoogleOAuthClientSecret, RedirectURL: cfg.GoogleOAuthRedirectURL, EncryptionKey: cfg.GoogleOAuthEncryptionKey, Scopes: strings.Fields(cfg.GoogleOAuthScopes), Timeout: cfg.GoogleSheetsTimeout, MaxBytes: cfg.GoogleSheetsMaxBytes})
	normalizer := domain.Normalizer{AllowUnknownTLD: cfg.AllowUnknownTLD}
	return Phase5Services{
		RDAP:    rdap.NewService(pool, auditStore, rdapClient, rdap.ServiceConfig{BootstrapTTL: cfg.RDAPBootstrapTTL, BootstrapMaxStale: cfg.RDAPBootstrapMaxStale, DomainTTL: cfg.RDAPDomainTTL}),
		Finance: finance.NewService(pool, auditStore, cfg.FinanceFXMaxAge),
		Sheets:  sheets.NewService(pool, auditStore, sheets.ConnectedSource{Google: googleClient, Tokens: driveService}, normalizer),
		Drive:   driveService,
	}
}
