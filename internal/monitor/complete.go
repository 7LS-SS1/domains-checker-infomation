package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/classification"
	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/httpcheck"
	"domainmonitor/internal/tlscheck"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) CompleteRun(ctx context.Context, execution Execution, evidence Evidence) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin monitoring result transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current classification.EffectiveInput
	err = tx.QueryRow(ctx, `
		SELECT current_availability_status::text, current_dns_status::text, current_http_status::text,
		       current_redirect_status::text, current_isp_status::text, current_tls_status::text,
		       current_content_status::text, current_confidence_score,
		       consecutive_failure_count, consecutive_success_count
		FROM domains WHERE id = $1 FOR UPDATE
	`, execution.Run.DomainID).Scan(
		&current.CurrentAvailability, &current.CurrentDNS, &current.CurrentHTTP, &current.CurrentRedirect,
		&current.CurrentISP, &current.CurrentTLS, &current.CurrentContent, &current.CurrentConfidence,
		&current.FailureStreak, &current.SuccessStreak,
	)
	if err != nil {
		return fmt.Errorf("lock domain monitoring state: %w", err)
	}

	resultID := uuid.New()
	decision := evidence.Decision
	checkedAt := evidence.CheckedAt.UTC()
	if checkedAt.IsZero() {
		checkedAt = s.now().UTC()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO monitoring_results (
			id, monitoring_run_id, domain_id, vantage_type, vantage_key,
			observed_availability, dns_status, http_status, redirect_status, isp_status, tls_status, content_status,
			initial_http_status_code, final_http_status_code, final_target_status,
			failure_stage, error_code, error_message, confidence_score, confidence_level,
			policy_version, checked_at, completed_at
		) VALUES ($1,$2,$3,'local','local-primary',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,now())
	`, resultID, execution.Run.ID, execution.Run.DomainID, decision.Availability, decision.DNS, decision.HTTP,
		decision.Redirect, decision.ISP, decision.TLS, decision.Content,
		nullableInt(decision.InitialHTTP), nullableInt(decision.FinalHTTP), nullableString(decision.FinalTarget),
		nullableString(decision.FailureStage), nullableString(decision.ErrorCode), nullableString(decision.ErrorMessage),
		decision.Confidence, decision.ConfidenceLevel, execution.Run.PolicyVersion, checkedAt)
	if err != nil {
		return fmt.Errorf("insert monitoring result: %w", err)
	}

	for _, result := range append(append([]dnscheck.Result{}, evidence.LocalDNS...), evidence.AlternateDNS...) {
		if err := PersistDNSResult(ctx, tx, resultID, result); err != nil {
			return err
		}
	}
	for _, attempt := range evidence.HTTP.HTTPS {
		if err := PersistHTTPAttempt(ctx, tx, resultID, "https", "system", attempt); err != nil {
			return err
		}
	}
	for _, attempt := range evidence.HTTP.HTTP {
		if err := PersistHTTPAttempt(ctx, tx, resultID, "http", "system", attempt); err != nil {
			return err
		}
	}
	if evidence.PinnedHTTP != nil {
		for _, attempt := range evidence.PinnedHTTP.HTTPS {
			if err := PersistHTTPAttempt(ctx, tx, resultID, "https", "doh_pinned", attempt); err != nil {
				return err
			}
		}
		for _, attempt := range evidence.PinnedHTTP.HTTP {
			if err := PersistHTTPAttempt(ctx, tx, resultID, "http", "doh_pinned", attempt); err != nil {
				return err
			}
		}
	}
	for _, reason := range decision.Reasons {
		if _, err := tx.Exec(ctx, `
			INSERT INTO classification_reasons (
				monitoring_run_id, monitoring_result_id, dimension, reason_code, score_delta, evidence_refs
			) VALUES ($1,$2,'availability',$3,0,'[]'::jsonb)
		`, execution.Run.ID, resultID, reason); err != nil {
			return fmt.Errorf("insert classification reason: %w", err)
		}
	}

	current.Observed = decision
	current.Qualifying = execution.Run.TriggerType != "manual"
	current.OpenFailures = s.cfg.IncidentOpenFailures
	current.CloseSuccesses = s.cfg.IncidentCloseSuccess
	effective := classification.Advance(current)
	if _, err := tx.Exec(ctx, `
		UPDATE domains SET
			current_availability_status = $2, current_dns_status = $3, current_http_status = $4,
			current_redirect_status = $5, current_isp_status = $6, current_tls_status = $7,
			current_content_status = $8, current_confidence_score = $9,
			consecutive_failure_count = $10, consecutive_success_count = $11,
			current_failure_stage = $12, current_error_code = $13, last_checked_at = $14
		WHERE id = $1
	`, execution.Run.DomainID, effective.Availability, effective.DNS, effective.HTTP, effective.Redirect,
		effective.ISP, effective.TLS, effective.Content, effective.Confidence,
		effective.FailureStreak, effective.SuccessStreak, nullableString(effective.FailureStage),
		nullableString(effective.ErrorCode), checkedAt); err != nil {
		return fmt.Errorf("update effective domain state: %w", err)
	}
	if err := persistHistory(ctx, tx, execution.Run, current, effective, decision.Reasons, checkedAt); err != nil {
		return err
	}
	if current.Qualifying {
		if err := applyIncident(ctx, tx, execution.Run, decision, effective, s.cfg.IncidentOpenFailures, s.cfg.IncidentCloseSuccess, checkedAt); err != nil {
			return err
		}
	}
	command, err := tx.Exec(ctx, `
		UPDATE monitoring_runs
		SET status = 'completed', completed_at = now(), worker_id = NULL, heartbeat_at = NULL,
		    last_error_code = NULL, last_error_message = NULL
		WHERE id = $1 AND status = 'running'
	`, execution.Run.ID)
	if err != nil {
		return fmt.Errorf("complete monitoring run: %w", err)
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("complete monitoring run: execution lease lost")
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit monitoring result: %w", err)
	}
	return nil
}

func PersistDNSResult(ctx context.Context, tx pgx.Tx, resultID uuid.UUID, result dnscheck.Result) error {
	checkID := uuid.New()
	raw, err := json.Marshal(map[string]any{"transport": result.Transport})
	if err != nil {
		return fmt.Errorf("marshal DNS raw evidence: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO dns_checks (
			id, monitoring_result_id, resolver_type, resolver_endpoint, query_name, query_type,
			attempt_no, rcode, truncated, authoritative, duration_us, error_code, error_message,
			raw_evidence, checked_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::jsonb,$15)
	`, checkID, resultID, string(result.Resolver), result.Endpoint, result.QueryName, string(result.QueryType),
		result.Attempt, nullableString(result.RCode), result.Truncated, result.Authoritative,
		max64(0, result.Duration.Microseconds()), nullableString(string(result.ErrorCode)), nullableString(result.ErrorMessage),
		string(raw), result.CheckedAt.UTC())
	if err != nil {
		return fmt.Errorf("insert DNS check: %w", err)
	}
	for order, answer := range result.Answers {
		if _, err := tx.Exec(ctx, `
			INSERT INTO dns_answers (dns_check_id, answer_order, rr_name, rr_type, rr_value, ttl_seconds)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, checkID, order, answer.Name, string(answer.Type), answer.Value, answer.TTL); err != nil {
			return fmt.Errorf("insert DNS answer: %w", err)
		}
	}
	return nil
}

func PersistHTTPAttempt(ctx context.Context, tx pgx.Tx, resultID uuid.UUID, scheme, resolverMode string, attempt httpcheck.Attempt) error {
	checkID := uuid.New()
	headers, err := json.Marshal(attempt.Headers)
	if err != nil {
		return fmt.Errorf("marshal HTTP selected headers: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO http_checks (
			id, monitoring_result_id, scheme, resolver_mode, request_url, effective_url, protocol, attempt_no,
			initial_status_code, final_status_code, dns_duration_us, connect_duration_us, tls_duration_us,
			ttfb_duration_us, total_duration_us, content_type, declared_content_length, body_size,
			body_sha256, hash_complete, body_excerpt, title, content_status, server_header,
			selected_headers, error_code, error_message, checked_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25::jsonb,$26,$27,$28)
	`, checkID, resultID, scheme, resolverMode, attempt.RequestURL, nullableString(attempt.EffectiveURL), nullableString(attempt.Protocol), attempt.Attempt,
		nullableInt(attempt.InitialStatusCode), nullableInt(attempt.FinalStatusCode), durationMicroseconds(attempt.Timing.DNS),
		durationMicroseconds(attempt.Timing.Connect), durationMicroseconds(attempt.Timing.TLS), durationMicroseconds(attempt.Timing.TTFB),
		max64(0, attempt.Timing.Total.Microseconds()), nullableString(attempt.Content.ContentType), attempt.Content.DeclaredContentLength,
		attempt.Content.BodySize, nullableBytes(attempt.Content.BodySHA256), attempt.Content.HashComplete, nullableBytes(attempt.Content.Excerpt),
		nullableString(attempt.Content.Title), defaultStatus(attempt.Content.Status), nullableString(attempt.ServerHeader), string(headers),
		nullableString(string(attempt.ErrorCode)), nullableString(attempt.ErrorMessage), attempt.CheckedAt.UTC())
	if err != nil {
		return fmt.Errorf("insert HTTP check: %w", err)
	}
	for _, hop := range attempt.Redirects {
		if _, err := tx.Exec(ctx, `
			INSERT INTO redirect_hops (
				http_check_id, hop_number, source_url, status_code, location, resolved_target,
				cross_domain, https_downgrade, duration_us, error_code
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		`, checkID, hop.Hop, hop.SourceURL, hop.StatusCode, hop.Location, nullableString(hop.ResolvedTarget),
			hop.CrossDomain, hop.HTTPSDowngrade, max64(0, hop.Duration.Microseconds()), nullableString(string(hop.ErrorCode))); err != nil {
			return fmt.Errorf("insert redirect hop: %w", err)
		}
	}
	if attempt.TLS != nil {
		if err := persistTLSCheck(ctx, tx, checkID, *attempt.TLS); err != nil {
			return err
		}
	}
	return nil
}

