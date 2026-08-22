package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListRuns(ctx context.Context, domainID uuid.UUID, page, pageSize int) (RunPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domains WHERE id = $1)`, domainID).Scan(&exists); err != nil {
		return RunPage{}, fmt.Errorf("check domain monitoring history: %w", err)
	}
	if !exists {
		return RunPage{}, ErrDomainNotFound
	}
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM monitoring_runs WHERE domain_id = $1`, domainID).Scan(&total); err != nil {
		return RunPage{}, fmt.Errorf("count monitoring runs: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+runColumns+`
		FROM monitoring_runs r
		JOIN domains d ON d.id = r.domain_id
		WHERE r.domain_id = $1
		ORDER BY r.scheduled_for DESC, r.id DESC
		LIMIT $2 OFFSET $3
	`, domainID, pageSize, (page-1)*pageSize)
	if err != nil {
		return RunPage{}, fmt.Errorf("list monitoring runs: %w", err)
	}
	defer rows.Close()
	items := make([]Run, 0, pageSize)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return RunPage{}, err
		}
		items = append(items, run)
	}
	if err := rows.Err(); err != nil {
		return RunPage{}, fmt.Errorf("iterate monitoring runs: %w", err)
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return RunPage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
}

func (s *Service) GetRunDetail(ctx context.Context, runID uuid.UUID) (RunDetail, error) {
	run, err := s.GetRun(ctx, runID)
	if err != nil {
		return RunDetail{}, err
	}
	detail := RunDetail{Run: run, Results: []Result{}, DNSChecks: []DNSCheck{}, HTTPChecks: []HTTPCheck{}}
	resultRows, err := s.pool.Query(ctx, `
		SELECT id, monitoring_run_id, domain_id, vantage_type, vantage_key,
		       observed_availability::text, dns_status::text, http_status::text, redirect_status::text,
		       isp_status::text, tls_status::text, content_status::text,
		       initial_http_status_code, final_http_status_code, final_target_status::text,
		       failure_stage, error_code, error_message, confidence_score, confidence_level::text,
		       policy_version, checked_at, completed_at
		FROM monitoring_results WHERE monitoring_run_id = $1 ORDER BY checked_at, id
	`, runID)
	if err != nil {
		return RunDetail{}, fmt.Errorf("query monitoring results: %w", err)
	}
	for resultRows.Next() {
		var result Result
		if err := resultRows.Scan(
			&result.ID, &result.RunID, &result.DomainID, &result.VantageType, &result.VantageKey,
			&result.Availability, &result.DNS, &result.HTTP, &result.Redirect, &result.ISP, &result.TLS,
			&result.Content, &result.InitialHTTP, &result.FinalHTTP, &result.FinalTarget, &result.FailureStage,
			&result.ErrorCode, &result.ErrorMessage, &result.Confidence, &result.ConfidenceLevel,
			&result.PolicyVersion, &result.CheckedAt, &result.CompletedAt,
		); err != nil {
			resultRows.Close()
			return RunDetail{}, fmt.Errorf("scan monitoring result: %w", err)
		}
		detail.Results = append(detail.Results, result)
	}
	if err := resultRows.Err(); err != nil {
		resultRows.Close()
		return RunDetail{}, fmt.Errorf("iterate monitoring results: %w", err)
	}
	resultRows.Close()
	if err := s.loadDNSChecks(ctx, runID, &detail); err != nil {
		return RunDetail{}, err
	}
	if err := s.loadHTTPChecks(ctx, runID, &detail); err != nil {
		return RunDetail{}, err
	}
	return detail, nil
}

func (s *Service) loadDNSChecks(ctx context.Context, runID uuid.UUID, detail *RunDetail) error {
	rows, err := s.pool.Query(ctx, `
		SELECT dns_check.id, dns_check.monitoring_result_id, dns_check.resolver_type, dns_check.resolver_endpoint,
		       dns_check.query_name, dns_check.query_type, dns_check.attempt_no, dns_check.rcode, dns_check.truncated,
		       dns_check.authoritative, dns_check.duration_us, dns_check.error_code, dns_check.error_message,
		       dns_check.raw_evidence, dns_check.checked_at
		FROM dns_checks dns_check
		JOIN monitoring_results result ON result.id = dns_check.monitoring_result_id
		WHERE result.monitoring_run_id = $1
		ORDER BY dns_check.checked_at, dns_check.resolver_type, dns_check.query_type, dns_check.attempt_no
	`, runID)
	if err != nil {
		return fmt.Errorf("query DNS checks: %w", err)
	}
	indices := map[uuid.UUID]int{}
	ids := []uuid.UUID{}
	for rows.Next() {
		var check DNSCheck
		if err := rows.Scan(&check.ID, &check.ResultID, &check.ResolverType, &check.Endpoint, &check.QueryName,
			&check.QueryType, &check.Attempt, &check.RCode, &check.Truncated, &check.Authoritative,
			&check.DurationUS, &check.ErrorCode, &check.ErrorMessage, &check.RawEvidence, &check.CheckedAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan DNS check: %w", err)
		}
		check.Answers = []DNSAnswer{}
		indices[check.ID] = len(detail.DNSChecks)
		ids = append(ids, check.ID)
		detail.DNSChecks = append(detail.DNSChecks, check)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate DNS checks: %w", err)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil
	}
	answerRows, err := s.pool.Query(ctx, `
		SELECT dns_check_id, rr_name, rr_type, rr_value, ttl_seconds
		FROM dns_answers WHERE dns_check_id = ANY($1)
		ORDER BY dns_check_id, answer_order
	`, ids)
	if err != nil {
		return fmt.Errorf("query DNS answers: %w", err)
	}
	defer answerRows.Close()
	for answerRows.Next() {
		var checkID uuid.UUID
		var answer DNSAnswer
		if err := answerRows.Scan(&checkID, &answer.Name, &answer.Type, &answer.Value, &answer.TTL); err != nil {
			return fmt.Errorf("scan DNS answer: %w", err)
		}
		if index, ok := indices[checkID]; ok {
			detail.DNSChecks[index].Answers = append(detail.DNSChecks[index].Answers, answer)
		}
	}
	return answerRows.Err()
}

func (s *Service) loadHTTPChecks(ctx context.Context, runID uuid.UUID, detail *RunDetail) error {
	rows, err := s.pool.Query(ctx, `
		SELECT http_check.id, http_check.monitoring_result_id, http_check.scheme, http_check.resolver_mode, http_check.request_url, http_check.effective_url,
		       http_check.protocol, http_check.attempt_no, http_check.initial_status_code, http_check.final_status_code,
		       http_check.dns_duration_us, http_check.connect_duration_us, http_check.tls_duration_us, http_check.ttfb_duration_us,
		       http_check.total_duration_us, http_check.content_type, http_check.declared_content_length, http_check.body_size,
		       http_check.body_sha256, http_check.hash_complete, http_check.body_excerpt, http_check.title, http_check.content_status::text,
		       http_check.server_header, http_check.selected_headers, http_check.error_code, http_check.error_message, http_check.checked_at
		FROM http_checks http_check
		JOIN monitoring_results result ON result.id = http_check.monitoring_result_id
		WHERE result.monitoring_run_id = $1
		ORDER BY http_check.checked_at, http_check.scheme DESC, http_check.attempt_no
	`, runID)
	if err != nil {
		return fmt.Errorf("query HTTP checks: %w", err)
	}
	indices := map[uuid.UUID]int{}
	ids := []uuid.UUID{}
	for rows.Next() {
		var check HTTPCheck
		if err := rows.Scan(
			&check.ID, &check.ResultID, &check.Scheme, &check.ResolverMode, &check.RequestURL, &check.EffectiveURL,
			&check.Protocol, &check.Attempt, &check.InitialStatus, &check.FinalStatus,
			&check.DNSDurationUS, &check.ConnectDurationUS, &check.TLSDurationUS, &check.TTFBDurationUS,
			&check.TotalDurationUS, &check.ContentType, &check.DeclaredContentLength, &check.BodySize,
			&check.BodySHA256, &check.HashComplete, &check.BodyExcerpt, &check.Title, &check.ContentStatus,
			&check.ServerHeader, &check.SelectedHeaders, &check.ErrorCode, &check.ErrorMessage, &check.CheckedAt,
		); err != nil {
			rows.Close()
			return fmt.Errorf("scan HTTP check: %w", err)
		}
		check.Redirects = []RedirectHop{}
		indices[check.ID] = len(detail.HTTPChecks)
		ids = append(ids, check.ID)
		detail.HTTPChecks = append(detail.HTTPChecks, check)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate HTTP checks: %w", err)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil
	}
	if err := s.loadRedirects(ctx, ids, indices, detail); err != nil {
		return err
	}
	return s.loadTLSChecks(ctx, ids, indices, detail)
}

func (s *Service) loadRedirects(ctx context.Context, ids []uuid.UUID, indices map[uuid.UUID]int, detail *RunDetail) error {
	rows, err := s.pool.Query(ctx, `
		SELECT http_check_id, hop_number, source_url, status_code, location, resolved_target,
		       cross_domain, https_downgrade, duration_us, error_code
		FROM redirect_hops WHERE http_check_id = ANY($1)
		ORDER BY http_check_id, hop_number
	`, ids)
	if err != nil {
		return fmt.Errorf("query redirect hops: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var checkID uuid.UUID
		var hop RedirectHop
		if err := rows.Scan(&checkID, &hop.Hop, &hop.SourceURL, &hop.StatusCode, &hop.Location,
			&hop.ResolvedTarget, &hop.CrossDomain, &hop.HTTPSDowngrade, &hop.DurationUS, &hop.ErrorCode); err != nil {
			return fmt.Errorf("scan redirect hop: %w", err)
		}
		if index, ok := indices[checkID]; ok {
			detail.HTTPChecks[index].Redirects = append(detail.HTTPChecks[index].Redirects, hop)
		}
	}
	return rows.Err()
}

func (s *Service) loadTLSChecks(ctx context.Context, ids []uuid.UUID, indices map[uuid.UUID]int, detail *RunDetail) error {
	rows, err := s.pool.Query(ctx, `
		SELECT http_check_id, server_name, remote_address, tls_version, cipher_suite,
		       certificate_subject, certificate_issuer, certificate_serial_hash, sans,
		       valid_from, valid_until, certificate_expiration_days, hostname_valid, chain_valid,
		       tls_status::text, diagnostic_only, error_code, error_message, checked_at
		FROM tls_checks WHERE http_check_id = ANY($1)
		ORDER BY http_check_id, checked_at DESC
	`, ids)
	if err != nil {
		return fmt.Errorf("query TLS checks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var checkID uuid.UUID
		var check TLSCheck
		if err := rows.Scan(&checkID, &check.ServerName, &check.RemoteAddress, &check.TLSVersion,
			&check.CipherSuite, &check.CertificateSubject, &check.CertificateIssuer,
			&check.CertificateSerialHash, &check.SANs, &check.ValidFrom, &check.ValidUntil,
			&check.ExpirationDays, &check.HostnameValid, &check.ChainValid, &check.Status,
			&check.DiagnosticOnly, &check.ErrorCode, &check.ErrorMessage, &check.CheckedAt); err != nil {
			return fmt.Errorf("scan TLS check: %w", err)
		}
		if index, ok := indices[checkID]; ok && detail.HTTPChecks[index].TLSCheck == nil {
			detail.HTTPChecks[index].TLSCheck = &check
		}
	}
	return rows.Err()
}

func (s *Service) GetHistory(ctx context.Context, domainID uuid.UUID, since, until time.Time) (History, error) {
	if !until.After(since) {
		return History{}, fmt.Errorf("history window end must be after start")
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domains WHERE id = $1)`, domainID).Scan(&exists); err != nil {
		return History{}, fmt.Errorf("check history domain: %w", err)
	}
	if !exists {
		return History{}, ErrDomainNotFound
	}
	history := History{DomainID: domainID, Timeline: []HistoryEntry{}, Aggregate: HistoryAggregate{WindowStart: since, WindowEnd: until}}
	rows, err := s.pool.Query(ctx, `
		SELECT id, dimension, previous_value, current_value, confidence_score, policy_version,
		       reason_codes, supporting_run_ids, effective_at
		FROM domain_status_history
		WHERE domain_id = $1 AND effective_at >= $2 AND effective_at <= $3
		ORDER BY effective_at, id
		LIMIT 5000
	`, domainID, since, until)
	if err != nil {
		return History{}, fmt.Errorf("query domain status history: %w", err)
	}
	for rows.Next() {
		var entry HistoryEntry
		if err := rows.Scan(&entry.ID, &entry.Dimension, &entry.PreviousValue, &entry.CurrentValue,
			&entry.Confidence, &entry.PolicyVersion, &entry.ReasonCodes, &entry.SupportingRunIDs, &entry.EffectiveAt); err != nil {
			rows.Close()
			return History{}, fmt.Errorf("scan domain status history: %w", err)
		}
		history.Timeline = append(history.Timeline, entry)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return History{}, fmt.Errorf("iterate domain status history: %w", err)
	}
	rows.Close()
	if err := s.calculateHistoryAggregate(ctx, domainID, since, until, &history); err != nil {
		return History{}, err
	}
	return history, nil
}

