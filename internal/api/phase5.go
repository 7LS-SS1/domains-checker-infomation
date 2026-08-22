package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"domainmonitor/internal/auth"
	"domainmonitor/internal/drive"
	"domainmonitor/internal/finance"
	"domainmonitor/internal/i18n"
	"domainmonitor/internal/rdap"
	"domainmonitor/internal/sheets"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) getRDAP(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	result, err := s.intelligence.RDAP.Latest(r.Context(), id)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "RDAP_RESULT_NOT_FOUND", nil)
		return
	}
	writeData(w, http.StatusOK, result)
}

type rdapCheckRequest struct {
	Force bool `json:"force"`
}

func (s *Server) checkRDAP(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	request := rdapCheckRequest{}
	if r.ContentLength != 0 && !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.RDAP.Check(r.Context(), rdap.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, id, request.Force)
	if err != nil {
		switch {
		case errors.Is(err, rdap.ErrRateLimited):
			writeError(w, r, http.StatusServiceUnavailable, "RDAP_RATE_LIMITED", map[string]any{"check_id": result.ID})
		case errors.Is(err, rdap.ErrNoBootstrap):
			writeError(w, r, http.StatusServiceUnavailable, "RDAP_BOOTSTRAP_NOT_FOUND", map[string]any{"check_id": result.ID})
		default:
			s.logger.WarnContext(r.Context(), "rdap_check_unavailable", "error", err, "domain_id", id)
			writeError(w, r, http.StatusServiceUnavailable, "RDAP_UNAVAILABLE", map[string]any{"check_id": result.ID})
		}
		return
	}
	writeData(w, http.StatusOK, result)
}

type addCostRequest struct {
	CostType           string  `json:"cost_type"`
	Amount             string  `json:"amount"`
	Currency           string  `json:"currency"`
	TaxRate            *string `json:"tax_rate"`
	TaxMode            string  `json:"tax_mode"`
	BillingCycleMonths int     `json:"billing_cycle_months"`
	EffectiveFrom      *string `json:"effective_from"`
	SourceReference    string  `json:"source_reference"`
	Reason             string  `json:"reason"`
}

func (s *Server) addDomainCost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	var request addCostRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	effective := time.Time{}
	if request.EffectiveFrom != nil {
		parsed, err := time.Parse("2006-01-02", *request.EffectiveFrom)
		if err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "FINANCE_VALIDATION_FAILED", map[string]any{"field": "effective_from"})
			return
		}
		effective = parsed
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Finance.AddCost(r.Context(), finance.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, id, finance.AddCostInput{CostType: request.CostType, Amount: request.Amount, Currency: request.Currency, TaxRate: request.TaxRate, TaxMode: request.TaxMode, BillingCycleMonths: request.BillingCycleMonths, EffectiveFrom: effective, SourceReference: request.SourceReference, Reason: request.Reason})
	if !s.handleFinanceError(w, r, err) {
		return
	}
	writeData(w, http.StatusCreated, result)
}
func (s *Server) listDomainCosts(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	result, err := s.intelligence.Finance.ListCosts(r.Context(), id)
	if !s.handleFinanceError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": result})
}

type addRateRequest struct {
	BaseCurrency  string `json:"base_currency"`
	QuoteCurrency string `json:"quote_currency"`
	Rate          string `json:"rate"`
	Source        string `json:"source"`
	ObservedAt    string `json:"observed_at"`
	Reason        string `json:"reason"`
}

func (s *Server) addExchangeRate(w http.ResponseWriter, r *http.Request) {
	var request addRateRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	observed, err := time.Parse(time.RFC3339, request.ObservedAt)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "FINANCE_VALIDATION_FAILED", map[string]any{"field": "observed_at"})
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Finance.AddRate(r.Context(), finance.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, finance.AddRateInput{BaseCurrency: request.BaseCurrency, QuoteCurrency: request.QuoteCurrency, Rate: request.Rate, Source: request.Source, ObservedAt: observed, Reason: request.Reason})
	if !s.handleFinanceError(w, r, err) {
		return
	}
	writeData(w, http.StatusCreated, result)
}
func (s *Server) getFinanceSummary(w http.ResponseWriter, r *http.Request) {
	currency := strings.TrimSpace(r.URL.Query().Get("reporting_currency"))
	if currency == "" {
		currency = "THB"
	}
	result, err := s.intelligence.Finance.BudgetSummary(r.Context(), currency)
	if !s.handleFinanceError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}

type overrideRequest struct {
	FieldName     string          `json:"field_name"`
	OverrideValue json.RawMessage `json:"override_value"`
	Reason        string          `json:"reason"`
	ExpiresAt     *string         `json:"expires_at"`
}

