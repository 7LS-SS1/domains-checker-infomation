package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const domainColumns = `
	id, original_input, domain_ascii, domain_unicode, registrable_domain, registrar_id,
	lifecycle_status::text, source_status::text, source_type, business_priority::text,
	monitoring_enabled, expected_content_mode, expiration_at, notes,
	current_availability_status::text, current_dns_status::text, current_http_status::text,
	current_redirect_status::text, current_isp_status::text, current_tls_status::text,
	current_content_status::text, current_confidence_score,
	consecutive_failure_count, consecutive_success_count, current_failure_stage, current_error_code,
	last_checked_at, archived_at, created_at, updated_at, version`

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (Domain, error) {
	return scanDomain(s.pool.QueryRow(ctx, `SELECT `+domainColumns+` FROM domains WHERE id = $1`, id))
}

func (s *Store) GetTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Domain, error) {
	return scanDomain(tx.QueryRow(ctx, `SELECT `+domainColumns+` FROM domains WHERE id = $1 FOR UPDATE`, id))
}

func (s *Store) ListProvenance(ctx context.Context, id uuid.UUID) ([]Provenance, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM domains WHERE id=$1)`, id).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, err := s.pool.Query(ctx, `SELECT field_name,value,source,COALESCE(source_reference,''),observed_at,is_current FROM domain_field_provenance WHERE domain_id=$1 ORDER BY field_name,observed_at DESC LIMIT 500`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Provenance{}
	for rows.Next() {
		var item Provenance
		if err := rows.Scan(&item.FieldName, &item.Value, &item.Source, &item.SourceReference, &item.ObservedAt, &item.IsCurrent); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) List(ctx context.Context, filter ListFilter) (Page, error) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 6)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if filter.Query != "" {
		placeholder := addArg("%" + filter.Query + "%")
		conditions = append(conditions, "(domain_ascii ILIKE "+placeholder+" OR domain_unicode ILIKE "+placeholder+")")
	}
	if filter.LifecycleStatus != "" {
		conditions = append(conditions, "lifecycle_status::text = "+addArg(filter.LifecycleStatus))
	}
	if filter.SourceStatus != "" {
		conditions = append(conditions, "source_status::text = "+addArg(filter.SourceStatus))
	}

	sortExpression := map[string]string{
		"domain":     "domain_ascii",
		"created_at": "created_at",
		"updated_at": "updated_at",
		"expiration": "expiration_at",
	}[filter.Sort]
	if sortExpression == "" {
		sortExpression = "domain_ascii"
	}
	direction := "ASC"
	if strings.EqualFold(filter.Direction, "desc") {
		direction = "DESC"
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	var total int64
	countQuery := `SELECT count(*) FROM domains WHERE ` + strings.Join(conditions, " AND ")
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count domains: %w", err)
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	if int64((page-1)*pageSize) >= total && total > 0 {
		return Page{Items: []Domain{}, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
	}
	limitPlaceholder := addArg(pageSize)
	offsetPlaceholder := addArg((page - 1) * pageSize)

	query := `
		SELECT ` + domainColumns + `
		FROM domains
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY ` + sortExpression + ` ` + direction + ` NULLS LAST, id ASC
		LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return Page{}, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	items := make([]Domain, 0, pageSize)
	for rows.Next() {
		item, err := scanDomain(rows)
		if err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("iterate domains: %w", err)
	}
	return Page{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
}

func (s *Store) InsertTx(ctx context.Context, tx pgx.Tx, item Domain, createdBy uuid.UUID) (Domain, error) {
	row := tx.QueryRow(ctx, `
		INSERT INTO domains (
			id, original_input, domain_ascii, domain_unicode, registrable_domain, registrar_id,
			business_priority, monitoring_enabled, expected_content_mode, expiration_at, notes,
			source_type, source_status, created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'manual','present',$12)
		RETURNING `+domainColumns,
		item.ID, item.OriginalInput, item.ASCII, item.Unicode, item.RegistrableDomain, item.RegistrarID,
		item.BusinessPriority, item.MonitoringEnabled, item.ExpectedContentMode, item.ExpirationAt, item.Notes, createdBy,
	)
	created, err := scanDomain(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Domain{}, ErrDuplicate
		}
		return Domain{}, err
	}
	return created, nil
}

