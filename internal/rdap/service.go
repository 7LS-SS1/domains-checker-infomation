package rdap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Actor struct {
	UserID    uuid.UUID
	RequestID string
}

type Conflict struct {
	Field          string `json:"field"`
	CurrentSource  string `json:"current_source"`
	CurrentValue   any    `json:"current_value"`
	ObservedSource string `json:"observed_source"`
	ObservedValue  any    `json:"observed_value"`
}

type Result struct {
	ID              uuid.UUID         `json:"id"`
	DomainID        uuid.UUID         `json:"domain_id"`
	BootstrapURL    string            `json:"bootstrap_url"`
	RDAPURL         string            `json:"rdap_url"`
	HTTPStatus      int               `json:"http_status,omitempty"`
	SourceStatus    string            `json:"source_status"`
	Confidence      int16             `json:"confidence_score"`
	BootstrapStale  bool              `json:"bootstrap_stale"`
	Normalized      Normalized        `json:"registration"`
	ResponseHeaders map[string]string `json:"response_headers"`
	Conflicts       []Conflict        `json:"conflicts"`
	ErrorCode       string            `json:"error_code,omitempty"`
	ErrorMessage    string            `json:"error_message,omitempty"`
	CheckedAt       time.Time         `json:"checked_at"`
	CacheUntil      *time.Time        `json:"cache_until,omitempty"`
	Cached          bool              `json:"cached"`
}

type ServiceConfig struct {
	BootstrapTTL      time.Duration
	BootstrapMaxStale time.Duration
	DomainTTL         time.Duration
}

type Service struct {
	pool   *pgxpool.Pool
	audit  *audit.Store
	client *Client
	config ServiceConfig
	now    func() time.Time
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store, client *Client, config ServiceConfig) *Service {
	if config.BootstrapTTL <= 0 {
		config.BootstrapTTL = 24 * time.Hour
	}
	if config.BootstrapMaxStale <= 0 {
		config.BootstrapMaxStale = 7 * 24 * time.Hour
	}
	if config.DomainTTL <= 0 {
		config.DomainTTL = 6 * time.Hour
	}
	return &Service{pool: pool, audit: auditStore, client: client, config: config, now: time.Now}
}

