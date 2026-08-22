package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"domainmonitor/internal/buildinfo"
	"domainmonitor/internal/config"
	"domainmonitor/internal/httpcheck"
	"domainmonitor/internal/probeprotocol"
	"domainmonitor/internal/protocolcheck"
	"github.com/google/uuid"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

type agentConfig struct {
	APIURL            string
	Name              string
	RegionCode        string
	CountryCode       string
	NetworkName       string
	RegistrationToken string
	StatePath         string
	AgentVersion      string
	AllowInsecureHTTP bool
}

type agentState struct {
	ProbeID    uuid.UUID `json:"probe_id"`
	PublicKey  string    `json:"public_key"`
	PrivateKey string    `json:"private_key"`
}

type apiEnvelope[T any] struct {
	Data T `json:"data"`
}

type agent struct {
	cfg         agentConfig
	state       agentState
	privateKey  ed25519.PrivateKey
	client      *http.Client
	token       string
	tokenExpiry time.Time
	logger      *slog.Logger
}

func main() {
	cfg, err := loadAgentConfig()
	if err != nil {
		slog.Error("probe_configuration_invalid", "error", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "domain-monitor-probe", "region", cfg.RegionCode)
	agent := &agent{cfg: cfg, logger: logger, client: &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{Proxy: nil, ForceAttemptHTTP2: true, TLSHandshakeTimeout: 7 * time.Second,
			ResponseHeaderTimeout: 12 * time.Second, MaxResponseHeaderBytes: 64 << 10,
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
	}}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := agent.initialize(ctx); err != nil {
		logger.Error("probe_initialization_failed", "error", err)
		os.Exit(1)
	}
	logger.Info("probe_started", "probe_id", agent.state.ProbeID, "protocol_version", probeprotocol.Version, "agent_version", cfg.AgentVersion)
	if err := agent.run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("probe_stopped_with_error", "error", err)
		os.Exit(1)
	}
	logger.Info("probe_stopped")
}

func loadAgentConfig() (agentConfig, error) {
	stateDir := strings.TrimSpace(os.Getenv("PROBE_STATE_DIR"))
	if stateDir == "" {
		stateDir = "/var/lib/domain-monitor-probe"
	}
	allowInsecure, err := strconv.ParseBool(envValue("PROBE_ALLOW_INSECURE_HTTP", "false"))
	if err != nil {
		return agentConfig{}, fmt.Errorf("PROBE_ALLOW_INSECURE_HTTP must be true or false")
	}
	registrationToken, err := secretValue("PROBE_REGISTRATION_TOKEN")
	if err != nil {
		return agentConfig{}, err
	}
	cfg := agentConfig{
		APIURL: strings.TrimRight(strings.TrimSpace(os.Getenv("PROBE_API_URL")), "/"),
		Name:   strings.TrimSpace(os.Getenv("PROBE_NAME")), RegionCode: strings.ToUpper(strings.TrimSpace(os.Getenv("PROBE_REGION_CODE"))),
		CountryCode: strings.ToUpper(strings.TrimSpace(os.Getenv("PROBE_COUNTRY_CODE"))), NetworkName: strings.TrimSpace(os.Getenv("PROBE_NETWORK_NAME")),
		RegistrationToken: registrationToken, StatePath: filepath.Join(stateDir, "identity.json"),
		AgentVersion: envValue("PROBE_AGENT_VERSION", buildinfo.Version), AllowInsecureHTTP: allowInsecure,
	}
	if cfg.AgentVersion == "" || cfg.AgentVersion == "dev" {
		cfg.AgentVersion = "phase4-dev"
	}
	parsed, err := url.Parse(cfg.APIURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && cfg.AllowInsecureHTTP)) {
		return agentConfig{}, errors.New("PROBE_API_URL must be an absolute HTTPS URL (HTTP requires explicit PROBE_ALLOW_INSECURE_HTTP=true)")
	}
	if cfg.Name == "" || len(cfg.RegionCode) < 2 || len(cfg.RegionCode) > 16 || len(cfg.CountryCode) != 2 {
		return agentConfig{}, errors.New("PROBE_NAME, PROBE_REGION_CODE and two-letter PROBE_COUNTRY_CODE are required")
	}
	return cfg, nil
}

