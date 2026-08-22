package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/classification"
	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/ispclass"
	"domainmonitor/internal/monitor"
	"domainmonitor/internal/probeprotocol"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SubmitResult struct {
	MonitoringResultID uuid.UUID         `json:"monitoring_result_id"`
	ISP                ispclass.Decision `json:"isp_classification"`
}

func (s *Service) SubmitResult(ctx context.Context, principal Principal, jobID uuid.UUID, envelope probeprotocol.ResultEnvelope) (SubmitResult, error) {
	if int64(len(envelope.Payload)) > s.cfg.ProbeMaxPayloadBytes {
		return SubmitResult{}, ErrPayloadTooLarge
	}
	payload, payloadHash, err := probeprotocol.VerifyResult(principal.PublicKey, envelope)
	if err != nil {
		return SubmitResult{}, errors.Join(ErrSignature, err)
	}
	if payload.ProtocolVersion != probeprotocol.Version || payload.ProbeID != principal.ProbeID || payload.JobID != jobID ||
		payload.RegionCode != principal.RegionCode || payload.CountryCode != principal.CountryCode ||
		payload.NetworkName != principal.NetworkName || payload.AgentVersion != principal.AgentVersion {
		return SubmitResult{}, ErrForbidden
	}
	if payload.StartedAt.IsZero() || payload.FinishedAt.Before(payload.StartedAt) ||
		payload.FinishedAt.Sub(payload.StartedAt) > s.cfg.ProbeJobTTL ||
		abs(payload.ClockOffsetMS) > int(s.cfg.ProbeMaxClockSkew.Milliseconds()) {
		return SubmitResult{}, ErrClockSkew
	}
	now := s.now().UTC()
	if durationAbs(now.Sub(payload.FinishedAt.UTC())) > s.cfg.ProbeMaxClockSkew+s.cfg.ProbeJobTTL {
		return SubmitResult{}, ErrClockSkew
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return SubmitResult{}, fmt.Errorf("begin remote result transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var runID, domainID, probeID uuid.UUID
	var status, nonce, policyVersion string
	var expiresAt time.Time
	var leaseUntil *time.Time
	err = tx.QueryRow(ctx, `
		SELECT monitoring_run_id, domain_id, probe_node_id, status::text, nonce, policy_version, expires_at, lease_until
		FROM remote_probe_jobs WHERE id = $1 FOR UPDATE
	`, jobID).Scan(&runID, &domainID, &probeID, &status, &nonce, &policyVersion, &expiresAt, &leaseUntil)
	if err == pgx.ErrNoRows {
		return SubmitResult{}, ErrNotFound
	}
	if err != nil {
		return SubmitResult{}, fmt.Errorf("load leased remote job: %w", err)
	}
	if status == "completed" {
		return SubmitResult{}, ErrReplay
	}
	if status != "leased" || probeID != principal.ProbeID || payload.RunID != runID || payload.Nonce != nonce {
		return SubmitResult{}, ErrForbidden
	}
	if !expiresAt.After(now) || leaseUntil == nil || !leaseUntil.After(now) {
		return SubmitResult{}, ErrExpired
	}
	remoteDNS := normalizeRemoteDNS(payload.DNS)
	remoteDecision := classification.Classify(remoteDNS, nil, payload.HTTP)
	resultID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO monitoring_results (
			id, monitoring_run_id, domain_id, probe_node_id, vantage_type, vantage_key,
			observed_availability, dns_status, http_status, redirect_status, isp_status, tls_status, content_status,
			initial_http_status_code, final_http_status_code, final_target_status, failure_stage, error_code, error_message,
			confidence_score, confidence_level, policy_version, checked_at, completed_at
		) VALUES ($1,$2,$3,$4,'remote',$5,$6,$7,$8,$9,'UNKNOWN',$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)
	`, resultID, runID, domainID, probeID, "remote:"+probeID.String(), remoteDecision.Availability, remoteDecision.DNS,
		remoteDecision.HTTP, remoteDecision.Redirect, remoteDecision.TLS, remoteDecision.Content,
		nullableNumber(remoteDecision.InitialHTTP), nullableNumber(remoteDecision.FinalHTTP), nullableText(remoteDecision.FinalTarget),
		nullableText(remoteDecision.FailureStage), nullableText(remoteDecision.ErrorCode), nullableText(remoteDecision.ErrorMessage),
		remoteDecision.Confidence, remoteDecision.ConfidenceLevel, policyVersion, payload.FinishedAt.UTC(), now)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("insert remote monitoring result: %w", err)
	}
	for _, result := range remoteDNS {
		if err := monitor.PersistDNSResult(ctx, tx, resultID, result); err != nil {
			return SubmitResult{}, err
		}
	}
	for _, attempt := range payload.HTTP.HTTPS {
		if err := monitor.PersistHTTPAttempt(ctx, tx, resultID, "https", "remote_system", attempt); err != nil {
			return SubmitResult{}, err
		}
	}
	for _, attempt := range payload.HTTP.HTTP {
		if err := monitor.PersistHTTPAttempt(ctx, tx, resultID, "http", "remote_system", attempt); err != nil {
			return SubmitResult{}, err
		}
	}
	signature, _ := probeprotocol.DecodeSignature(envelope.Signature)
	rawEnvelope, err := json.Marshal(envelope)
	if err != nil {
		return SubmitResult{}, ErrInvalidRequest
	}
	clockSkewMS := int(now.Sub(payload.FinishedAt.UTC()).Milliseconds())
	_, err = tx.Exec(ctx, `
		INSERT INTO remote_probe_results (
			monitoring_result_id, probe_node_id, job_id, protocol_version, nonce, payload_hash,
			signature, signature_valid, probe_started_at, probe_finished_at, server_received_at,
			clock_skew_ms, raw_envelope
		) VALUES ($1,$2,$3,$4,$5,$6,$7,true,$8,$9,$10,$11,$12::jsonb)
	`, resultID, probeID, jobID, payload.ProtocolVersion, payload.Nonce, payloadHash[:], signature,
		payload.StartedAt.UTC(), payload.FinishedAt.UTC(), now, clockSkewMS, string(rawEnvelope))
	if err != nil {
		if strings.Contains(err.Error(), "remote_probe_results_nonce_idx") || strings.Contains(err.Error(), "remote_probe_results_job_id_key") {
			return SubmitResult{}, ErrReplay
		}
		return SubmitResult{}, fmt.Errorf("insert signed remote result: %w", err)
	}
	ispInput, localResultID, err := s.loadISPInput(ctx, tx, domainID, runID, resultID, remoteDecision, principal, now)
	if err != nil {
		return SubmitResult{}, err
	}
	ispDecision := ispclass.Classify(ispInput)
	if _, err := tx.Exec(ctx, `UPDATE monitoring_results SET isp_status = $2 WHERE id = $1`, resultID, ispDecision.Status); err != nil {
		return SubmitResult{}, fmt.Errorf("update remote ISP classification: %w", err)
	}
	for _, reason := range ispDecision.Reasons {
		evidenceRefs, _ := json.Marshal([]map[string]any{{"local_result_id": localResultID}, {"remote_result_id": resultID}, {"probe_id": probeID}})
		if _, err := tx.Exec(ctx, `
			INSERT INTO classification_reasons
			(monitoring_run_id, monitoring_result_id, dimension, reason_code, score_delta, evidence_refs)
			VALUES ($1,$2,'isp',$3,0,$4::jsonb)
		`, runID, resultID, reason, string(evidenceRefs)); err != nil {
			return SubmitResult{}, fmt.Errorf("insert ISP reason: %w", err)
		}
	}
	var previousISP string
	if err := tx.QueryRow(ctx, `SELECT current_isp_status::text FROM domains WHERE id = $1 FOR UPDATE`, domainID).Scan(&previousISP); err != nil {
		return SubmitResult{}, fmt.Errorf("lock domain ISP state: %w", err)
	}
	if previousISP != ispDecision.Status {
		reasonsJSON, _ := json.Marshal(ispDecision.Reasons)
		if _, err := tx.Exec(ctx, `
			INSERT INTO domain_status_history
			(domain_id, dimension, previous_value, current_value, confidence_score, policy_version, reason_codes, supporting_run_ids, effective_at)
			VALUES ($1,'isp',$2,$3,$4,$5,$6::jsonb,$7,$8)
		`, domainID, previousISP, ispDecision.Status, ispDecision.Confidence, policyVersion, string(reasonsJSON), []uuid.UUID{runID}, now); err != nil {
			return SubmitResult{}, fmt.Errorf("insert ISP status history: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE domains SET current_isp_status = $2 WHERE id = $1`, domainID, ispDecision.Status); err != nil {
		return SubmitResult{}, fmt.Errorf("update effective ISP state: %w", err)
	}
	command, err := tx.Exec(ctx, `
		UPDATE remote_probe_jobs SET status = 'completed', completed_at = $2, lease_until = NULL,
		last_error_code = NULL, last_error_message = NULL WHERE id = $1 AND status = 'leased'
	`, jobID, now)
	if err != nil {
		return SubmitResult{}, fmt.Errorf("complete remote probe job: %w", err)
	}
	if command.RowsAffected() != 1 {
		return SubmitResult{}, ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return SubmitResult{}, fmt.Errorf("commit remote result: %w", err)
	}
	return SubmitResult{MonitoringResultID: resultID, ISP: ispDecision}, nil
}

func (s *Service) loadISPInput(ctx context.Context, tx pgx.Tx, domainID, runID, _ uuid.UUID, remoteDecision classification.Decision, principal Principal, now time.Time) (ispclass.Input, uuid.UUID, error) {
	input := ispclass.Input{
		RemoteAvailability: remoteDecision.Availability, RemoteFresh: true,
		ProbeHealthy: principal.Status == "ONLINE", ClockHealthy: true,
	}
	var localResultID uuid.UUID
	var localFinalHTTP *int
	var localDNSStatus string
	err := tx.QueryRow(ctx, `
		SELECT id, observed_availability::text, dns_status::text, final_http_status_code
		FROM monitoring_results WHERE monitoring_run_id = $1 AND vantage_type = 'local'
		ORDER BY checked_at DESC LIMIT 1
	`, runID).Scan(&localResultID, &input.LocalAvailability, &localDNSStatus, &localFinalHTTP)
	if err != nil {
		return input, uuid.Nil, fmt.Errorf("load local result for ISP classification: %w", err)
	}
	input.LocalDNSDiscrepancy = localDNSStatus == "DISCREPANCY"
	if localFinalHTTP != nil {
		input.PossibleGeoPolicy = *localFinalHTTP == 451
		input.PossibleWAF = *localFinalHTTP == 403 || *localFinalHTTP == 429
	}
	var pinnedStatus, pinnedContent, pinnedError string
	var pinnedHTTP *int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(final_status_code, 0), content_status::text, COALESCE(error_code, '')
		FROM http_checks WHERE monitoring_result_id = $1 AND resolver_mode = 'doh_pinned'
		ORDER BY (scheme = 'http') DESC, attempt_no DESC LIMIT 1
	`, localResultID).Scan(&pinnedHTTP, &pinnedContent, &pinnedError)
	if err == nil {
		if pinnedHTTP != nil && *pinnedHTTP >= 200 && *pinnedHTTP < 300 && (pinnedContent == "VALID_HTML" || pinnedContent == "VALID_NON_HTML") {
			pinnedStatus = "ACTIVE"
		} else {
			pinnedStatus = "UNAVAILABLE"
		}
		input.PinnedAvailability = pinnedStatus
		input.PinnedFailureStage = errorStage(pinnedError)
	} else if err != pgx.ErrNoRows {
		return input, uuid.Nil, fmt.Errorf("load DoH-pinned result: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT date_bin('5 minutes', result.checked_at, timestamptz '2001-01-01'))
		FROM monitoring_results result
		JOIN monitoring_runs run ON run.id = result.monitoring_run_id
		WHERE result.domain_id = $1 AND result.vantage_type = 'local'
		  AND result.observed_availability = 'UNAVAILABLE'
		  AND run.trigger_type <> 'manual' AND result.checked_at >= $2
	`, domainID, now.Add(-s.cfg.ProbeEvidenceFresh)).Scan(&input.LocalFailureCount, &input.LocalFailureBuckets); err != nil {
		return input, uuid.Nil, fmt.Errorf("count qualifying local failures: %w", err)
	}
	var doHSetCount, doHDistinct int
	if err := tx.QueryRow(ctx, `
		WITH answer_sets AS (
			SELECT result.id,
			       string_agg(dns_check.query_type || ':' || answer.rr_type || ':' || answer.rr_value, ',' ORDER BY dns_check.query_type, answer.rr_type, answer.rr_value) AS answer_set
			FROM monitoring_results result
			JOIN dns_checks dns_check ON dns_check.monitoring_result_id = result.id AND dns_check.resolver_type = 'CLOUDFLARE_DOH'
			JOIN dns_answers answer ON answer.dns_check_id = dns_check.id AND answer.rr_type IN ('A','AAAA')
			WHERE result.domain_id = $1 AND result.vantage_type = 'local' AND result.checked_at >= $2
			  AND dns_check.rcode = 'NOERROR' AND dns_check.error_code IS NULL
			GROUP BY result.id
		)
		SELECT COUNT(*), COUNT(DISTINCT answer_set) FROM answer_sets
	`, domainID, now.Add(-s.cfg.ProbeEvidenceFresh)).Scan(&doHSetCount, &doHDistinct); err != nil {
		return input, uuid.Nil, fmt.Errorf("load DoH stability evidence: %w", err)
	}
	input.DoHStable = doHSetCount >= 2 && doHDistinct == 1
	var remoteHashes int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(DISTINCT result.id), COUNT(DISTINCT encode(http.body_sha256, 'hex'))
		FROM monitoring_results result
		JOIN probe_nodes node ON node.id = result.probe_node_id
		JOIN http_checks http ON http.monitoring_result_id = result.id AND http.resolver_mode = 'remote_system'
		WHERE result.domain_id = $1 AND result.vantage_type = 'remote' AND node.region_code = $2
		  AND result.observed_availability = 'ACTIVE' AND result.checked_at >= $3
		  AND http.final_status_code BETWEEN 200 AND 299
		  AND http.content_status IN ('VALID_HTML','VALID_NON_HTML') AND http.body_sha256 IS NOT NULL
	`, domainID, s.cfg.ProbeRequiredRegion, now.Add(-s.cfg.ProbeEvidenceFresh)).Scan(&input.RemoteSuccessCount, &remoteHashes); err != nil {
		return input, uuid.Nil, fmt.Errorf("load remote success evidence: %w", err)
	}
	input.RemoteBodyHashStable = input.RemoteSuccessCount >= 2 && remoteHashes == 1
	return input, localResultID, nil
}

func normalizeRemoteDNS(values []dnscheck.Result) []dnscheck.Result {
	result := append([]dnscheck.Result(nil), values...)
	for index := range result {
		result[index].Resolver = dnscheck.ResolverRemoteSystem
		if strings.TrimSpace(result[index].Endpoint) == "" {
			result[index].Endpoint = "system"
		}
	}
	return result
}

func nullableNumber(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func errorStage(code string) string {
	switch {
	case strings.HasPrefix(code, "DNS_"):
		return "dns"
	case strings.HasPrefix(code, "TCP_") || strings.HasPrefix(code, "SSRF_"):
		return "tcp"
	case strings.HasPrefix(code, "TLS_"):
		return "tls"
	case strings.HasPrefix(code, "HTTP_"):
		return "http"
	case strings.HasPrefix(code, "CONTENT_"):
		return "content"
	default:
		return ""
	}
}

func durationAbs(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}
