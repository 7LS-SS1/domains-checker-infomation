package sheets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Apply(ctx context.Context, actor Actor, importID uuid.UUID, idempotencyKey, reason string) (Import, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	reason = strings.TrimSpace(reason)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 200 || reason == "" {
		return Import{}, fmt.Errorf("%w: idempotency key and reason are required", ErrValidation)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Import{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	item, err := scanImport(tx.QueryRow(ctx, importSelect+` WHERE id=$1 FOR UPDATE`, importID))
	if err != nil {
		return Import{}, err
	}
	if item.Status == "applied" {
		var existingKey *string
		if err := tx.QueryRow(ctx, `SELECT apply_idempotency_key FROM google_sheet_imports WHERE id=$1`, importID).Scan(&existingKey); err != nil {
			return Import{}, err
		}
		if existingKey != nil && *existingKey == idempotencyKey {
			_ = tx.Rollback(ctx)
			return s.GetImport(ctx, importID)
		}
		return Import{}, ErrConflict
	}
	if item.Status != "preview" {
		return Import{}, ErrConflict
	}
	rows, err := tx.Query(ctx, `SELECT id,row_number,matched_domain_id,action,valid,raw_values,normalized_values,validation_errors,diff FROM google_sheet_import_rows WHERE import_id=$1 ORDER BY row_number FOR UPDATE`, importID)
	if err != nil {
		return Import{}, err
	}
	importRows := []ImportRow{}
	for rows.Next() {
		row, scanErr := scanImportRow(rows)
		if scanErr != nil {
			rows.Close()
			return Import{}, scanErr
		}
		importRows = append(importRows, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return Import{}, err
	}
	rows.Close()
	applied := 0
	for _, row := range importRows {
		if !row.Valid || row.Action == "INVALID" {
			continue
		}
		switch row.Action {
		case "ADD":
			if row.NormalizedValues == nil {
				return Import{}, fmt.Errorf("%w: missing normalized row", ErrValidation)
			}
			if _, err := s.applyAdd(ctx, tx, actor, item, row); err != nil {
				return Import{}, err
			}
		case "MODIFY":
			if row.NormalizedValues == nil || row.MatchedDomainID == nil {
				return Import{}, fmt.Errorf("%w: invalid modify row", ErrValidation)
			}
			if err := s.applyModify(ctx, tx, actor, item, row); err != nil {
				return Import{}, err
			}
		case "MISSING":
			if row.MatchedDomainID == nil {
				return Import{}, fmt.Errorf("%w: invalid missing row", ErrValidation)
			}
			if err := s.applyMissing(ctx, tx, *row.MatchedDomainID, item.SourceKind); err != nil {
				return Import{}, err
			}
		case "UNCHANGED":
			if row.MatchedDomainID != nil {
				if _, err := tx.Exec(ctx, `UPDATE domains SET source_status='present',version=version+1 WHERE id=$1 AND source_status<>'present'`, *row.MatchedDomainID); err != nil {
					return Import{}, err
				}
			}
		default:
			return Import{}, fmt.Errorf("%w: unsupported action", ErrValidation)
		}
		applied++
	}
	now := s.now().UTC()
	command, err := tx.Exec(ctx, `UPDATE google_sheet_imports SET status='applied',apply_idempotency_key=$2,applied_by=$3,applied_at=$4,valid_rows_applied=$5 WHERE id=$1 AND status='preview'`, importID, idempotencyKey, actor.UserID, now, applied)
	if err != nil {
		return Import{}, err
	}
	if command.RowsAffected() != 1 {
		return Import{}, ErrConflict
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "SHEET_IMPORT_APPLIED", ResourceType: "sheet_import", ResourceID: &importID, RequestID: actor.RequestID, Reason: reason, Before: item, After: map[string]any{"status": "applied", "valid_rows_applied": applied, "source_hash": item.SourceHash}, Metadata: map[string]any{"invalid_rows_excluded": item.InvalidCount, "source_kind": item.SourceKind}}); err != nil {
		return Import{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Import{}, err
	}
	return s.GetImport(ctx, importID)
}

func (s *Service) applyAdd(ctx context.Context, tx pgx.Tx, actor Actor, importItem Import, row ImportRow) (uuid.UUID, error) {
	value := *row.NormalizedValues
	var existing uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT id FROM domains WHERE domain_ascii=$1 FOR UPDATE`, value.Domain).Scan(&existing); err == nil {
		row.MatchedDomainID = &existing
		return existing, s.applyModify(ctx, tx, actor, importItem, row)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	registrarID, err := ensureRegistrar(ctx, tx, value.Registrar)
	if err != nil {
		return uuid.Nil, err
	}
	domainID := uuid.New()
	lifecycle := "active"
	if !value.Active {
		lifecycle = "inactive"
	}
	purchaseDate, err := nullableDate(value.PurchaseDate)
	if err != nil {
		return uuid.Nil, err
	}
	expiration, err := nullableTimestamp(value.ExpirationDate)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domains (id,original_input,domain_ascii,domain_unicode,registrable_domain,registrar_id,lifecycle_status,source_status,source_type,business_priority,monitoring_enabled,purchase_date,expiration_at,notes,created_by)
		VALUES ($1,$2,$2,$3,$4,$5,$6,'present',$7,$8,$9,$10,$11,$12,$13)
	`, domainID, value.Domain, value.DomainUnicode, value.RegistrableDomain, registrarID, lifecycle, inventorySourceType(importItem.SourceKind), value.BusinessPriority, value.Active, purchaseDate, expiration, value.Notes, actor.UserID)
	if err != nil {
		return uuid.Nil, err
	}
	if err := ensureSchedule(ctx, tx, domainID, value.Active); err != nil {
		return uuid.Nil, err
	}
	if err := s.applySheetFields(ctx, tx, domainID, importItem, row, value); err != nil {
		return uuid.Nil, err
	}
	return domainID, nil
}

func (s *Service) applyModify(ctx context.Context, tx pgx.Tx, actor Actor, importItem Import, row ImportRow) error {
	value := *row.NormalizedValues
	domainID := *row.MatchedDomainID
	var lifecycle string
	if err := tx.QueryRow(ctx, `SELECT lifecycle_status::text FROM domains WHERE id=$1 FOR UPDATE`, domainID).Scan(&lifecycle); err != nil {
		return err
	}
	overrides, err := activeOverrideFields(ctx, tx, domainID)
	if err != nil {
		return err
	}
	registrarID, err := ensureRegistrar(ctx, tx, value.Registrar)
	if err != nil {
		return err
	}
	purchaseDate, err := nullableDate(value.PurchaseDate)
	if err != nil {
		return err
	}
	expiration, err := nullableTimestamp(value.ExpirationDate)
	if err != nil {
		return err
	}
	nextLifecycle := lifecycle
	monitoring := value.Active
	if lifecycle != "archived" {
		if value.Active {
			nextLifecycle = "active"
		} else {
			nextLifecycle = "inactive"
		}
	} else {
		monitoring = false
	}
	_, err = tx.Exec(ctx, `
		UPDATE domains SET registrar_id=$2,purchase_date=$3,
		 expiration_at=CASE WHEN $4 THEN expiration_at ELSE $5 END,
		 business_priority=CASE WHEN $6 THEN business_priority ELSE $7::business_priority END,
		 notes=$8,lifecycle_status=$9::domain_lifecycle_status,monitoring_enabled=$10,
		 source_type=$11,source_status='present',version=version+1 WHERE id=$1
	`, domainID, registrarID, purchaseDate, overrides["expiration_date"], expiration, overrides["business_priority"], value.BusinessPriority, value.Notes, nextLifecycle, monitoring, inventorySourceType(importItem.SourceKind))
	if err != nil {
		return err
	}
	if err := ensureSchedule(ctx, tx, domainID, monitoring && nextLifecycle == "active"); err != nil {
		return err
	}
	return s.applySheetFields(ctx, tx, domainID, importItem, row, value)
}

func (s *Service) applySheetFields(ctx context.Context, tx pgx.Tx, domainID uuid.UUID, importItem Import, row ImportRow, value NormalizedRow) error {
	reference := "sheet-import:" + importItem.ID.String() + ":row:" + fmt.Sprint(row.RowNumber)
	fields := map[string]any{"domain": value.Domain, "registrar": value.Registrar, "business_priority": value.BusinessPriority, "notes": value.Notes, "active": value.Active}
	if value.PurchaseDate != nil {
		fields["purchase_date"] = *value.PurchaseDate
	}
	if value.ExpirationDate != nil {
		fields["expiration_date"] = *value.ExpirationDate
	}
	for field, fieldValue := range fields {
		if err := setProvenance(ctx, tx, domainID, field, fieldValue, "google_sheet", reference, s.now().UTC()); err != nil {
			return err
		}
	}
	if value.PurchasePrice != nil {
		if err := upsertSheetCost(ctx, tx, domainID, "purchase", *value.PurchasePrice, value.Currency, value.TaxRate, reference); err != nil {
			return err
		}
	}
	if value.RenewalPrice != nil {
		if err := upsertSheetCost(ctx, tx, domainID, "renewal", *value.RenewalPrice, value.Currency, value.TaxRate, reference); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) applyMissing(ctx context.Context, tx pgx.Tx, domainID uuid.UUID, sourceKind string) error {
	_, err := tx.Exec(ctx, `UPDATE domains SET source_status='missing_from_source',monitoring_enabled=false,version=version+1 WHERE id=$1 AND source_type=$2`, domainID, inventorySourceType(sourceKind))
	if err != nil {
		return err
	}
	return ensureSchedule(ctx, tx, domainID, false)
}

func upsertSheetCost(ctx context.Context, tx pgx.Tx, domainID uuid.UUID, costType, amount, currency string, taxRate *string, reference string) error {
	taxMode := "unknown"
	if taxRate != nil {
		taxMode = "exclusive"
	}
	_, err := tx.Exec(ctx, `UPDATE domain_costs SET effective_to=CASE WHEN effective_from<current_date THEN current_date-1 ELSE current_date END WHERE domain_id=$1 AND cost_type=$2 AND price_source='google_sheet' AND effective_to IS NULL`, domainID, costType)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO domain_costs (domain_id,cost_type,amount,currency_code,price_source,tax_rate,tax_inclusive,tax_mode,billing_cycle_months,effective_from,source_reference) VALUES ($1,$2,$3::numeric,$4,'google_sheet',$5::numeric,false,$6,12,current_date,$7)`, domainID, costType, amount, currency, taxRate, taxMode, reference)
	return err
}

func inventorySourceType(sourceKind string) string {
	if sourceKind == "excel" {
		return "import"
	}
	return "google_sheet"
}

func setProvenance(ctx context.Context, tx pgx.Tx, domainID uuid.UUID, field string, value any, source, reference string, observed time.Time) error {
	var previous *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM domain_field_provenance WHERE domain_id=$1 AND field_name=$2 AND is_current=true FOR UPDATE`, domainID, field).Scan(&previous)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if previous != nil {
		if _, err := tx.Exec(ctx, `UPDATE domain_field_provenance SET is_current=false WHERE id=$1`, *previous); err != nil {
			return err
		}
	}
	encoded, _ := json.Marshal(value)
	_, err = tx.Exec(ctx, `INSERT INTO domain_field_provenance (domain_id,field_name,value,source,source_reference,observed_at,is_current,supersedes_id) VALUES ($1,$2,$3::jsonb,$4,$5,$6,true,$7)`, domainID, field, encoded, source, reference, observed, previous)
	return err
}

func ensureRegistrar(ctx context.Context, tx pgx.Tx, name string) (*uuid.UUID, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	var id uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM registrars WHERE lower(name)=lower($1) ORDER BY id LIMIT 1`, name).Scan(&id)
	if err == nil {
		return &id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	id = uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO registrars (id,name) VALUES ($1,$2) ON CONFLICT (name) DO NOTHING`, id, name)
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT id FROM registrars WHERE lower(name)=lower($1) ORDER BY id LIMIT 1`, name).Scan(&id); err != nil {
		return nil, err
	}
	return &id, nil
}

func ensureSchedule(ctx context.Context, tx pgx.Tx, domainID uuid.UUID, enabled bool) error {
	_, err := tx.Exec(ctx, `INSERT INTO monitor_schedules (domain_id,enabled,next_due_at) VALUES ($1,$2,now()) ON CONFLICT (domain_id) DO UPDATE SET enabled=EXCLUDED.enabled,next_due_at=CASE WHEN EXCLUDED.enabled AND NOT monitor_schedules.enabled THEN now() ELSE monitor_schedules.next_due_at END,version=monitor_schedules.version+1`, domainID, enabled)
	return err
}

func activeOverrideFields(ctx context.Context, tx pgx.Tx, domainID uuid.UUID) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `SELECT field_name FROM manual_overrides WHERE domain_id=$1 AND revoked_at IS NULL AND effective_from<=now() AND (expires_at IS NULL OR expires_at>now())`, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var field string
		if err := rows.Scan(&field); err != nil {
			return nil, err
		}
		result[field] = true
	}
	return result, rows.Err()
}

func nullableDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
func nullableTimestamp(value *string) (*time.Time, error) { return nullableDate(value) }

func (s *Service) Reject(ctx context.Context, actor Actor, importID uuid.UUID, reason string) (Import, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Import{}, fmt.Errorf("%w: reason", ErrValidation)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Import{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	item, err := scanImport(tx.QueryRow(ctx, importSelect+` WHERE id=$1 FOR UPDATE`, importID))
	if err != nil {
		return Import{}, err
	}
	if item.Status != "preview" {
		return Import{}, ErrConflict
	}
	now := s.now().UTC()
	command, err := tx.Exec(ctx, `UPDATE google_sheet_imports SET status='rejected',rejected_by=$2,rejected_at=$3 WHERE id=$1 AND status='preview'`, importID, actor.UserID, now)
	if err != nil {
		return Import{}, err
	}
	if command.RowsAffected() != 1 {
		return Import{}, ErrConflict
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "SHEET_IMPORT_REJECTED", ResourceType: "sheet_import", ResourceID: &importID, RequestID: actor.RequestID, Reason: reason, Before: item, After: map[string]any{"status": "rejected"}, Metadata: map[string]any{"source_kind": item.SourceKind}}); err != nil {
		return Import{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Import{}, err
	}
	return s.GetImport(ctx, importID)
}

func (s *Service) ScheduleDue(ctx context.Context) (int, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	config, err := scanConfig(tx.QueryRow(ctx, configSelect+` WHERE enabled=true AND next_sync_at<=now() LIMIT 1 FOR UPDATE SKIP LOCKED`))
	if errors.Is(err, ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	due := config.NextSyncAt
	if due == nil {
		value := s.now().UTC()
		due = &value
	}
	_, err = tx.Exec(ctx, `UPDATE google_sheet_configs SET next_sync_at=now()+(sync_interval_minutes*interval '1 minute'),version=version+1 WHERE id=$1`, config.ID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	key := "scheduled:" + config.ID.String() + ":" + due.UTC().Format(time.RFC3339)
	actor := Actor{UserID: config.UpdatedBy, RequestID: key}
	if _, err := s.Preview(ctx, actor, key, "scheduled"); err != nil {
		return 0, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE google_sheet_configs SET last_sync_at=now() WHERE id=$1`, config.ID)
	return 1, nil
}
