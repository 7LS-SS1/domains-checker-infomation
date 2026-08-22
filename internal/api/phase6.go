package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"domainmonitor/internal/auth"
	"domainmonitor/internal/finance"
	"domainmonitor/internal/recommendation"
	"domainmonitor/internal/report"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) getDomainRecommendation(w http.ResponseWriter, r *http.Request) {
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	result, err := s.intelligence.Recommendations.GetLatest(r.Context(), domainID)
	if !s.handleRecommendationError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}

func (s *Server) generateDomainRecommendation(w http.ResponseWriter, r *http.Request) {
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Recommendations.Generate(r.Context(), recommendation.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, domainID)
	if !s.handleRecommendationError(w, r, err) {
		return
	}
	writeData(w, http.StatusCreated, result)
}

type recommendationRunRequest struct {
	Limit int `json:"limit"`
}

func (s *Server) runRecommendations(w http.ResponseWriter, r *http.Request) {
	var request recommendationRunRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	items, err := s.intelligence.Recommendations.Run(r.Context(), recommendation.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, request.Limit)
	if !s.handleRecommendationError(w, r, err) {
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"items": items, "count": len(items), "policy_version": recommendation.PolicyVersion})
}

func (s *Server) listRecommendations(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.intelligence.Recommendations.List(r.Context(), r.URL.Query().Get("action"), limit)
	if !s.handleRecommendationError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleRecommendationError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, recommendation.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "RECOMMENDATION_NOT_FOUND", nil)
	case errors.Is(err, recommendation.ErrValidation):
		writeError(w, r, http.StatusUnprocessableEntity, "RECOMMENDATION_VALIDATION_FAILED", nil)
	default:
		s.logger.ErrorContext(r.Context(), "recommendation_operation_failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
	}
	return false
}

func (s *Server) getReportSummary(w http.ResponseWriter, r *http.Request) {
	result, err := s.intelligence.Reports.Summary(r.Context(), r.URL.Query().Get("reporting_currency"))
	if !s.handleReportError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}

type reportRequest struct {
	Format            string `json:"format"`
	ReportingCurrency string `json:"reporting_currency"`
}

func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	var request reportRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.intelligence.Reports.Create(r.Context(), report.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, r.Header.Get("Idempotency-Key"), request.Format, request.ReportingCurrency)
	if !s.handleReportError(w, r, err) {
		return
	}
	w.Header().Set("Location", "/api/v1/reports/"+result.ID.String())
	writeData(w, http.StatusCreated, result)
}

func parseReportID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "reportID"))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "REPORT_VALIDATION_FAILED", map[string]any{"field": "report_id"})
		return uuid.Nil, false
	}
	return id, true
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	id, ok := parseReportID(w, r)
	if !ok {
		return
	}
	result, err := s.intelligence.Reports.Get(r.Context(), id)
	if !s.handleReportError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}

func (s *Server) downloadReport(w http.ResponseWriter, r *http.Request) {
	id, ok := parseReportID(w, r)
	if !ok {
		return
	}
	payload, err := s.intelligence.Reports.Download(r.Context(), id)
	if !s.handleReportError(w, r, err) {
		return
	}
	extension := strings.ToLower(payload.Record.Format)
	w.Header().Set("Content-Type", payload.ContentType+"; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=domain-summary-%s.%s", payload.Record.ID, extension))
	w.Header().Set("X-Content-SHA256", payload.Record.SHA256)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload.Content)
}

func (s *Server) handleReportError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, report.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "REPORT_NOT_FOUND", nil)
	case errors.Is(err, report.ErrValidation), errors.Is(err, finance.ErrValidation):
		writeError(w, r, http.StatusUnprocessableEntity, "REPORT_VALIDATION_FAILED", nil)
	default:
		s.logger.ErrorContext(r.Context(), "report_operation_failed", "error", err, "request_id", RequestID(r.Context()))
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
	}
	return false
}