func (a *agent) initialize(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(a.cfg.StatePath), 0700); err != nil {
		return fmt.Errorf("create probe state directory: %w", err)
	}
	raw, err := os.ReadFile(a.cfg.StatePath)
	if err == nil {
		if err := json.Unmarshal(raw, &a.state); err != nil {
			return fmt.Errorf("decode probe identity: %w", err)
		}
		privateKey, err := base64.RawURLEncoding.DecodeString(a.state.PrivateKey)
		if err != nil || len(privateKey) != ed25519.PrivateKeySize || a.state.ProbeID == uuid.Nil {
			return errors.New("stored probe identity is invalid")
		}
		a.privateKey = ed25519.PrivateKey(privateKey)
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read probe identity: %w", err)
	}
	if a.cfg.RegistrationToken == "" {
		return errors.New("PROBE_REGISTRATION_TOKEN is required for first registration")
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Ed25519 identity: %w", err)
	}
	request := probeprotocol.RegisterRequest{
		RegistrationToken: a.cfg.RegistrationToken, Name: a.cfg.Name, RegionCode: a.cfg.RegionCode,
		CountryCode: a.cfg.CountryCode, NetworkName: a.cfg.NetworkName, PublicKey: probeprotocol.EncodePublicKey(publicKey),
		ProtocolVersion: probeprotocol.Version, AgentVersion: a.cfg.AgentVersion,
		Capabilities: map[string]any{"dns": true, "http": true, "tls": true, "content_hash": true},
	}
	var response apiEnvelope[struct {
		Registration probeprotocol.RegisterResponse `json:"registration"`
	}]
	if err := a.doJSON(ctx, http.MethodPost, "/api/v1/probe-auth/register", "", request, &response); err != nil {
		return err
	}
	a.state = agentState{ProbeID: response.Data.Registration.ProbeID, PublicKey: probeprotocol.EncodePublicKey(publicKey), PrivateKey: base64.RawURLEncoding.EncodeToString(privateKey)}
	a.privateKey = privateKey
	encoded, _ := json.MarshalIndent(a.state, "", "  ")
	if err := os.WriteFile(a.cfg.StatePath, encoded, 0600); err != nil {
		return fmt.Errorf("persist probe identity: %w", err)
	}
	return nil
}

func (a *agent) run(ctx context.Context) error {
	lastHeartbeat := time.Time{}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := a.ensureToken(ctx); err != nil {
			a.logger.Warn("probe_token_refresh_failed", "error", err)
			if err := waitContext(ctx, 5*time.Second); err != nil {
				return err
			}
			continue
		}
		if time.Since(lastHeartbeat) >= 25*time.Second {
			if err := a.heartbeat(ctx); err != nil {
				a.logger.Warn("probe_heartbeat_failed", "error", err)
				if err := waitContext(ctx, 5*time.Second); err != nil {
					return err
				}
				continue
			}
			lastHeartbeat = time.Now()
		}
		job, err := a.claim(ctx)
		if err != nil {
			a.logger.Warn("probe_job_claim_failed", "error", err)
			if err := waitContext(ctx, 2*time.Second); err != nil {
				return err
			}
			continue
		}
		if job == nil {
			continue
		}
		if err := a.execute(ctx, *job); err != nil {
			a.logger.Error("probe_job_failed", "job_id", job.JobID, "error", err)
		} else {
			a.logger.Info("probe_job_completed", "job_id", job.JobID, "run_id", job.RunID, "domain", job.Target.DomainASCII)
		}
	}
}

