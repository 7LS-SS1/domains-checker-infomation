//go:build integration

package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
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
	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/httpcheck"
	"domainmonitor/internal/platform/postgres"
	"domainmonitor/internal/platform/rediscache"
	"domainmonitor/internal/probe"
	"domainmonitor/internal/probeprotocol"
	"github.com/google/uuid"
)

func TestPhase4SignedRemoteProbeWorkflow(t *testing.T) {
	unique := strings.ToLower(strings.ReplaceAll(uuid.NewString(), "-", ""))
	regionCode := "SGT" + strings.ToUpper(unique[:8])
	t.Setenv("PROBE_EVIDENCE_FRESHNESS", "2s")
	t.Setenv("PROBE_REQUIRED_REGION", regionCode)
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

	userID := uuid.New()
	passwordHash, err := auth.HashPassword("phase4-integration-" + unique)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,display_name,password_hash,locale) VALUES ($1,$2,'Phase 4 Admin',$3,'th')`, userID, "phase4-"+unique+"@example.internal", passwordHash); err != nil {
		t.Fatal(err)
	}
	domainID, runID := uuid.New(), uuid.New()
	domainName := "phase4-" + unique + ".com"
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO domains (id, original_input, domain_ascii, domain_unicode, registrable_domain, source_type, expected_content_mode)
		VALUES ($1,$2,$2,$2,$2,'manual','HTML')
	`, domainID, domainName); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO monitoring_runs
		(id,domain_id,trigger_type,status,priority,deduplication_key,policy_version,policy_snapshot,scheduled_for,deadline_at,started_at,completed_at)
		VALUES ($1,$2,'scheduled','completed','normal',$3,$4,'{}'::jsonb,$5,$6,$5,$5)
	`, runID, domainID, "phase4:"+unique, cfg.MonitorPolicyVersion, now.Add(-time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	localResultID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO monitoring_results
		(id,monitoring_run_id,domain_id,vantage_type,vantage_key,observed_availability,dns_status,http_status,redirect_status,isp_status,tls_status,content_status,failure_stage,error_code,confidence_score,confidence_level,policy_version,checked_at,completed_at)
		VALUES ($1,$2,$3,'local','local-primary','UNAVAILABLE','DISCREPANCY','CONNECTION_ERROR','NONE','UNKNOWN','UNKNOWN','UNKNOWN','tcp','TCP_TIMEOUT',60,'MEDIUM',$4,$5,$5)
	`, localResultID, runID, domainID, cfg.MonitorPolicyVersion, now.Add(-30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO http_checks
		(id,monitoring_result_id,scheme,resolver_mode,request_url,attempt_no,total_duration_us,body_size,content_status,selected_headers,error_code,checked_at)
		VALUES ($1,$2,'https','doh_pinned',$3,1,1000,0,'UNKNOWN','{}'::jsonb,'TCP_TIMEOUT',$4)
	`, uuid.New(), localResultID, "https://"+domainName+"/", now.Add(-30*time.Second)); err != nil {
		t.Fatal(err)
	}

	probeService := probe.NewService(pool, audit.NewStore(), cfg)
	registration, err := probeService.CreateRegistrationToken(ctx, probe.RegistrationSpec{
		Name: "sg-" + unique, Region: regionCode, Country: "SG", Network: "integration-egress",
		TTL: 10 * time.Minute, CreatedBy: userID, RequestID: uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	handler := app.NewAPIHandler(cfg, logger, pool, redisClient)
	registerResponse := performJSON(t, handler, http.MethodPost, "/api/v1/probe-auth/register", probeprotocol.RegisterRequest{
		RegistrationToken: registration.Token, Name: registration.Name, RegionCode: registration.Region,
		CountryCode: registration.Country, NetworkName: registration.Network, PublicKey: probeprotocol.EncodePublicKey(publicKey),
		ProtocolVersion: probeprotocol.Version, AgentVersion: "phase4-integration", Capabilities: map[string]any{"http": true},
	}, nil, map[string]string{"Accept-Language": "en"})
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", registerResponse.Code, registerResponse.Body.String())
	}
	var registered struct {
		Data struct {
			Registration probeprotocol.RegisterResponse `json:"registration"`
		} `json:"data"`
	}
	decodeResponse(t, registerResponse, &registered)
	probeID := registered.Data.Registration.ProbeID

	challengeResponse := performJSON(t, handler, http.MethodPost, "/api/v1/probe-auth/token", probeprotocol.TokenRequest{ProbeID: probeID}, nil, nil)
	var challengeBody struct {
		Data struct {
			Challenge probeprotocol.TokenChallenge `json:"challenge"`
		} `json:"data"`
	}
	decodeResponse(t, challengeResponse, &challengeBody)
	challenge := challengeBody.Data.Challenge
	invalidSignatureResponse := performJSON(t, handler, http.MethodPost, "/api/v1/probe-auth/token", probeprotocol.TokenRequest{
		ProbeID: probeID, ChallengeID: challenge.ChallengeID, Signature: probeprotocol.EncodeSignature(make([]byte, ed25519.SignatureSize)),
	}, nil, map[string]string{"Accept-Language": "en"})
	if invalidSignatureResponse.Code != http.StatusUnauthorized || !strings.Contains(invalidSignatureResponse.Body.String(), `"code":"PROBE_SIGNATURE_INVALID"`) {
		t.Fatalf("invalid signature status = %d, body = %s", invalidSignatureResponse.Code, invalidSignatureResponse.Body.String())
	}
	tokenResponse := performJSON(t, handler, http.MethodPost, "/api/v1/probe-auth/token", probeprotocol.TokenRequest{
		ProbeID: probeID, ChallengeID: challenge.ChallengeID,
		Signature: probeprotocol.EncodeSignature(ed25519.Sign(privateKey, []byte(challenge.SigningMessage))),
	}, nil, nil)
	if tokenResponse.Code != http.StatusOK {
		t.Fatalf("token status = %d, body = %s", tokenResponse.Code, tokenResponse.Body.String())
	}
	var tokenBody struct {
		Data probeprotocol.TokenResponse `json:"data"`
	}
	decodeResponse(t, tokenResponse, &tokenBody)
	authorization := map[string]string{"Authorization": "Bearer " + tokenBody.Data.AccessToken, "Accept-Language": "th"}
	heartbeatResponse := performJSON(t, handler, http.MethodPost, "/api/v1/probe-agent/heartbeat", probeprotocol.HeartbeatRequest{
		ProtocolVersion: probeprotocol.Version, AgentVersion: "phase4-integration", QueueCapacity: 1,
	}, nil, authorization)
	if heartbeatResponse.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d, body = %s", heartbeatResponse.Code, heartbeatResponse.Body.String())
	}
	upgradeResponse := performJSON(t, handler, http.MethodPost, "/api/v1/probe-agent/heartbeat", probeprotocol.HeartbeatRequest{
		ProtocolVersion: "unsupported.v0", AgentVersion: "phase4-integration", QueueCapacity: 1,
	}, nil, authorization)
	if upgradeResponse.Code != http.StatusOK || !strings.Contains(upgradeResponse.Body.String(), `"status":"UPGRADE_REQUIRED"`) {
		t.Fatalf("upgrade heartbeat status = %d, body = %s", upgradeResponse.Code, upgradeResponse.Body.String())
	}
	upgradeClaim := performJSON(t, handler, http.MethodPost, "/api/v1/probe-agent/jobs/claim", probeprotocol.ClaimRequest{}, nil, authorization)
	if upgradeClaim.Code != http.StatusForbidden {
		t.Fatalf("upgrade-required claim status = %d, body = %s", upgradeClaim.Code, upgradeClaim.Body.String())
	}
	heartbeatResponse = performJSON(t, handler, http.MethodPost, "/api/v1/probe-agent/heartbeat", probeprotocol.HeartbeatRequest{
		ProtocolVersion: probeprotocol.Version, AgentVersion: "phase4-integration", QueueCapacity: 1,
	}, nil, authorization)
	if heartbeatResponse.Code != http.StatusOK || !strings.Contains(heartbeatResponse.Body.String(), `"status":"ONLINE"`) {
		t.Fatalf("restored heartbeat status = %d, body = %s", heartbeatResponse.Code, heartbeatResponse.Body.String())
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `UPDATE probe_nodes SET status = 'REVOKED', revoked_at = COALESCE(revoked_at, now()) WHERE id = $1`, probeID)
	}()
	if _, err := pool.Exec(ctx, `UPDATE monitoring_runs SET completed_at = now() WHERE id = $1`, runID); err != nil {
		t.Fatal(err)
	}
	if count, err := probeService.DispatchPending(ctx); err != nil || count < 1 {
		t.Fatalf("dispatched = %d, err = %v", count, err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE remote_probe_jobs SET status = 'cancelled', last_error_code = 'INTEGRATION_ISOLATION'
		WHERE probe_node_id = $1 AND monitoring_run_id <> $2 AND status = 'queued'
	`, probeID, runID); err != nil {
		t.Fatal(err)
	}
	claimResponse := performJSON(t, handler, http.MethodPost, "/api/v1/probe-agent/jobs/claim", probeprotocol.ClaimRequest{}, nil, authorization)
	if claimResponse.Code != http.StatusOK {
		t.Fatalf("claim status = %d, body = %s", claimResponse.Code, claimResponse.Body.String())
	}
	var claimBody struct {
		Data struct {
			Job probeprotocol.Job `json:"job"`
		} `json:"data"`
	}
	decodeResponse(t, claimResponse, &claimBody)
	job := claimBody.Data.Job
	bodyHash := sha256.Sum256([]byte("phase4 stable remote content"))
	payload := probeprotocol.ResultPayload{
		ProtocolVersion: probeprotocol.Version, ProbeID: probeID, AgentVersion: "phase4-integration",
		RegionCode: regionCode, CountryCode: "SG", NetworkName: "integration-egress", JobID: job.JobID, RunID: job.RunID,
		Nonce: job.Nonce, StartedAt: time.Now().UTC().Add(-time.Second), FinishedAt: time.Now().UTC(), DNS: []dnscheck.Result{{
			Resolver: dnscheck.ResolverRemoteSystem, Endpoint: "fixture", QueryName: domainName + ".", QueryType: dnscheck.TypeA,
			Attempt: 1, RCode: "NOERROR", Answers: []dnscheck.Answer{{Name: domainName + ".", Type: dnscheck.TypeA, Value: "93.184.216.34", TTL: 60}}, CheckedAt: time.Now().UTC(),
		}}, HTTP: httpcheck.OriginResult{HTTPS: []httpcheck.Attempt{{
			Attempt: 1, RequestURL: "https://" + domainName + "/", EffectiveURL: "https://" + domainName + "/", Protocol: "HTTP/2.0",
			InitialStatusCode: 200, FinalStatusCode: 200, Timing: httpcheck.Timing{Total: 100 * time.Millisecond},
			Content: httpcheck.ContentEvidence{Status: "VALID_HTML", ContentType: "text/html", BodySize: 128, BodySHA256: bodyHash[:], HashComplete: true},
			Headers: map[string]string{"Content-Type": "text/html"}, CheckedAt: time.Now().UTC(),
		}}},
	}
	envelope, err := probeprotocol.SignResult(privateKey, payload)
	if err != nil {
		t.Fatal(err)
	}
	resultResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/probe-agent/jobs/%s/result", job.JobID), envelope, nil, authorization)
	if resultResponse.Code != http.StatusAccepted || !strings.Contains(resultResponse.Body.String(), `"isp_status":"SUSPECTED"`) {
		t.Fatalf("result status = %d, body = %s", resultResponse.Code, resultResponse.Body.String())
	}
	replayResponse := performJSON(t, handler, http.MethodPost, fmt.Sprintf("/api/v1/probe-agent/jobs/%s/result", job.JobID), envelope, nil, authorization)
	if replayResponse.Code != http.StatusConflict || !strings.Contains(replayResponse.Body.String(), `"messages":{"th":`) {
		t.Fatalf("replay status = %d, body = %s", replayResponse.Code, replayResponse.Body.String())
	}
	principal, err := probeService.Authenticate(ctx, tokenBody.Data.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	tooLarge := probeprotocol.ResultEnvelope{Payload: make([]byte, cfg.ProbeMaxPayloadBytes+1)}
	if _, err := probeService.SubmitResult(ctx, principal, job.JobID, tooLarge); !errors.Is(err, probe.ErrPayloadTooLarge) {
		t.Fatalf("oversize result error = %v", err)
	}
	payload.ClockOffsetMS = int(cfg.ProbeMaxClockSkew.Milliseconds()) + 1
	skewed, _ := probeprotocol.SignResult(privateKey, payload)
	if _, err := probeService.SubmitResult(ctx, principal, job.JobID, skewed); !errors.Is(err, probe.ErrClockSkew) {
		t.Fatalf("clock skew error = %v", err)
	}
	expiredChallenge, err := probeService.CreateChallenge(ctx, probeID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE probe_auth_challenges SET created_at = now() - interval '2 seconds', expires_at = now() - interval '1 second' WHERE id = $1`, expiredChallenge.ChallengeID); err != nil {
		t.Fatal(err)
	}
	_, err = probeService.IssueToken(ctx, probeprotocol.TokenRequest{ProbeID: probeID, ChallengeID: expiredChallenge.ChallengeID, Signature: probeprotocol.EncodeSignature(ed25519.Sign(privateKey, []byte(expiredChallenge.SigningMessage)))})
	if !errors.Is(err, probe.ErrExpired) {
		t.Fatalf("expired challenge error = %v", err)
	}
	var signatureValid bool
	var jobStatus, ispStatus string
	if err := pool.QueryRow(ctx, `SELECT signature_valid FROM remote_probe_results WHERE job_id = $1`, job.JobID).Scan(&signatureValid); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status::text FROM remote_probe_jobs WHERE id = $1`, job.JobID).Scan(&jobStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT current_isp_status::text FROM domains WHERE id = $1`, domainID).Scan(&ispStatus); err != nil {
		t.Fatal(err)
	}
	if !signatureValid || jobStatus != "completed" || ispStatus != "SUSPECTED" {
		t.Fatalf("signature=%v job=%s isp=%s", signatureValid, jobStatus, ispStatus)
	}
	if err := probeService.Revoke(ctx, probeID, userID, uuid.NewString(), "integration cleanup"); err != nil {
		t.Fatal(err)
	}
	if _, err := probeService.Authenticate(ctx, tokenBody.Data.AccessToken); !errors.Is(err, probe.ErrUnauthorized) {
		t.Fatalf("revoked token authentication error = %v", err)
	}
}