func (s *Server) createDomainOverride(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	var request overrideRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	expires, valid := parseOptionalTime(w, r, "expires_at", request.ExpiresAt)
	if !valid {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Finance.CreateOverride(r.Context(), finance.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, id, request.FieldName, request.OverrideValue, request.Reason, expires)
	if !s.handleFinanceError(w, r, err) {
		return
	}
	writeData(w, http.StatusCreated, result)
}
func (s *Server) listDomainOverrides(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	result, err := s.intelligence.Finance.ListOverrides(r.Context(), id)
	if !s.handleFinanceError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": result})
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) revokeDomainOverride(w http.ResponseWriter, r *http.Request) {
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	overrideID, err := uuid.Parse(chi.URLParam(r, "overrideID"))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", map[string]any{"field": "override_id"})
		return
	}
	var request reasonRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	err = s.intelligence.Finance.RevokeOverride(r.Context(), finance.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, domainID, overrideID, request.Reason)
	if !s.handleFinanceError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, map[string]any{"message": i18n.Message("MANUAL_OVERRIDE_REVOKED", i18n.FromContext(r.Context(), i18n.Thai)), "messages": i18n.Messages("MANUAL_OVERRIDE_REVOKED")})
}

func (s *Server) handleFinanceError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, finance.ErrValidation):
		writeError(w, r, http.StatusUnprocessableEntity, "FINANCE_VALIDATION_FAILED", nil)
	case errors.Is(err, finance.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "FINANCE_NOT_FOUND", nil)
	case errors.Is(err, finance.ErrConflict):
		writeError(w, r, http.StatusConflict, "VERSION_CONFLICT", nil)
	default:
		s.logger.ErrorContext(r.Context(), "finance_operation_failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
	}
	return false
}

type sheetConfigRequest struct {
	ConnectionID        *uuid.UUID        `json:"connection_id"`
	SpreadsheetID       string            `json:"spreadsheet_id"`
	SheetName           string            `json:"sheet_name"`
	Range               string            `json:"range"`
	ColumnMapping       map[string]string `json:"column_mapping"`
	Enabled             bool              `json:"enabled"`
	SyncIntervalMinutes int               `json:"sync_interval_minutes"`
	Version             int64             `json:"version"`
	Reason              string            `json:"reason"`
}

func (s *Server) getSheetConfig(w http.ResponseWriter, r *http.Request) {
	result, err := s.intelligence.Sheets.GetConfig(r.Context())
	if !s.handleSheetError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}
func (s *Server) saveSheetConfig(w http.ResponseWriter, r *http.Request) {
	var request sheetConfigRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Sheets.SaveConfig(r.Context(), sheets.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, sheets.ConfigInput{ConnectionID: request.ConnectionID, SpreadsheetID: request.SpreadsheetID, SheetName: request.SheetName, Range: request.Range, ColumnMapping: request.ColumnMapping, Enabled: request.Enabled, SyncIntervalMinutes: request.SyncIntervalMinutes, Version: request.Version, Reason: request.Reason})
	if !s.handleSheetError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}
func (s *Server) previewSheetImport(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Sheets.Preview(r.Context(), sheets.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, r.Header.Get("Idempotency-Key"), "manual")
	if !s.handleSheetError(w, r, err) {
		return
	}
	w.Header().Set("Location", "/api/v1/google-sheets/imports/"+result.ID.String())
	writeData(w, http.StatusCreated, result)
}

func (s *Server) previewExcelImport(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(s.cfg.ExcelImportMaxBytes); err != nil {
		writeError(w, r, http.StatusRequestEntityTooLarge, "EXCEL_IMPORT_TOO_LARGE", nil)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "EXCEL_IMPORT_INVALID", map[string]any{"field": "file"})
		return
	}
	defer file.Close()
	if header.Size <= 0 || header.Size > s.cfg.ExcelImportMaxBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, "EXCEL_IMPORT_TOO_LARGE", nil)
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, s.cfg.ExcelImportMaxBytes+1))
	if err != nil || int64(len(data)) > s.cfg.ExcelImportMaxBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge, "EXCEL_IMPORT_TOO_LARGE", nil)
		return
	}
	mapping := map[string]string{}
	if raw := strings.TrimSpace(r.FormValue("column_mapping")); raw != "" {
		if json.Unmarshal([]byte(raw), &mapping) != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "EXCEL_IMPORT_INVALID", map[string]any{"field": "column_mapping"})
			return
		}
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Sheets.PreviewExcel(r.Context(), sheets.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, r.Header.Get("Idempotency-Key"), header.Filename, r.FormValue("source_name"), r.FormValue("sheet_name"), mapping, data, sheets.ExcelOptions{MaxBytes: s.cfg.ExcelImportMaxBytes, MaxUncompressedBytes: s.cfg.ExcelUnzipMaxBytes, MaxRows: s.cfg.ExcelImportMaxRows, MaxColumns: s.cfg.ExcelImportMaxColumns})
	if !s.handleSheetError(w, r, err) {
		return
	}
	w.Header().Set("Location", "/api/v1/google-sheets/imports/"+result.ID.String())
	writeData(w, http.StatusCreated, result)
}

