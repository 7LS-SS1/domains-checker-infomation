package sheets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/audit"
	"domainmonitor/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool       *pgxpool.Pool
	audit      *audit.Store
	source     Source
	normalizer domain.Normalizer
	now        func() time.Time
}

func NewService(pool *pgxpool.Pool, auditStore *audit.Store, source Source, normalizer domain.Normalizer) *Service {
	return &Service{pool: pool, audit: auditStore, source: source, normalizer: normalizer, now: time.Now}
}

func (s *Service) GetConfig(ctx context.Context) (Config, error) {
	return scanConfig(s.pool.QueryRow(ctx, configSelect+` LIMIT 1`))
}

func (s *Service) SaveConfig(ctx context.Context, actor Actor, input ConfigInput) (Config, error) {
	input.SpreadsheetID = strings.TrimSpace(input.SpreadsheetID)
	input.SheetName = strings.TrimSpace(input.SheetName)
	input.Range = strings.TrimSpace(input.Range)
	if input.Range == "" {
		input.Range = "A:Z"
	}
	if input.SyncIntervalMinutes == 0 {
		input.SyncIntervalMinutes = 60
	}
	if input.SpreadsheetID == "" || input.SheetName == "" || input.SyncIntervalMinutes < 5 || input.SyncIntervalMinutes > 10080 {
		return Config{}, fmt.Errorf("%w: invalid Google Sheet configuration", ErrValidation)
	}
	for semantic := range input.ColumnMapping {
		if _, ok := headerAliases[semantic]; !ok {
			return Config{}, fmt.Errorf("%w: unsupported column semantic %s", ErrValidation, semantic)
		}
	}
	if input.ConnectionID != nil {
		var allowed bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM google_drive_connections WHERE id=$1 AND user_id=$2 AND status='active')`, *input.ConnectionID, actor.UserID).Scan(&allowed); err != nil {
			return Config{}, err
		}
		if !allowed {
			return Config{}, fmt.Errorf("%w: Google Drive connection is unavailable", ErrValidation)
		}
	}
	mapping, _ := json.Marshal(input.ColumnMapping)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Config{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	before, getErr := scanConfig(tx.QueryRow(ctx, configSelect+` LIMIT 1 FOR UPDATE`))
	var after Config
	if errors.Is(getErr, ErrNotFound) {
		if input.Version > 0 {
			return Config{}, ErrConflict
		}
		after, err = scanConfig(tx.QueryRow(ctx, `
			INSERT INTO google_sheet_configs (connection_id,spreadsheet_id,sheet_name,sheet_range,column_mapping,enabled,sync_interval_minutes,next_sync_at,updated_by)
			VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,CASE WHEN $6 THEN now() ELSE NULL END,$8)
			RETURNING id,connection_id,spreadsheet_id,sheet_name,sheet_range,column_mapping,enabled,sync_interval_minutes,next_sync_at,last_sync_at,updated_by,created_at,updated_at,version
		`, input.ConnectionID, input.SpreadsheetID, input.SheetName, input.Range, mapping, input.Enabled, input.SyncIntervalMinutes, actor.UserID))
	} else if getErr != nil {
		return Config{}, getErr
	} else {
		if input.Version != before.Version {
			return Config{}, ErrConflict
		}
		after, err = scanConfig(tx.QueryRow(ctx, `
			UPDATE google_sheet_configs SET connection_id=$2,spreadsheet_id=$3,sheet_name=$4,sheet_range=$5,column_mapping=$6::jsonb,enabled=$7,sync_interval_minutes=$8,
			 next_sync_at=CASE WHEN $7 AND (enabled=false OR next_sync_at IS NULL) THEN now() WHEN $7 THEN next_sync_at ELSE NULL END,updated_by=$9,version=version+1
			WHERE id=$1 AND version=$10
			RETURNING id,connection_id,spreadsheet_id,sheet_name,sheet_range,column_mapping,enabled,sync_interval_minutes,next_sync_at,last_sync_at,updated_by,created_at,updated_at,version
		`, before.ID, input.ConnectionID, input.SpreadsheetID, input.SheetName, input.Range, mapping, input.Enabled, input.SyncIntervalMinutes, actor.UserID, input.Version))
	}
	if err != nil {
		return Config{}, err
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "GOOGLE_SHEET_CONFIG_SAVED", ResourceType: "google_sheet_config", ResourceID: &after.ID, RequestID: actor.RequestID, Reason: strings.TrimSpace(input.Reason), Before: nullableConfig(before, getErr), After: after}); err != nil {
		return Config{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Config{}, err
	}
	return after, nil
}

func (s *Service) Preview(ctx context.Context, actor Actor, idempotencyKey, trigger string) (Import, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return Import{}, err
	}
	snapshot, err := s.source.Fetch(ctx, config)
	if err != nil {
		return Import{}, err
	}
	sourceKind := "google_sheets"
	if config.ConnectionID != nil {
		sourceKind = "google_drive"
	}
	return s.previewSnapshot(ctx, actor, idempotencyKey, trigger, sourceKind, &config.ID, config.SpreadsheetID, config.SheetName, config.ColumnMapping, snapshot, map[string]any{})
}

func (s *Service) PreviewExcel(ctx context.Context, actor Actor, idempotencyKey, filename, sourceName, sheetName string, columnMapping map[string]string, data []byte, options ExcelOptions) (Import, error) {
	snapshot, selectedSheet, metadata, err := ParseExcel(data, filename, sheetName, options)
	if err != nil {
		return Import{}, err
	}
	sourceName = strings.TrimSpace(sourceName)
	if sourceName == "" {
		sourceName = metadata.Filename
	}
	sourceName = strings.ToLower(strings.TrimSpace(sourceName))
	if len(sourceName) < 1 || len(sourceName) > 450 || strings.ContainsAny(sourceName, "\r\n\x00") {
		return Import{}, fmt.Errorf("%w: invalid Excel source name", ErrValidation)
	}
	return s.previewSnapshot(ctx, actor, idempotencyKey, "manual", "excel", nil, "excel:"+sourceName, selectedSheet, columnMapping, snapshot, map[string]any{"filename": metadata.Filename, "size_bytes": metadata.SizeBytes, "sheet_names": metadata.SheetNames})
}

func (s *Service) previewSnapshot(ctx context.Context, actor Actor, idempotencyKey, trigger, sourceKind string, configID *uuid.UUID, spreadsheetID, sheetName string, configuredMapping map[string]string, snapshot Snapshot, metadata map[string]any) (Import, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 200 {
		return Import{}, fmt.Errorf("%w: idempotency key", ErrValidation)
	}
	if existing, err := s.importByIdempotency(ctx, idempotencyKey); err == nil {
		return existing, nil
	}
	hash := SnapshotHash(snapshot)
	if existing, err := s.openImportByHash(ctx, sourceKind, spreadsheetID, sheetName, hash); err == nil {
		return existing, nil
	}
	rows, mapping, err := ParseRows(snapshot, configuredMapping, s.normalizer)
	if err != nil {
		return Import{}, err
	}
	existing, err := s.currentDomains(ctx)
	if err != nil {
		return Import{}, err
	}
	seen := map[string]bool{}
	for index := range rows {
		if !rows[index].Valid || rows[index].NormalizedValues == nil {
			continue
		}
		normalized := rows[index].NormalizedValues
		seen[normalized.Domain] = true
		current, exists := existing[normalized.Domain]
		if !exists {
			rows[index].Action = "ADD"
			rows[index].Diff = map[string]any{"domain": map[string]any{"before": nil, "after": normalized.Domain}}
			continue
		}
		rows[index].MatchedDomainID = &current.ID
		rows[index].Diff = sheetDiff(current, *normalized)
		if len(rows[index].Diff) > 0 {
			rows[index].Action = "MODIFY"
		} else {
			rows[index].Action = "UNCHANGED"
		}
	}
	nextRow := len(snapshot.Values) + 1
	for domainName, current := range existing {
		expectedType := "google_sheet"
		if sourceKind == "excel" {
			expectedType = "import"
		}
		if current.SourceType != expectedType || current.SourceStatus != "present" || current.SourceKind != sourceKind || current.SourceSpreadsheet != spreadsheetID || current.SourceSheet != sheetName || seen[domainName] {
			continue
		}
		id := current.ID
		rows = append(rows, ImportRow{ID: uuid.New(), RowNumber: nextRow, MatchedDomainID: &id, Action: "MISSING", Valid: true, RawValues: map[string]string{"domain": domainName}, NormalizedValues: &NormalizedRow{Domain: domainName, DomainUnicode: current.DomainUnicode, RegistrableDomain: current.RegistrableDomain, Active: false}, ValidationErrors: []string{}, Diff: map[string]any{"source_status": map[string]any{"before": "present", "after": "missing_from_source"}}})
		nextRow++
	}
	result := Import{ID: uuid.New(), ConfigID: configID, SpreadsheetID: spreadsheetID, SheetName: sheetName, Status: "preview", TriggerType: trigger, SourceKind: sourceKind, SourceMetadata: metadata, SourceRevision: snapshot.Revision, SourceHash: hashString(hash), ColumnMapping: mapping, RequestedBy: &actor.UserID, PreviewedAt: s.now().UTC(), Rows: rows}
	if result.TriggerType != "scheduled" {
		result.TriggerType = "manual"
	}
	countActions(&result)
	if err := s.persistPreview(ctx, result, hash, idempotencyKey, actor); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if existing, getErr := s.importByIdempotency(ctx, idempotencyKey); getErr == nil {
				return existing, nil
			}
			if existing, getErr := s.openImportByHash(ctx, sourceKind, spreadsheetID, sheetName, hash); getErr == nil {
				return existing, nil
			}
		}
		return Import{}, err
	}
	return result, nil
}

func (s *Service) persistPreview(ctx context.Context, result Import, hash []byte, idempotencyKey string, actor Actor) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	mapping, _ := json.Marshal(result.ColumnMapping)
	metadata, _ := json.Marshal(result.SourceMetadata)
	_, err = tx.Exec(ctx, `
		INSERT INTO google_sheet_imports (id,config_id,spreadsheet_id,sheet_name,status,source_revision,source_hash,column_mapping,total_rows,added_count,modified_count,unchanged_count,missing_count,invalid_count,requested_by,previewed_at,idempotency_key,trigger_type,source_kind,source_metadata)
		VALUES ($1,$2,$3,$4,'preview',NULLIF($5,''),$6,$7::jsonb,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19::jsonb)
	`, result.ID, result.ConfigID, result.SpreadsheetID, result.SheetName, result.SourceRevision, hash, mapping, result.TotalRows, result.AddedCount, result.ModifiedCount, result.UnchangedCount, result.MissingCount, result.InvalidCount, result.RequestedBy, result.PreviewedAt, idempotencyKey, result.TriggerType, result.SourceKind, metadata)
	if err != nil {
		return err
	}
	for _, row := range result.Rows {
		raw, _ := json.Marshal(row.RawValues)
		var normalized []byte
		if row.NormalizedValues != nil {
			normalized, _ = json.Marshal(row.NormalizedValues)
		}
		validation, _ := json.Marshal(row.ValidationErrors)
		diff, _ := json.Marshal(row.Diff)
		_, err = tx.Exec(ctx, `INSERT INTO google_sheet_import_rows (id,import_id,row_number,matched_domain_id,action,valid,raw_values,normalized_values,validation_errors,diff) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9::jsonb,$10::jsonb)`, row.ID, result.ID, row.RowNumber, row.MatchedDomainID, row.Action, row.Valid, raw, nullableJSON(normalized), validation, diff)
		if err != nil {
			return err
		}
	}
	if err := s.audit.AppendTx(ctx, tx, audit.Entry{ActorUserID: &actor.UserID, Action: "SHEET_IMPORT_PREVIEW_CREATED", ResourceType: "sheet_import", ResourceID: &result.ID, RequestID: actor.RequestID, After: map[string]any{"status": result.Status, "source_hash": result.SourceHash, "counts": map[string]int{"added": result.AddedCount, "modified": result.ModifiedCount, "unchanged": result.UnchangedCount, "missing": result.MissingCount, "invalid": result.InvalidCount}}, Metadata: map[string]any{"trigger_type": result.TriggerType, "source_kind": result.SourceKind}}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type currentDomain struct {
	ID, RegistrarID                                                                                                                                    uuid.UUID
	HasRegistrar                                                                                                                                       bool
	Domain, DomainUnicode, RegistrableDomain, Registrar, PurchasePrice, RenewalPrice, Currency, TaxRate, PurchaseDate, ExpirationDate, Priority, Notes string
	Active                                                                                                                                             bool
	SourceType, SourceStatus                                                                                                                           string
	SourceConfigID                                                                                                                                     *uuid.UUID
	SourceKind, SourceSpreadsheet, SourceSheet                                                                                                         string
}

func (s *Service) currentDomains(ctx context.Context) (map[string]currentDomain, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT d.id,d.domain_ascii,d.domain_unicode,d.registrable_domain,d.registrar_id,COALESCE(r.name,''),
		       COALESCE(p.amount::text,''),COALESCE(n.amount::text,''),COALESCE(n.currency_code::text,p.currency_code::text,''),COALESCE(n.tax_rate::text,''),
		       COALESCE(d.purchase_date::text,''),COALESCE(d.expiration_at::date::text,''),d.business_priority::text,d.notes,
		       d.lifecycle_status='active' AND d.monitoring_enabled,d.source_type,d.source_status::text,source.config_id,COALESCE(source.source_kind,''),COALESCE(source.spreadsheet_id,''),COALESCE(source.sheet_name,'')
		FROM domains d LEFT JOIN registrars r ON r.id=d.registrar_id
		LEFT JOIN LATERAL (SELECT amount,currency_code FROM domain_costs WHERE domain_id=d.id AND cost_type='purchase' AND price_source='google_sheet' AND effective_to IS NULL ORDER BY effective_from DESC,created_at DESC LIMIT 1) p ON true
		LEFT JOIN LATERAL (SELECT amount,currency_code,tax_rate FROM domain_costs WHERE domain_id=d.id AND cost_type='renewal' AND price_source='google_sheet' AND effective_to IS NULL ORDER BY effective_from DESC,created_at DESC LIMIT 1) n ON true
		LEFT JOIN LATERAL (
			SELECT imp.config_id,imp.source_kind,imp.spreadsheet_id,imp.sheet_name FROM domain_field_provenance fp
			JOIN google_sheet_imports imp ON fp.source_reference LIKE ANY (ARRAY['google-sheet:' || imp.id::text || ':row:%','sheet-import:' || imp.id::text || ':row:%'])
			WHERE fp.domain_id=d.id AND fp.field_name='domain' AND fp.source='google_sheet'
			ORDER BY fp.observed_at DESC,fp.created_at DESC LIMIT 1
		) source ON true
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]currentDomain{}
	for rows.Next() {
		var item currentDomain
		var registrarID *uuid.UUID
		if err := rows.Scan(&item.ID, &item.Domain, &item.DomainUnicode, &item.RegistrableDomain, &registrarID, &item.Registrar, &item.PurchasePrice, &item.RenewalPrice, &item.Currency, &item.TaxRate, &item.PurchaseDate, &item.ExpirationDate, &item.Priority, &item.Notes, &item.Active, &item.SourceType, &item.SourceStatus, &item.SourceConfigID, &item.SourceKind, &item.SourceSpreadsheet, &item.SourceSheet); err != nil {
			return nil, err
		}
		if registrarID != nil {
			item.RegistrarID, item.HasRegistrar = *registrarID, true
		}
		result[item.Domain] = item
	}
	return result, rows.Err()
}

func sheetDiff(current currentDomain, next NormalizedRow) map[string]any {
	diff := map[string]any{}
	compare := func(field, before, after string) {
		if strings.TrimSpace(before) != strings.TrimSpace(after) {
			diff[field] = map[string]any{"before": before, "after": after}
		}
	}
	compare("registrar", current.Registrar, next.Registrar)
	compare("purchase_price", current.PurchasePrice, pointerValue(next.PurchasePrice))
	compare("renewal_price", current.RenewalPrice, pointerValue(next.RenewalPrice))
	compare("currency", current.Currency, next.Currency)
	compare("tax_rate", current.TaxRate, pointerValue(next.TaxRate))
	compare("purchase_date", current.PurchaseDate, pointerValue(next.PurchaseDate))
	compare("expiration_date", current.ExpirationDate, pointerValue(next.ExpirationDate))
	compare("business_priority", current.Priority, next.BusinessPriority)
	compare("notes", current.Notes, next.Notes)
	if current.Active != next.Active {
		diff["active"] = map[string]any{"before": current.Active, "after": next.Active}
	}
	return diff
}

func countActions(result *Import) {
	result.TotalRows = len(result.Rows)
	for _, row := range result.Rows {
		switch row.Action {
		case "ADD":
			result.AddedCount++
		case "MODIFY":
			result.ModifiedCount++
		case "UNCHANGED":
			result.UnchangedCount++
		case "MISSING":
			result.MissingCount++
		case "INVALID":
			result.InvalidCount++
		}
	}
}

func (s *Service) GetImport(ctx context.Context, id uuid.UUID) (Import, error) {
	result, err := scanImport(s.pool.QueryRow(ctx, importSelect+` WHERE id=$1`, id))
	if err != nil {
		return Import{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,row_number,matched_domain_id,action,valid,raw_values,normalized_values,validation_errors,diff FROM google_sheet_import_rows WHERE import_id=$1 ORDER BY row_number`, id)
	if err != nil {
		return Import{}, err
	}
	defer rows.Close()
	result.Rows = []ImportRow{}
	for rows.Next() {
		row, scanErr := scanImportRow(rows)
		if scanErr != nil {
			return Import{}, scanErr
		}
		result.Rows = append(result.Rows, row)
	}
	return result, rows.Err()
}

