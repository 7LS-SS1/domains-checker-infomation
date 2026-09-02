package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"domainmonitor/internal/auth"
	"domainmonitor/internal/i18n"
	"domainmonitor/internal/monitor"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) createManualCheck(w http.ResponseWriter, r *http.Request) {
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	run, created, err := s.monitors.CreateManualRun(
		r.Context(), domainID, principal.UserID, RequestID(r.Context()), r.Header.Get("Idempotency-Key"),
	)
	if err != nil {
		switch {
		case errors.Is(err, monitor.ErrInvalidIdempotencyKey):
			writeError(w, r, http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REQUIRED", map[string]any{"header": "Idempotency-Key", "max_length": 200})
		case errors.Is(err, monitor.ErrDomainNotFound):
			writeError(w, r, http.StatusNotFound, "DOMAIN_NOT_FOUND", nil)
		case errors.Is(err, monitor.ErrDomainInactive):
			writeError(w, r, http.StatusConflict, "DOMAIN_INACTIVE", nil)
		default:
			s.logger.ErrorContext(r.Context(), "manual_monitoring_run_failed", "domain_id", domainID, "request_id", RequestID(r.Context()), "error", err)
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		}
		return
	}
	w.Header().Set("Location", "/api/v1/monitoring-runs/"+run.ID.String())
	writeData(w, http.StatusAccepted, map[string]any{
		"run": run, "created": created,
		"message":  i18n.Message("MONITOR_RUN_ACCEPTED", i18n.FromContext(r.Context(), i18n.Thai)),
		"messages": i18n.Messages("MONITOR_RUN_ACCEPTED"),
	})
}

func (s *Server) createISPCheck(w http.ResponseWriter, r *http.Request) {
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	run, created, err := s.monitors.CreateManualISPCheckRun(
		r.Context(), domainID, principal.UserID, RequestID(r.Context()), r.Header.Get("Idempotency-Key"),
	)
	if err != nil {
		switch {
		case errors.Is(err, monitor.ErrInvalidIdempotencyKey):
			writeError(w, r, http.StatusUnprocessableEntity, "IDEMPOTENCY_KEY_REQUIRED", map[string]any{"header": "Idempotency-Key", "max_length": 200})
		case errors.Is(err, monitor.ErrDomainNotFound):
			writeError(w, r, http.StatusNotFound, "DOMAIN_NOT_FOUND", nil)
		case errors.Is(err, monitor.ErrDomainInactive):
			writeError(w, r, http.StatusConflict, "DOMAIN_INACTIVE", nil)
		default:
			s.logger.ErrorContext(r.Context(), "isp_check_run_failed", "domain_id", domainID, "request_id", RequestID(r.Context()), "error", err)
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		}
		return
	}
	w.Header().Set("Location", "/api/v1/monitoring-runs/"+run.ID.String())
	writeData(w, http.StatusAccepted, map[string]any{
		"run": run, "created": created,
		"message":  i18n.Message("ISP_CHECK_ACCEPTED", i18n.FromContext(r.Context(), i18n.Thai)),
		"messages": i18n.Messages("ISP_CHECK_ACCEPTED"),
	})
}

func (s *Server) getMonitoringRun(w http.ResponseWriter, r *http.Request) {
	runID, err := uuid.Parse(chi.URLParam(r, "runID"))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", map[string]any{"field": "run_id", "reason": "must be a UUID"})
		return
	}
	detail, err := s.monitors.GetRunDetail(r.Context(), runID)
	if err != nil {
		if errors.Is(err, monitor.ErrRunNotFound) {
			writeError(w, r, http.StatusNotFound, "MONITOR_RUN_NOT_FOUND", nil)
			return
		}
		s.logger.ErrorContext(r.Context(), "get_monitoring_run_failed", "monitor_run_id", runID, "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		return
	}
	writeData(w, http.StatusOK, detail)
}

func (s *Server) listMonitoringRuns(w http.ResponseWriter, r *http.Request) {
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	runs, err := s.monitors.ListRuns(r.Context(), domainID, page, pageSize)
	if err != nil {
		if errors.Is(err, monitor.ErrDomainNotFound) {
			writeError(w, r, http.StatusNotFound, "DOMAIN_NOT_FOUND", nil)
			return
		}
		s.logger.ErrorContext(r.Context(), "list_monitoring_runs_failed", "domain_id", domainID, "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		return
	}
	writeData(w, http.StatusOK, runs)
}

func (s *Server) getMonitoringHistory(w http.ResponseWriter, r *http.Request) {
	domainID, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	duration, valid := historyWindow(r.URL.Query().Get("window"))
	if !valid {
		writeError(w, r, http.StatusUnprocessableEntity, "INVALID_HISTORY_WINDOW", map[string]any{"allowed": []string{"24h", "7d", "30d", "90d"}})
		return
	}
	until := time.Now().UTC()
	history, err := s.monitors.GetHistory(r.Context(), domainID, until.Add(-duration), until)
	if err != nil {
		if errors.Is(err, monitor.ErrDomainNotFound) {
			writeError(w, r, http.StatusNotFound, "DOMAIN_NOT_FOUND", nil)
			return
		}
		s.logger.ErrorContext(r.Context(), "get_monitoring_history_failed", "domain_id", domainID, "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		return
	}
	writeData(w, http.StatusOK, history)
}

func (s *Server) listIncidents(w http.ResponseWriter, r *http.Request) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	incidents, err := s.monitors.ListIncidents(r.Context(), status, page, pageSize)
	if err != nil {
		if errors.Is(err, monitor.ErrInvalidIncidentStatus) {
			writeError(w, r, http.StatusUnprocessableEntity, "INVALID_INCIDENT_STATUS", map[string]any{"allowed": []string{"open", "acknowledged", "closed"}})
			return
		}
		s.logger.ErrorContext(r.Context(), "list_incidents_failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		return
	}
	writeData(w, http.StatusOK, incidents)
}

func historyWindow(value string) (time.Duration, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "24h":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	case "30d":
		return 30 * 24 * time.Hour, true
	case "90d":
		return 90 * 24 * time.Hour, true
	default:
		return 0, false
	}
}