func (s *Server) connectGoogleDrive(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Drive.Begin(r.Context(), drive.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())})
	if !s.handleDriveError(w, r, err) {
		return
	}
	writeData(w, http.StatusCreated, result)
}

func (s *Server) googleDriveCallback(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Drive.Complete(r.Context(), drive.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if !s.handleDriveError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}

func (s *Server) getGoogleDriveConnection(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Drive.Get(r.Context(), principal.UserID)
	if !s.handleDriveError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}

func (s *Server) listGoogleDriveFiles(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Drive.ListFiles(r.Context(), principal.UserID, r.URL.Query().Get("page_token"))
	if !s.handleDriveError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}

func (s *Server) disconnectGoogleDrive(w http.ResponseWriter, r *http.Request) {
	var request reasonRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	err := s.intelligence.Drive.Disconnect(r.Context(), drive.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, request.Reason)
	if !s.handleDriveError(w, r, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDriveError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, drive.ErrNotConfigured):
		writeError(w, r, http.StatusServiceUnavailable, "DRIVE_NOT_CONFIGURED", nil)
	case errors.Is(err, drive.ErrNotConnected):
		writeError(w, r, http.StatusNotFound, "DRIVE_NOT_CONNECTED", nil)
	case errors.Is(err, drive.ErrOAuthState):
		writeError(w, r, http.StatusUnprocessableEntity, "DRIVE_OAUTH_STATE_INVALID", nil)
	case errors.Is(err, drive.ErrValidation):
		writeError(w, r, http.StatusUnprocessableEntity, "DRIVE_VALIDATION_FAILED", nil)
	case errors.Is(err, drive.ErrUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "DRIVE_UNAVAILABLE", nil)
	default:
		s.logger.ErrorContext(r.Context(), "drive_operation_failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
	}
	return false
}
func (s *Server) listSheetImports(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := s.intelligence.Sheets.ListImports(r.Context(), limit)
	if !s.handleSheetError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": result})
}
func parseImportID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "importID"))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", map[string]any{"field": "import_id"})
		return uuid.Nil, false
	}
	return id, true
}
func (s *Server) getSheetImport(w http.ResponseWriter, r *http.Request) {
	id, ok := parseImportID(w, r)
	if !ok {
		return
	}
	result, err := s.intelligence.Sheets.GetImport(r.Context(), id)
	if !s.handleSheetError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}
func (s *Server) applySheetImport(w http.ResponseWriter, r *http.Request) {
	id, ok := parseImportID(w, r)
	if !ok {
		return
	}
	var request reasonRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Sheets.Apply(r.Context(), sheets.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, id, r.Header.Get("Idempotency-Key"), request.Reason)
	if !s.handleSheetError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}
func (s *Server) rejectSheetImport(w http.ResponseWriter, r *http.Request) {
	id, ok := parseImportID(w, r)
	if !ok {
		return
	}
	var request reasonRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Sheets.Reject(r.Context(), sheets.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, id, request.Reason)
	if !s.handleSheetError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}
func (s *Server) handleSheetError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, drive.ErrNotConfigured):
		writeError(w, r, http.StatusServiceUnavailable, "DRIVE_NOT_CONFIGURED", nil)
	case errors.Is(err, drive.ErrNotConnected):
		writeError(w, r, http.StatusServiceUnavailable, "DRIVE_NOT_CONNECTED", nil)
	case errors.Is(err, drive.ErrUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "DRIVE_UNAVAILABLE", nil)
	case errors.Is(err, sheets.ErrCredentials):
		writeError(w, r, http.StatusServiceUnavailable, "SHEETS_CREDENTIALS_UNAVAILABLE", nil)
	case errors.Is(err, sheets.ErrUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "SHEETS_UNAVAILABLE", nil)
	case errors.Is(err, sheets.ErrValidation):
		writeError(w, r, http.StatusUnprocessableEntity, "SHEETS_VALIDATION_FAILED", map[string]any{"reason": err.Error()})
	case errors.Is(err, sheets.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "SHEETS_NOT_FOUND", nil)
	case errors.Is(err, sheets.ErrConflict):
		writeError(w, r, http.StatusConflict, "SHEETS_CONFLICT", nil)
	default:
		s.logger.ErrorContext(r.Context(), "sheet_operation_failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
	}
	return false
}

func (s *Server) getDomainProvenance(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	items, err := s.domains.Provenance(r.Context(), id)
	if !s.handleDomainError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}