func (a *agent) ensureToken(ctx context.Context) error {
	if a.token != "" && time.Until(a.tokenExpiry) > 30*time.Second {
		return nil
	}
	var challengeResponse apiEnvelope[struct {
		Challenge probeprotocol.TokenChallenge `json:"challenge"`
	}]
	if err := a.doJSON(ctx, http.MethodPost, "/api/v1/probe-auth/token", "", probeprotocol.TokenRequest{ProbeID: a.state.ProbeID}, &challengeResponse); err != nil {
		return err
	}
	challenge := challengeResponse.Data.Challenge
	signature := ed25519.Sign(a.privateKey, []byte(challenge.SigningMessage))
	var tokenResponse apiEnvelope[probeprotocol.TokenResponse]
	if err := a.doJSON(ctx, http.MethodPost, "/api/v1/probe-auth/token", "", probeprotocol.TokenRequest{
		ProbeID: a.state.ProbeID, ChallengeID: challenge.ChallengeID, Signature: probeprotocol.EncodeSignature(signature),
	}, &tokenResponse); err != nil {
		return err
	}
	a.token, a.tokenExpiry = tokenResponse.Data.AccessToken, tokenResponse.Data.ExpiresAt
	return nil
}

func (a *agent) heartbeat(ctx context.Context) error {
	var response apiEnvelope[probeprotocol.HeartbeatResponse]
	err := a.doJSON(ctx, http.MethodPost, "/api/v1/probe-agent/heartbeat", a.token, probeprotocol.HeartbeatRequest{
		ProtocolVersion: probeprotocol.Version, AgentVersion: a.cfg.AgentVersion, ClockOffsetMS: 0, QueueCapacity: 1,
		Capabilities: map[string]any{"dns": true, "http": true, "tls": true, "content_hash": true},
	}, &response)
	if err == nil && response.Data.Status != "ONLINE" {
		return fmt.Errorf("server marked probe as %s", response.Data.Status)
	}
	return err
}

func (a *agent) claim(ctx context.Context) (*probeprotocol.Job, error) {
	var response apiEnvelope[struct {
		Job probeprotocol.Job `json:"job"`
	}]
	status, err := a.doJSONStatus(ctx, http.MethodPost, "/api/v1/probe-agent/jobs/claim", a.token, probeprotocol.ClaimRequest{MaxWaitMS: 10000}, &response)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent {
		return nil, nil
	}
	return &response.Data.Job, nil
}

func (a *agent) execute(ctx context.Context, job probeprotocol.Job) error {
	if err := validateJob(job); err != nil {
		return err
	}
	deadline := time.Duration(job.Policy.DeadlineMS) * time.Millisecond
	checkCfg := config.Config{
		DoHEndpoint: "https://cloudflare-dns.com/dns-query", DoHMaxBytes: 64 << 10,
		DNSAttemptTimeout: min(3*time.Second, deadline), HTTPConnectTimeout: min(5*time.Second, deadline),
		TLSHandshakeTimeout: min(7*time.Second, deadline), HTTPHeaderTimeout: min(10*time.Second, deadline),
		HTTPChainTimeout: deadline, HTTPMaxRedirects: job.Policy.MaxRedirects, HTTPMaxBodyBytes: job.Policy.MaxBodyBytes,
		HTTPExcerptBytes: job.Policy.StoreExcerptBytes, HTTPHeaderBytes: 16 << 10, HTTPMinBodyBytes: 64,
		MonitorMaxAttempts: 2, RetryBaseDelay: 100 * time.Millisecond, RetryMaxDelay: time.Second,
		TLSExpiringSoon: 30 * 24 * time.Hour, MonitorUserAgent: "DomainMonitorProbe/" + a.cfg.AgentVersion,
	}
	protocols, err := protocolcheck.New(checkCfg)
	if err != nil {
		return err
	}
	defer protocols.CloseIdleConnections()
	jobCtx, cancel := context.WithDeadline(ctx, minTime(job.ExpiresAt, time.Now().Add(deadline)))
	defer cancel()
	startedAt := time.Now().UTC()
	dnsResults := protocols.DNS.QueryAll(jobCtx, protocols.LocalResolver, job.Target.DomainASCII, 2)
	httpResult := protocols.HTTP.CheckDomain(jobCtx, job.Target.DomainASCII, httpcheck.ContentHTML)
	payload := probeprotocol.ResultPayload{
		ProtocolVersion: probeprotocol.Version, ProbeID: a.state.ProbeID, AgentVersion: a.cfg.AgentVersion,
		RegionCode: a.cfg.RegionCode, CountryCode: a.cfg.CountryCode, NetworkName: a.cfg.NetworkName,
		JobID: job.JobID, RunID: job.RunID, Nonce: job.Nonce, StartedAt: startedAt, FinishedAt: time.Now().UTC(),
		ClockOffsetMS: 0, DNS: dnsResults, HTTP: httpResult,
	}
	envelope, err := probeprotocol.SignResult(a.privateKey, payload)
	if err != nil {
		return err
	}
	var response apiEnvelope[any]
	return a.doJSON(ctx, http.MethodPost, "/api/v1/probe-agent/jobs/"+job.JobID.String()+"/result", a.token, envelope, &response)
}

