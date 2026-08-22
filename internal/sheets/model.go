package sheets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"domainmonitor/internal/domain"
	"domainmonitor/internal/finance"
	"github.com/google/uuid"
)

var (
	ErrNotFound   = errors.New("sheet resource not found")
	ErrConflict   = errors.New("sheet state conflict")
	ErrValidation = errors.New("sheet validation failed")
)

type Config struct {
	ID                  uuid.UUID         `json:"id"`
	ConnectionID        *uuid.UUID        `json:"connection_id,omitempty"`
	SpreadsheetID       string            `json:"spreadsheet_id"`
	SheetName           string            `json:"sheet_name"`
	Range               string            `json:"range"`
	ColumnMapping       map[string]string `json:"column_mapping"`
	Enabled             bool              `json:"enabled"`
	SyncIntervalMinutes int               `json:"sync_interval_minutes"`
	NextSyncAt          *time.Time        `json:"next_sync_at,omitempty"`
	LastSyncAt          *time.Time        `json:"last_sync_at,omitempty"`
	UpdatedBy           uuid.UUID         `json:"updated_by"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	Version             int64             `json:"version"`
}

type ConfigInput struct {
	ConnectionID        *uuid.UUID
	SpreadsheetID       string
	SheetName           string
	Range               string
	ColumnMapping       map[string]string
	Enabled             bool
	SyncIntervalMinutes int
	Version             int64
	Reason              string
}

type Actor struct {
	UserID    uuid.UUID
	RequestID string
}

type NormalizedRow struct {
	Domain            string  `json:"domain"`
	DomainUnicode     string  `json:"domain_unicode"`
	RegistrableDomain string  `json:"registrable_domain"`
	Registrar         string  `json:"registrar,omitempty"`
	PurchasePrice     *string `json:"purchase_price,omitempty"`
	RenewalPrice      *string `json:"renewal_price,omitempty"`
	Currency          string  `json:"currency,omitempty"`
	TaxRate           *string `json:"tax_rate,omitempty"`
	PurchaseDate      *string `json:"purchase_date,omitempty"`
	ExpirationDate    *string `json:"expiration_date,omitempty"`
	BusinessPriority  string  `json:"business_priority"`
	Notes             string  `json:"notes"`
	Active            bool    `json:"active"`
}

type ImportRow struct {
	ID               uuid.UUID         `json:"id"`
	RowNumber        int               `json:"row_number"`
	MatchedDomainID  *uuid.UUID        `json:"matched_domain_id,omitempty"`
	Action           string            `json:"action"`
	Valid            bool              `json:"valid"`
	RawValues        map[string]string `json:"raw_values"`
	NormalizedValues *NormalizedRow    `json:"normalized_values,omitempty"`
	ValidationErrors []string          `json:"validation_errors"`
	Diff             map[string]any    `json:"diff"`
}

type Import struct {
	ID               uuid.UUID         `json:"id"`
	ConfigID         *uuid.UUID        `json:"config_id,omitempty"`
	SpreadsheetID    string            `json:"spreadsheet_id"`
	SheetName        string            `json:"sheet_name"`
	Status           string            `json:"status"`
	TriggerType      string            `json:"trigger_type"`
	SourceKind       string            `json:"source_kind"`
	SourceMetadata   map[string]any    `json:"source_metadata"`
	SourceRevision   string            `json:"source_revision"`
	SourceHash       string            `json:"source_hash"`
	ColumnMapping    map[string]string `json:"column_mapping"`
	TotalRows        int               `json:"total_rows"`
	AddedCount       int               `json:"added_count"`
	ModifiedCount    int               `json:"modified_count"`
	UnchangedCount   int               `json:"unchanged_count"`
	MissingCount     int               `json:"missing_count"`
	InvalidCount     int               `json:"invalid_count"`
	ValidRowsApplied int               `json:"valid_rows_applied"`
	RequestedBy      *uuid.UUID        `json:"requested_by,omitempty"`
	PreviewedAt      time.Time         `json:"previewed_at"`
	AppliedAt        *time.Time        `json:"applied_at,omitempty"`
	RejectedAt       *time.Time        `json:"rejected_at,omitempty"`
	Rows             []ImportRow       `json:"rows,omitempty"`
}

var requiredSemantics = []string{"domain"}

var headerAliases = map[string][]string{
	"domain":            {"domain", "domain_name", "โดเมน"},
	"registrar":         {"registrar", "provider", "domain_provider", "ผู้ให้บริการ"},
	"purchase_price":    {"purchase_price", "buy_price", "ราคาซื้อ"},
	"renewal_price":     {"renewal_price", "renew_price", "ราคาต่ออายุ"},
	"currency":          {"currency", "สกุลเงิน"},
	"tax_rate":          {"tax_rate", "vat_rate", "อัตราภาษี"},
	"purchase_date":     {"purchase_date", "วันที่ซื้อ"},
	"expiration_date":   {"expiration_date", "expiry_date", "expires_at", "วันหมดอายุ"},
	"business_priority": {"business_priority", "priority", "ความสำคัญ"},
	"notes":             {"notes", "note", "หมายเหตุ"},
	"active":            {"active", "enabled", "ใช้งาน"},
}

func ParseRows(snapshot Snapshot, configured map[string]string, normalizer domain.Normalizer) ([]ImportRow, map[string]string, error) {
	if len(snapshot.Values) == 0 {
		return nil, nil, fmt.Errorf("%w: sheet has no header row", ErrValidation)
	}
	headers := snapshot.Values[0]
	mapping, indexes, err := resolveMapping(headers, configured)
	if err != nil {
		return nil, nil, err
	}
	rows := make([]ImportRow, 0, max(0, len(snapshot.Values)-1))
	for rowIndex, cells := range snapshot.Values[1:] {
		raw := map[string]string{}
		for semantic, index := range indexes {
			if index < len(cells) {
				raw[semantic] = strings.TrimSpace(cells[index])
			} else {
				raw[semantic] = ""
			}
		}
		row := ImportRow{ID: uuid.New(), RowNumber: rowIndex + 2, Action: "INVALID", Valid: false, RawValues: raw, ValidationErrors: []string{}, Diff: map[string]any{}}
		if allEmpty(raw) {
			row.ValidationErrors = append(row.ValidationErrors, "EMPTY_ROW")
			rows = append(rows, row)
			continue
		}
		normalized, validationErrors := normalizeRow(raw, normalizer)
		row.NormalizedValues = &normalized
		row.ValidationErrors = validationErrors
		row.Valid = len(validationErrors) == 0
		if row.Valid {
			row.Action = "UNCHANGED"
		}
		rows = append(rows, row)
	}
	duplicates := map[string][]int{}
	for index, row := range rows {
		if row.Valid && row.NormalizedValues != nil {
			duplicates[row.NormalizedValues.Domain] = append(duplicates[row.NormalizedValues.Domain], index)
		}
	}
	for domainName, indexes := range duplicates {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			rows[index].Valid = false
			rows[index].Action = "INVALID"
			rows[index].ValidationErrors = append(rows[index].ValidationErrors, "DUPLICATE_DOMAIN:"+domainName)
		}
	}
	return rows, mapping, nil
}

func resolveMapping(headers []string, configured map[string]string) (map[string]string, map[string]int, error) {
	byNormalized := map[string][]int{}
	for index, header := range headers {
		normalized := normalizeHeader(header)
		if normalized != "" {
			byNormalized[normalized] = append(byNormalized[normalized], index)
		}
	}
	mapping := map[string]string{}
	indexes := map[string]int{}
	for semantic := range headerAliases {
		wanted := strings.TrimSpace(configured[semantic])
		candidates := []string{}
		if wanted != "" {
			candidates = []string{wanted}
		} else {
			candidates = headerAliases[semantic]
		}
		matches := []int{}
		for _, candidate := range candidates {
			matches = append(matches, byNormalized[normalizeHeader(candidate)]...)
		}
		matches = uniqueInts(matches)
		if len(matches) > 1 {
			return nil, nil, fmt.Errorf("%w: ambiguous column mapping for %s", ErrValidation, semantic)
		}
		if len(matches) == 1 {
			indexes[semantic] = matches[0]
			mapping[semantic] = headers[matches[0]]
		}
	}
	for _, required := range requiredSemantics {
		if _, ok := indexes[required]; !ok {
			return nil, nil, fmt.Errorf("%w: required column %s is missing", ErrValidation, required)
		}
	}
	return mapping, indexes, nil
}

func normalizeRow(raw map[string]string, normalizer domain.Normalizer) (NormalizedRow, []string) {
	result := NormalizedRow{Registrar: strings.TrimSpace(raw["registrar"]), Currency: strings.ToUpper(strings.TrimSpace(raw["currency"])), BusinessPriority: strings.ToLower(strings.TrimSpace(raw["business_priority"])), Notes: strings.TrimSpace(raw["notes"]), Active: true}
	errorsList := []string{}
	normalized, err := normalizer.Normalize(raw["domain"])
	if err != nil {
		errorsList = append(errorsList, "INVALID_DOMAIN")
	} else {
		result.Domain, result.DomainUnicode, result.RegistrableDomain = normalized.ASCII, normalized.Unicode, normalized.RegistrableDomain
	}
	if result.BusinessPriority == "" {
		result.BusinessPriority = "medium"
	}
	if result.BusinessPriority != "low" && result.BusinessPriority != "medium" && result.BusinessPriority != "high" && result.BusinessPriority != "critical" {
		errorsList = append(errorsList, "INVALID_BUSINESS_PRIORITY")
	}
	if raw["active"] != "" {
		active, parseErr := parseBool(raw["active"])
		if parseErr != nil {
			errorsList = append(errorsList, "INVALID_ACTIVE")
		} else {
			result.Active = active
		}
	}
	for field, target := range map[string]**string{"purchase_price": &result.PurchasePrice, "renewal_price": &result.RenewalPrice} {
		if raw[field] == "" {
			continue
		}
		decimal, parseErr := finance.ParseDecimal(raw[field], 6)
		if parseErr != nil || decimal.Sign() < 0 {
			errorsList = append(errorsList, "INVALID_"+strings.ToUpper(field))
			continue
		}
		value := decimal.String(6)
		*target = &value
	}
	if (result.PurchasePrice != nil || result.RenewalPrice != nil) && len(result.Currency) != 3 {
		errorsList = append(errorsList, "CURRENCY_REQUIRED_FOR_PRICE")
	}
	if raw["tax_rate"] != "" {
		decimal, parseErr := finance.ParseDecimal(raw["tax_rate"], 10)
		if parseErr != nil || decimal.Sign() < 0 {
			errorsList = append(errorsList, "INVALID_TAX_RATE")
		} else {
			value := decimal.String(10)
			if value > "1.0000000000" {
				errorsList = append(errorsList, "INVALID_TAX_RATE")
			} else {
				result.TaxRate = &value
			}
		}
	}
	for field, target := range map[string]**string{"purchase_date": &result.PurchaseDate, "expiration_date": &result.ExpirationDate} {
		if raw[field] == "" {
			continue
		}
		parsed, parseErr := parseDate(raw[field])
		if parseErr != nil {
			errorsList = append(errorsList, "INVALID_"+strings.ToUpper(field))
			continue
		}
		value := parsed.Format("2006-01-02")
		*target = &value
	}
	return result, errorsList
}

func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, errors.New("invalid date")
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "y", "active", "ใช่", "ใช้งาน":
		return true, nil
	case "false", "0", "no", "n", "inactive", "ไม่", "ไม่ใช้งาน":
		return false, nil
	default:
		return false, errors.New("invalid boolean")
	}
}

func SnapshotHash(snapshot Snapshot) []byte {
	encoded, _ := json.Marshal(snapshot.Values)
	digest := sha256.Sum256(encoded)
	return digest[:]
}

func hashString(value []byte) string { return hex.EncodeToString(value) }

func normalizeHeader(value string) string {
	var result strings.Builder
	underscore := false
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			result.WriteRune(character)
			underscore = false
		} else if !underscore && result.Len() > 0 {
			result.WriteByte('_')
			underscore = true
		}
	}
	return strings.Trim(result.String(), "_")
}

func uniqueInts(values []int) []int {
	seen := map[int]bool{}
	result := []int{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func allEmpty(values map[string]string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func rowJSON(row NormalizedRow) []byte {
	encoded, _ := json.Marshal(row)
	return encoded
}