func persistTLSCheck(ctx context.Context, tx pgx.Tx, httpCheckID uuid.UUID, result tlscheck.Result) error {
	sans, err := json.Marshal(result.SANs)
	if err != nil {
		return fmt.Errorf("marshal TLS SANs: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO tls_checks (
			http_check_id, server_name, remote_address, tls_version, cipher_suite,
			certificate_subject, certificate_issuer, certificate_serial_hash, sans,
			valid_from, valid_until, certificate_expiration_days, hostname_valid, chain_valid,
			tls_status, diagnostic_only, error_code, error_message, checked_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`, httpCheckID, result.ServerName, nullableString(result.RemoteAddress), nullableString(result.TLSVersion),
		nullableString(result.CipherSuite), nullableString(result.CertificateSubject), nullableString(result.CertificateIssuer),
		nullableBytes(result.SerialHash), string(sans), result.ValidFrom, result.ValidUntil, result.ExpirationDays,
		result.HostnameValid, result.ChainValid, defaultStatus(result.Status), result.DiagnosticOnly,
		nullableString(string(result.ErrorCode)), nullableString(result.ErrorMessage), result.CheckedAt.UTC())
	if err != nil {
		return fmt.Errorf("insert TLS check: %w", err)
	}
	return nil
}

func persistHistory(ctx context.Context, tx pgx.Tx, run Run, before classification.EffectiveInput, after classification.EffectiveState, reasons []string, effectiveAt time.Time) error {
	beforeValues := map[string]string{
		"availability": before.CurrentAvailability, "dns": before.CurrentDNS, "http": before.CurrentHTTP,
		"redirect": before.CurrentRedirect, "isp": before.CurrentISP, "tls": before.CurrentTLS, "content": before.CurrentContent,
	}
	afterValues := map[string]string{
		"availability": after.Availability, "dns": after.DNS, "http": after.HTTP,
		"redirect": after.Redirect, "isp": after.ISP, "tls": after.TLS, "content": after.Content,
	}
	encodedReasons, err := json.Marshal(reasons)
	if err != nil {
		return fmt.Errorf("marshal status history reasons: %w", err)
	}
	for _, dimension := range after.ChangedDimensions {
		_, err := tx.Exec(ctx, `
			INSERT INTO domain_status_history (
				domain_id, dimension, previous_value, current_value, confidence_score,
				policy_version, reason_codes, supporting_run_ids, effective_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)
		`, run.DomainID, dimension, nullableString(beforeValues[dimension]), afterValues[dimension], after.Confidence,
			run.PolicyVersion, string(encodedReasons), []uuid.UUID{run.ID}, effectiveAt)
		if err != nil {
			return fmt.Errorf("insert %s status history: %w", dimension, err)
		}
	}
	return nil
}

func applyIncident(ctx context.Context, tx pgx.Tx, run Run, decision classification.Decision, state classification.EffectiveState, openFailures, closeSuccesses int, checkedAt time.Time) error {
	var incidentID uuid.UUID
	var incidentStatus string
	err := tx.QueryRow(ctx, `
		SELECT id, status::text FROM incidents
		WHERE domain_id = $1 AND status IN ('open','acknowledged')
		FOR UPDATE
	`, run.DomainID).Scan(&incidentID, &incidentStatus)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("load active incident: %w", err)
	}
	hasIncident := err == nil
	if decision.Availability == "ACTIVE" && hasIncident {
		if _, err := tx.Exec(ctx, `UPDATE incidents SET close_success_count = $2 WHERE id = $1`, incidentID, state.SuccessStreak); err != nil {
			return fmt.Errorf("update incident recovery streak: %w", err)
		}
		if err := insertIncidentEvent(ctx, tx, incidentID, "success_observed", run.ID, map[string]any{"success_streak": state.SuccessStreak}); err != nil {
			return err
		}
		if state.Availability == "ACTIVE" && state.SuccessStreak >= closeSuccesses {
			if _, err := tx.Exec(ctx, `
				UPDATE incidents SET status = 'closed', closed_at = $2, closed_by_run_id = $3 WHERE id = $1
			`, incidentID, checkedAt, run.ID); err != nil {
				return fmt.Errorf("close incident: %w", err)
			}
			if err := insertIncidentEvent(ctx, tx, incidentID, "closed", run.ID, map[string]any{"success_streak": state.SuccessStreak}); err != nil {
				return err
			}
		}
		return nil
	}
	if decision.Availability != "UNAVAILABLE" && decision.Availability != "DEGRADED" {
		return nil
	}
	if hasIncident {
		if _, err := tx.Exec(ctx, `
			UPDATE incidents SET open_failure_count = GREATEST(open_failure_count, $2),
			failure_stage = $3, error_code = $4, close_success_count = 0 WHERE id = $1
		`, incidentID, state.FailureStreak, nullableString(decision.FailureStage), nullableString(decision.ErrorCode)); err != nil {
			return fmt.Errorf("update incident failure streak: %w", err)
		}
		return insertIncidentEvent(ctx, tx, incidentID, "failure_observed", run.ID, map[string]any{"failure_streak": state.FailureStreak, "error_code": decision.ErrorCode})
	}
	if state.Availability != "UNAVAILABLE" || state.FailureStreak < openFailures {
		return nil
	}
	incidentID = uuid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO incidents (
			id, domain_id, status, failure_stage, error_code, open_failure_count,
			opened_at, opened_by_run_id
		) VALUES ($1,$2,'open',$3,$4,$5,$6,$7)
	`, incidentID, run.DomainID, nullableString(decision.FailureStage), nullableString(decision.ErrorCode),
		state.FailureStreak, checkedAt, run.ID); err != nil {
		return fmt.Errorf("open incident: %w", err)
	}
	return insertIncidentEvent(ctx, tx, incidentID, "opened", run.ID, map[string]any{"failure_streak": state.FailureStreak, "error_code": decision.ErrorCode})
}

func insertIncidentEvent(ctx context.Context, tx pgx.Tx, incidentID uuid.UUID, eventType string, runID uuid.UUID, details map[string]any) error {
	encoded, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal incident event: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO incident_events (incident_id, event_type, monitoring_run_id, details)
		VALUES ($1,$2,$3,$4::jsonb)
	`, incidentID, eventType, runID, string(encoded))
	if err != nil {
		return fmt.Errorf("insert incident event: %w", err)
	}
	return nil
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func durationMicroseconds(value time.Duration) any {
	if value <= 0 {
		return nil
	}
	return value.Microseconds()
}

func max64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func defaultStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "UNKNOWN"
	}
	return value
}