func (s *Store) EnsureScheduleTx(ctx context.Context, tx pgx.Tx, domainID uuid.UUID, enabled bool) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO monitor_schedules (domain_id, enabled, jitter_seconds, next_due_at)
		VALUES ($1::uuid, $2, mod((('x' || substr(md5(($1::uuid)::text), 1, 8))::bit(32)::bigint), 60)::integer, now())
		ON CONFLICT (domain_id) DO UPDATE
		SET enabled = EXCLUDED.enabled,
		    next_due_at = CASE
		        WHEN EXCLUDED.enabled AND NOT monitor_schedules.enabled THEN now()
		        ELSE monitor_schedules.next_due_at
		    END,
		    version = monitor_schedules.version + 1
	`, domainID, enabled)
	if err != nil {
		return fmt.Errorf("ensure monitor schedule: %w", err)
	}
	return nil
}

func (s *Store) UpdateTx(ctx context.Context, tx pgx.Tx, item Domain, expectedVersion int64) (Domain, error) {
	row := tx.QueryRow(ctx, `
		UPDATE domains SET
			original_input = $2,
			domain_ascii = $3,
			domain_unicode = $4,
			registrable_domain = $5,
			registrar_id = $6,
			business_priority = $7,
			monitoring_enabled = $8,
			expected_content_mode = $9,
			expiration_at = $10,
			notes = $11,
			version = version + 1
		WHERE id = $1 AND version = $12
		RETURNING `+domainColumns,
		item.ID, item.OriginalInput, item.ASCII, item.Unicode, item.RegistrableDomain, item.RegistrarID,
		item.BusinessPriority, item.MonitoringEnabled, item.ExpectedContentMode, item.ExpirationAt, item.Notes,
		expectedVersion,
	)
	updated, err := scanDomain(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Domain{}, ErrDuplicate
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return Domain{}, ErrConflict
		}
		return Domain{}, err
	}
	return updated, nil
}

func (s *Store) ArchiveTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, version int64) (Domain, error) {
	return scanDomain(tx.QueryRow(ctx, `
		UPDATE domains SET
			lifecycle_status = 'archived',
			monitoring_enabled_before_archive = monitoring_enabled,
			monitoring_enabled = false,
			archived_at = now(),
			version = version + 1
		WHERE id = $1 AND version = $2 AND lifecycle_status <> 'archived'
		RETURNING `+domainColumns, id, version))
}

func (s *Store) RestoreTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, version int64) (Domain, error) {
	return scanDomain(tx.QueryRow(ctx, `
		UPDATE domains SET
			lifecycle_status = 'active',
			monitoring_enabled = monitoring_enabled_before_archive,
			archived_at = NULL,
			version = version + 1
		WHERE id = $1 AND version = $2 AND lifecycle_status = 'archived'
		RETURNING `+domainColumns, id, version))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDomain(row rowScanner) (Domain, error) {
	var item Domain
	err := row.Scan(
		&item.ID, &item.OriginalInput, &item.ASCII, &item.Unicode, &item.RegistrableDomain, &item.RegistrarID,
		&item.LifecycleStatus, &item.SourceStatus, &item.SourceType, &item.BusinessPriority,
		&item.MonitoringEnabled, &item.ExpectedContentMode, &item.ExpirationAt, &item.Notes,
		&item.AvailabilityStatus, &item.DNSStatus, &item.HTTPStatus, &item.RedirectStatus,
		&item.ISPStatus, &item.TLSStatus, &item.ContentStatus, &item.ConfidenceScore,
		&item.ConsecutiveFailures, &item.ConsecutiveSuccesses, &item.CurrentFailureStage, &item.CurrentErrorCode,
		&item.LastCheckedAt, &item.ArchivedAt,
		&item.CreatedAt, &item.UpdatedAt, &item.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Domain{}, ErrNotFound
		}
		return Domain{}, fmt.Errorf("scan domain: %w", err)
	}
	return item, nil
}