func (s *Service) calculateHistoryAggregate(ctx context.Context, domainID uuid.UUID, since, until time.Time, history *History) error {
	state := "UNKNOWN"
	err := s.pool.QueryRow(ctx, `
		SELECT current_value FROM domain_status_history
		WHERE domain_id = $1 AND dimension = 'availability' AND effective_at < $2
		ORDER BY effective_at DESC, id DESC LIMIT 1
	`, domainID, since).Scan(&state)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load history window initial state: %w", err)
	}
	cursor := since
	changes := 0
	for _, entry := range history.Timeline {
		if entry.Dimension != "availability" {
			continue
		}
		accumulateState(&history.Aggregate, state, entry.EffectiveAt.Sub(cursor).Seconds())
		cursor = entry.EffectiveAt
		state = entry.CurrentValue
		changes++
	}
	accumulateState(&history.Aggregate, state, until.Sub(cursor).Seconds())
	history.Aggregate.StatusChangeCount = changes
	history.Aggregate.KnownSeconds = history.Aggregate.ActiveSeconds + history.Aggregate.DegradedSeconds + history.Aggregate.UnavailableSeconds
	windowSeconds := until.Sub(since).Seconds()
	if windowSeconds > 0 {
		history.Aggregate.MonitoringCoverage = round(history.Aggregate.KnownSeconds/windowSeconds*100, 4)
	}
	if history.Aggregate.KnownSeconds > 0 {
		uptime := round(history.Aggregate.ActiveSeconds/history.Aggregate.KnownSeconds*100, 4)
		history.Aggregate.UptimePercentage = &uptime
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM incidents
		WHERE domain_id = $1 AND opened_at >= $2 AND opened_at <= $3
	`, domainID, since, until).Scan(&history.Aggregate.IncidentCount); err != nil {
		return fmt.Errorf("count incidents in history window: %w", err)
	}
	var average *float64
	if err := s.pool.QueryRow(ctx, `
		SELECT avg(http.total_duration_us)::float8 / 1000.0
		FROM http_checks http
		JOIN monitoring_results result ON result.id = http.monitoring_result_id
		WHERE result.domain_id = $1 AND http.checked_at >= $2 AND http.checked_at <= $3
		  AND http.final_status_code IS NOT NULL
	`, domainID, since, until).Scan(&average); err != nil {
		return fmt.Errorf("calculate average response time: %w", err)
	}
	if average != nil {
		value := round(*average, 3)
		history.Aggregate.AverageResponseMS = &value
	}
	return nil
}

func accumulateState(aggregate *HistoryAggregate, state string, seconds float64) {
	if seconds <= 0 {
		return
	}
	switch state {
	case "ACTIVE":
		aggregate.ActiveSeconds += seconds
	case "DEGRADED":
		aggregate.DegradedSeconds += seconds
	case "UNAVAILABLE":
		aggregate.UnavailableSeconds += seconds
	}
}

func round(value float64, digits int) float64 {
	factor := math.Pow10(digits)
	return math.Round(value*factor) / factor
}

func (s *Service) ListIncidents(ctx context.Context, status string, page, pageSize int) (IncidentPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if status != "" && status != "open" && status != "acknowledged" && status != "closed" {
		return IncidentPage{}, ErrInvalidIncidentStatus
	}
	condition := "($1 = '' OR incident.status::text = $1)"
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM incidents incident WHERE `+condition, status).Scan(&total); err != nil {
		return IncidentPage{}, fmt.Errorf("count incidents: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT incident.id, incident.domain_id, domain.domain_ascii, incident.status::text,
		       incident.failure_stage, incident.error_code, incident.open_failure_count,
		       incident.close_success_count, incident.opened_at, incident.closed_at,
		       incident.opened_by_run_id, incident.closed_by_run_id
		FROM incidents incident
		JOIN domains domain ON domain.id = incident.domain_id
		WHERE `+condition+`
		ORDER BY incident.opened_at DESC, incident.id DESC
		LIMIT $2 OFFSET $3
	`, status, pageSize, (page-1)*pageSize)
	if err != nil {
		return IncidentPage{}, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()
	items := make([]Incident, 0, pageSize)
	for rows.Next() {
		var incident Incident
		if err := rows.Scan(&incident.ID, &incident.DomainID, &incident.DomainASCII, &incident.Status,
			&incident.FailureStage, &incident.ErrorCode, &incident.OpenFailureCount,
			&incident.CloseSuccessCount, &incident.OpenedAt, &incident.ClosedAt,
			&incident.OpenedByRunID, &incident.ClosedByRunID); err != nil {
			return IncidentPage{}, fmt.Errorf("scan incident: %w", err)
		}
		items = append(items, incident)
	}
	if err := rows.Err(); err != nil {
		return IncidentPage{}, fmt.Errorf("iterate incidents: %w", err)
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return IncidentPage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
}

func jsonOrEmpty(value json.RawMessage, fallback string) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(fallback)
	}
	return value
}