func (s *Service) Latest(ctx context.Context, domainID uuid.UUID) (Result, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id,domain_id,COALESCE(bootstrap_url,''),COALESCE(rdap_url,''),COALESCE(http_status,0),source_status,
		       confidence_score,bootstrap_stale,registrar_name,registrar_iana_id,registration_at,expiration_at,updated_at_source,
		       nameservers,dnssec,domain_statuses,response_headers,conflicts,COALESCE(error_code,''),COALESCE(error_message,''),checked_at,cache_until
		FROM rdap_checks WHERE domain_id=$1 ORDER BY checked_at DESC,created_at DESC LIMIT 1
	`, domainID)
	result, err := scanResult(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrUnavailable
	}
	return result, err
}

func (s *Service) Check(ctx context.Context, actor Actor, domainID uuid.UUID, force bool) (Result, error) {
	now := s.now().UTC()
	if !force {
		if cached, err := s.Latest(ctx, domainID); err == nil && cached.CacheUntil != nil && cached.CacheUntil.After(now) && cached.SourceStatus != "unavailable" {
			cached.Cached = true
			return cached, nil
		}
	}
	var domainName string
	if err := s.pool.QueryRow(ctx, `SELECT domain_ascii FROM domains WHERE id=$1`, domainID).Scan(&domainName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrUnavailable
		}
		return Result{}, err
	}
	parts := strings.Split(domainName, ".")
	if len(parts) < 2 {
		return Result{}, ErrNoBootstrap
	}
	bootstrapPayload, stale, err := s.bootstrap(ctx)
	if err != nil {
		result := Result{ID: uuid.New(), DomainID: domainID, BootstrapURL: s.client.config.BootstrapURL, SourceStatus: "unavailable", Confidence: 5, BootstrapStale: stale, Conflicts: []Conflict{}, ResponseHeaders: map[string]string{}, ErrorCode: errorCode(err), ErrorMessage: safeError(err), CheckedAt: now}
		return result, s.persist(ctx, actor, result, nil, err)
	}
	baseURL, err := ResolveBase(bootstrapPayload, parts[len(parts)-1])
	if err != nil {
		result := Result{ID: uuid.New(), DomainID: domainID, BootstrapURL: s.client.config.BootstrapURL, SourceStatus: "unavailable", Confidence: 5, BootstrapStale: stale, Conflicts: []Conflict{}, ResponseHeaders: map[string]string{}, ErrorCode: errorCode(err), ErrorMessage: safeError(err), CheckedAt: now}
		return result, s.persist(ctx, actor, result, nil, err)
	}
	httpResult, fetchErr := s.client.Domain(ctx, baseURL, domainName)
	result := Result{ID: uuid.New(), DomainID: domainID, BootstrapURL: s.client.config.BootstrapURL, RDAPURL: httpResult.URL, HTTPStatus: httpResult.Status, BootstrapStale: stale, ResponseHeaders: httpResult.Headers, Conflicts: []Conflict{}, CheckedAt: now}
	if result.RDAPURL == "" {
		result.RDAPURL = strings.TrimRight(baseURL, "/") + "/domain/" + domainName
	}
	if fetchErr != nil {
		result.SourceStatus, result.Confidence, result.ErrorCode, result.ErrorMessage = "unavailable", 10, errorCode(fetchErr), safeError(fetchErr)
		return result, s.persist(ctx, actor, result, httpResult.Body, fetchErr)
	}
	normalized, normalizeErr := Normalize(httpResult.Body)
	if normalizeErr != nil {
		result.SourceStatus, result.Confidence, result.ErrorCode, result.ErrorMessage = "unavailable", 10, "RDAP_INVALID_RESPONSE", "RDAP returned invalid JSON"
		return result, s.persist(ctx, actor, result, httpResult.Body, normalizeErr)
	}
	result.Normalized = normalized
	result.SourceStatus, result.Confidence = completeness(normalized)
	cacheUntil := now.Add(s.config.DomainTTL)
	result.CacheUntil = &cacheUntil
	conflicts, conflictErr := s.detectConflicts(ctx, domainID, normalized, result.RDAPURL)
	if conflictErr != nil {
		return Result{}, conflictErr
	}
	result.Conflicts = conflicts
	return result, s.persist(ctx, actor, result, httpResult.Body, nil)
}

func (s *Service) bootstrap(ctx context.Context) ([]byte, bool, error) {
	now := s.now().UTC()
	cacheKey := bootstrapCacheKey(s.client.config.BootstrapURL)
	var payload []byte
	var etag, lastModified string
	var fetchedAt, expiresAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT payload,COALESCE(etag,''),COALESCE(last_modified,''),fetched_at,expires_at FROM rdap_bootstrap_cache WHERE registry_key=$1`, cacheKey).Scan(&payload, &etag, &lastModified, &fetchedAt, &expiresAt)
	if err == nil && expiresAt.After(now) {
		return payload, false, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	response, fetchErr := s.client.Bootstrap(ctx, etag, lastModified)
	if fetchErr != nil {
		if len(payload) > 0 && now.Sub(fetchedAt) <= s.config.BootstrapMaxStale {
			_, _ = s.pool.Exec(ctx, `UPDATE rdap_bootstrap_cache SET last_error_code=$2,last_error_at=now() WHERE registry_key=$1`, cacheKey, errorCode(fetchErr))
			return payload, true, nil
		}
		return nil, false, fetchErr
	}
	if response.Status == httpStatusNotModified && len(payload) > 0 {
		_, err = s.pool.Exec(ctx, `UPDATE rdap_bootstrap_cache SET fetched_at=$2,expires_at=$3,last_error_code=NULL,last_error_at=NULL WHERE registry_key=$1`, cacheKey, now, now.Add(s.config.BootstrapTTL))
		return payload, false, err
	}
	if _, resolveErr := ResolveBase(response.Body, "com"); resolveErr != nil {
		return nil, false, resolveErr
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO rdap_bootstrap_cache (registry_key,source_url,etag,last_modified,payload,payload_hash,fetched_at,expires_at)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8)
		ON CONFLICT (registry_key) DO UPDATE SET source_url=EXCLUDED.source_url,etag=EXCLUDED.etag,last_modified=EXCLUDED.last_modified,payload=EXCLUDED.payload,payload_hash=EXCLUDED.payload_hash,fetched_at=EXCLUDED.fetched_at,expires_at=EXCLUDED.expires_at,last_error_code=NULL,last_error_at=NULL
	`, cacheKey, response.URL, response.ETag, response.LastModified, response.Body, payloadHash(response.Body), now, now.Add(s.config.BootstrapTTL))
	if err != nil {
		return nil, false, err
	}
	return response.Body, false, nil
}

func bootstrapCacheKey(sourceURL string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(sourceURL)))
	return "dns:" + hex.EncodeToString(digest[:8])
}

const httpStatusNotModified = 304

func (s *Service) detectConflicts(ctx context.Context, domainID uuid.UUID, normalized Normalized, reference string) ([]Conflict, error) {
	var registrarName *string
	var expiration *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT r.name,d.expiration_at FROM domains d LEFT JOIN registrars r ON r.id=d.registrar_id WHERE d.id=$1`, domainID).Scan(&registrarName, &expiration); err != nil {
		return nil, err
	}
	conflicts := []Conflict{}
	expirationSource := "database"
	var overrideExpiration string
	if err := s.pool.QueryRow(ctx, `SELECT override_value #>> '{}' FROM manual_overrides WHERE domain_id=$1 AND field_name='expiration_date' AND revoked_at IS NULL AND effective_from<=now() AND (expires_at IS NULL OR expires_at>now()) ORDER BY effective_from DESC LIMIT 1`, domainID).Scan(&overrideExpiration); err == nil {
		if parsed, parseErr := time.Parse("2006-01-02", overrideExpiration); parseErr == nil {
			expiration = &parsed
			expirationSource = "manual_override"
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if registrarName != nil && normalized.RegistrarName != nil && !strings.EqualFold(strings.TrimSpace(*registrarName), strings.TrimSpace(*normalized.RegistrarName)) {
		conflicts = append(conflicts, Conflict{Field: "registrar", CurrentSource: "database", CurrentValue: *registrarName, ObservedSource: "rdap", ObservedValue: *normalized.RegistrarName})
	}
	if expiration != nil && normalized.ExpirationAt != nil && expiration.UTC().Format("2006-01-02") != normalized.ExpirationAt.UTC().Format("2006-01-02") {
		conflicts = append(conflicts, Conflict{Field: "expiration_date", CurrentSource: expirationSource, CurrentValue: expiration.UTC().Format("2006-01-02"), ObservedSource: "rdap", ObservedValue: normalized.ExpirationAt.UTC().Format("2006-01-02")})
	}
	return conflicts, nil
}

func (s *Service) persist(ctx context.Context, actor Actor, result Result, raw []byte, sourceErr error) error {
	nameservers, _ := json.Marshal(result.Normalized.Nameservers)
	statuses, _ := json.Marshal(result.Normalized.Statuses)
	headers, _ := json.Marshal(result.ResponseHeaders)
	conflictsJSON, _ := json.Marshal(result.Conflicts)
	var hash []byte
	var excerpt []byte
	if len(raw) > 0 {
		hash = payloadHash(raw)
		excerpt = raw
		if len(excerpt) > 262144 {
			excerpt = excerpt[:262144]
		}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO rdap_checks (id,domain_id,bootstrap_url,rdap_url,http_status,registrar_name,registrar_iana_id,registration_at,expiration_at,updated_at_source,nameservers,dnssec,domain_statuses,source_status,raw_payload_hash,raw_excerpt,error_code,error_message,checked_at,response_headers,bootstrap_stale,cache_until,confidence_score,conflicts)
		VALUES ($1,$2,$3,$4,NULLIF($5,0),$6,$7,$8,$9,$10,$11::jsonb,$12,$13::jsonb,$14,$15,$16,NULLIF($17,''),NULLIF($18,''),$19,$20::jsonb,$21,$22,$23,$24::jsonb)
	`, result.ID, result.DomainID, result.BootstrapURL, result.RDAPURL, result.HTTPStatus, result.Normalized.RegistrarName, result.Normalized.RegistrarIANA, result.Normalized.RegistrationAt, result.Normalized.ExpirationAt, result.Normalized.UpdatedAt, nameservers, result.Normalized.DNSSEC, statuses, result.SourceStatus, hash, excerpt, result.ErrorCode, result.ErrorMessage, result.CheckedAt, headers, result.BootstrapStale, result.CacheUntil, result.Confidence, conflictsJSON)
	if err != nil {
		return err
	}
	for _, conflict := range result.Conflicts {
		currentJSON, _ := json.Marshal(conflict.CurrentValue)
		observedJSON, _ := json.Marshal(conflict.ObservedValue)
		_, err = tx.Exec(ctx, `INSERT INTO provenance_conflicts (domain_id,field_name,current_source,current_value,observed_source,observed_value,source_reference) VALUES ($1,$2,$3,$4::jsonb,$5,$6::jsonb,$7)`, result.DomainID, conflict.Field, conflict.CurrentSource, currentJSON, conflict.ObservedSource, observedJSON, result.RDAPURL)
		if err != nil {
			return err
		}
	}
	for field, value := range provenanceValues(result.Normalized) {
		encoded, _ := json.Marshal(value)
		_, err = tx.Exec(ctx, `INSERT INTO domain_field_provenance (domain_id,field_name,value,source,source_reference,observed_at,is_current) VALUES ($1,$2,$3::jsonb,'rdap',$4,$5,false)`, result.DomainID, field, encoded, result.RDAPURL, result.CheckedAt)
		if err != nil {
			return err
		}
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "RDAP_CHECK_COMPLETED", ResourceType: "rdap_check", ResourceID: &result.ID, RequestID: actor.RequestID, After: result, Metadata: map[string]any{"domain_id": result.DomainID, "source_error": sourceErr != nil}}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return sourceErr
}

func scanResult(row pgx.Row) (Result, error) {
	var result Result
	var nameservers, statuses, headers, conflicts []byte
	err := row.Scan(&result.ID, &result.DomainID, &result.BootstrapURL, &result.RDAPURL, &result.HTTPStatus, &result.SourceStatus, &result.Confidence, &result.BootstrapStale, &result.Normalized.RegistrarName, &result.Normalized.RegistrarIANA, &result.Normalized.RegistrationAt, &result.Normalized.ExpirationAt, &result.Normalized.UpdatedAt, &nameservers, &result.Normalized.DNSSEC, &statuses, &headers, &conflicts, &result.ErrorCode, &result.ErrorMessage, &result.CheckedAt, &result.CacheUntil)
	if err != nil {
		return Result{}, err
	}
	_ = json.Unmarshal(nameservers, &result.Normalized.Nameservers)
	_ = json.Unmarshal(statuses, &result.Normalized.Statuses)
	_ = json.Unmarshal(headers, &result.ResponseHeaders)
	_ = json.Unmarshal(conflicts, &result.Conflicts)
	return result, nil
}

func completeness(value Normalized) (string, int16) {
	known := 0
	for _, present := range []bool{value.RegistrarName != nil, value.RegistrarIANA != nil, value.RegistrationAt != nil, value.ExpirationAt != nil, value.UpdatedAt != nil, len(value.Nameservers) > 0, value.DNSSEC != nil, len(value.Statuses) > 0} {
		if present {
			known++
		}
	}
	if known == 0 {
		return "unavailable", 10
	}
	if known < 5 {
		return "partial", int16(35 + known*7)
	}
	return "available", int16(min(60+known*4, 92))
}

func provenanceValues(value Normalized) map[string]any {
	result := map[string]any{}
	if value.RegistrarName != nil {
		result["registrar"] = *value.RegistrarName
	}
	if value.RegistrarIANA != nil {
		result["registrar_iana_id"] = *value.RegistrarIANA
	}
	if value.RegistrationAt != nil {
		result["registration_date"] = value.RegistrationAt.UTC().Format(time.RFC3339)
	}
	if value.ExpirationAt != nil {
		result["expiration_date"] = value.ExpirationAt.UTC().Format(time.RFC3339)
	}
	if value.UpdatedAt != nil {
		result["rdap_updated_at"] = value.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if len(value.Nameservers) > 0 {
		result["nameservers"] = value.Nameservers
	}
	if value.DNSSEC != nil {
		result["dnssec"] = *value.DNSSEC
	}
	if len(value.Statuses) > 0 {
		result["domain_statuses"] = value.Statuses
	}
	return result
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, ErrRateLimited):
		return "RDAP_RATE_LIMITED"
	case errors.Is(err, ErrNoBootstrap):
		return "RDAP_BOOTSTRAP_NOT_FOUND"
	default:
		return "RDAP_UNAVAILABLE"
	}
}

func safeError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 300 {
		message = message[:300]
	}
	return message
}