func validateJob(job probeprotocol.Job) error {
	if !validCanonicalDomain(job.Target.DomainASCII) {
		return errors.New("job target is not a canonical public domain")
	}
	if len(job.Target.Schemes) != 2 || job.Target.Schemes[0] != "https" || job.Target.Schemes[1] != "http" ||
		len(job.Target.Ports) != 2 || job.Target.Ports[0] != 443 || job.Target.Ports[1] != 80 {
		return errors.New("job target violates the fixed scheme and port contract")
	}
	if job.Policy.DeadlineMS < 1000 || job.Policy.DeadlineMS > 60000 || job.Policy.MaxRedirects < 1 || job.Policy.MaxRedirects > 20 ||
		job.Policy.MaxBodyBytes < 1024 || job.Policy.MaxBodyBytes > 16<<20 || job.Policy.StoreExcerptBytes < 0 ||
		job.Policy.StoreExcerptBytes > 64<<10 || int64(job.Policy.StoreExcerptBytes) > job.Policy.MaxBodyBytes {
		return errors.New("job policy exceeds probe safety limits")
	}
	return nil
}

func validCanonicalDomain(value string) bool {
	if value == "" || value != strings.ToLower(value) || strings.ContainsAny(value, "/:@ \\t\\r\\n") || net.ParseIP(value) != nil {
		return false
	}
	ascii, err := idna.Lookup.ToASCII(value)
	if err != nil || ascii != value || len(ascii) > 253 || strings.HasPrefix(ascii, ".") || strings.HasSuffix(ascii, ".") {
		return false
	}
	for _, label := range strings.Split(ascii, ".") {
		if len(label) < 1 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	suffix, icann := publicsuffix.PublicSuffix(ascii)
	if suffix == "" || !icann || suffix == ascii {
		return false
	}
	_, err = publicsuffix.EffectiveTLDPlusOne(ascii)
	return err == nil
}

func (a *agent) doJSON(ctx context.Context, method, path, token string, requestBody, responseBody any) error {
	_, err := a.doJSONStatus(ctx, method, path, token, requestBody, responseBody)
	return err
}

func (a *agent) doJSONStatus(ctx context.Context, method, path, token string, requestBody, responseBody any) (int, error) {
	raw, err := json.Marshal(requestBody)
	if err != nil {
		return 0, err
	}
	request, err := http.NewRequestWithContext(ctx, method, a.cfg.APIURL+path, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", "en")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := a.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, 2<<20)
	responseRaw, err := io.ReadAll(limited)
	if err != nil {
		return response.StatusCode, err
	}
	if response.StatusCode == http.StatusNoContent {
		return response.StatusCode, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if len(responseRaw) > 512 {
			responseRaw = responseRaw[:512]
		}
		return response.StatusCode, fmt.Errorf("platform returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseRaw)))
	}
	if responseBody != nil {
		if err := json.Unmarshal(responseRaw, responseBody); err != nil {
			return response.StatusCode, fmt.Errorf("decode platform response: %w", err)
		}
	}
	return response.StatusCode, nil
}

func envValue(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func secretValue(key string) (string, error) {
	direct := strings.TrimSpace(os.Getenv(key))
	filePath := strings.TrimSpace(os.Getenv(key + "_FILE"))
	if direct != "" && filePath != "" {
		return "", fmt.Errorf("set only one of %s and %s_FILE", key, key)
	}
	if filePath == "" {
		return direct, nil
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", key, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