func (s *Service) ListImports(ctx context.Context, limit int) ([]Import, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, importSelect+` ORDER BY previewed_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Import{}
	for rows.Next() {
		item, scanErr := scanImport(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const importSelect = `SELECT id,config_id,spreadsheet_id,sheet_name,status::text,trigger_type,source_kind,source_metadata,COALESCE(source_revision,''),COALESCE(encode(source_hash,'hex'),''),column_mapping,total_rows,added_count,modified_count,unchanged_count,missing_count,invalid_count,valid_rows_applied,requested_by,previewed_at,applied_at,rejected_at FROM google_sheet_imports`

const configSelect = `SELECT id,connection_id,spreadsheet_id,sheet_name,sheet_range,column_mapping,enabled,sync_interval_minutes,next_sync_at,last_sync_at,updated_by,created_at,updated_at,version FROM google_sheet_configs`

func (s *Service) importByIdempotency(ctx context.Context, key string) (Import, error) {
	result, err := scanImport(s.pool.QueryRow(ctx, importSelect+` WHERE idempotency_key=$1`, key))
	if err != nil {
		return Import{}, err
	}
	return s.GetImport(ctx, result.ID)
}

func (s *Service) openImportByHash(ctx context.Context, sourceKind, spreadsheetID, sheetName string, hash []byte) (Import, error) {
	result, err := scanImport(s.pool.QueryRow(ctx, importSelect+` WHERE source_kind=$1 AND spreadsheet_id=$2 AND sheet_name=$3 AND source_hash=$4 AND status='preview' ORDER BY previewed_at DESC LIMIT 1`, sourceKind, spreadsheetID, sheetName, hash))
	if err != nil {
		return Import{}, err
	}
	return s.GetImport(ctx, result.ID)
}

func scanConfig(row pgx.Row) (Config, error) {
	var result Config
	var mapping []byte
	err := row.Scan(&result.ID, &result.ConnectionID, &result.SpreadsheetID, &result.SheetName, &result.Range, &mapping, &result.Enabled, &result.SyncIntervalMinutes, &result.NextSyncAt, &result.LastSyncAt, &result.UpdatedBy, &result.CreatedAt, &result.UpdatedAt, &result.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return Config{}, ErrNotFound
	}
	if err != nil {
		return Config{}, err
	}
	_ = json.Unmarshal(mapping, &result.ColumnMapping)
	if result.ColumnMapping == nil {
		result.ColumnMapping = map[string]string{}
	}
	return result, nil
}

func scanImport(row pgx.Row) (Import, error) {
	var result Import
	var mapping, metadata []byte
	err := row.Scan(&result.ID, &result.ConfigID, &result.SpreadsheetID, &result.SheetName, &result.Status, &result.TriggerType, &result.SourceKind, &metadata, &result.SourceRevision, &result.SourceHash, &mapping, &result.TotalRows, &result.AddedCount, &result.ModifiedCount, &result.UnchangedCount, &result.MissingCount, &result.InvalidCount, &result.ValidRowsApplied, &result.RequestedBy, &result.PreviewedAt, &result.AppliedAt, &result.RejectedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Import{}, ErrNotFound
	}
	if err != nil {
		return Import{}, err
	}
	_ = json.Unmarshal(mapping, &result.ColumnMapping)
	_ = json.Unmarshal(metadata, &result.SourceMetadata)
	if result.SourceMetadata == nil {
		result.SourceMetadata = map[string]any{}
	}
	return result, nil
}

func scanImportRow(row pgx.Row) (ImportRow, error) {
	var result ImportRow
	var raw, normalized, validation, diff []byte
	err := row.Scan(&result.ID, &result.RowNumber, &result.MatchedDomainID, &result.Action, &result.Valid, &raw, &normalized, &validation, &diff)
	if err != nil {
		return ImportRow{}, err
	}
	_ = json.Unmarshal(raw, &result.RawValues)
	if len(normalized) > 0 && string(normalized) != "null" {
		var value NormalizedRow
		if json.Unmarshal(normalized, &value) == nil {
			result.NormalizedValues = &value
		}
	}
	_ = json.Unmarshal(validation, &result.ValidationErrors)
	_ = json.Unmarshal(diff, &result.Diff)
	return result, nil
}

func nullableConfig(value Config, err error) any {
	if err != nil {
		return nil
	}
	return value
}
func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
