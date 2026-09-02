package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"domainmonitor/internal/auth"
	"domainmonitor/internal/buildinfo"
	"domainmonitor/internal/config"
	"domainmonitor/internal/domain"
	"domainmonitor/internal/drive"
	"domainmonitor/internal/finance"
	"domainmonitor/internal/i18n"
	"domainmonitor/internal/monitor"
	"domainmonitor/internal/probe"
	"domainmonitor/internal/rdap"
	"domainmonitor/internal/recommendation"
	"domainmonitor/internal/report"
	"domainmonitor/internal/sheets"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	cfg          config.Config
	logger       *slog.Logger
	pool         *pgxpool.Pool
	redis        *redis.Client
	auth         *auth.Service
	domains      *domain.Service
	monitors     *monitor.Service
	probes       *probe.Service
	intelligence IntelligenceServices
	startTime    time.Time
}

type IntelligenceServices struct {
	RDAP            *rdap.Service
	Finance         *finance.Service
	Sheets          *sheets.Service
	Drive           *drive.Service
	Recommendations *recommendation.Service
	Reports         *report.Service
}

func NewServer(
	cfg config.Config,
	logger *slog.Logger,
	pool *pgxpool.Pool,
	redisClient *redis.Client,
	authService *auth.Service,
	domainService *domain.Service,
	monitorService *monitor.Service,
	probeService *probe.Service,
	intelligence IntelligenceServices,
) *Server {
	return &Server{
		cfg:          cfg,
		logger:       logger,
		pool:         pool,
		redis:        redisClient,
		auth:         authService,
		domains:      domainService,
		monitors:     monitorService,
		probes:       probeService,
		intelligence: intelligence,
		startTime:    time.Now().UTC(),
	}
}

func (s *Server) Handler() http.Handler {
	router := chi.NewRouter()
	defaultLocale := i18n.Parse(s.cfg.DefaultLocale, i18n.Thai)
	router.Use(requestIDMiddleware)
	router.Use(recoverMiddleware(s.logger))
	router.Use(accessLogMiddleware(s.logger))
	router.Use(securityHeadersMiddleware)
	router.Use(localeMiddleware(defaultLocale))
	router.Use(requestBodyLimitMiddleware(s.cfg.APIMaxBodyBytes, s.cfg.ExcelImportMaxBytes))

	router.Get("/", s.index)
	router.Get("/health", s.health)
	router.Get("/ready", s.ready)

	router.Route("/api/v1", func(api chi.Router) {
		api.Get("/meta/locales", s.locales)
		api.Post("/auth/login", s.login)
		api.Post("/probe-auth/register", s.registerProbe)
		api.Post("/probe-auth/token", s.probeToken)
		api.Post("/probe-agent/heartbeat", s.probeHeartbeat)
		api.Post("/probe-agent/jobs/claim", s.claimProbeJob)
		api.Post("/probe-agent/jobs/{jobID}/result", s.submitProbeResult)

		api.Group(func(protected chi.Router) {
			protected.Use(authenticationMiddleware(s.auth, s.cfg.CookieName))
			protected.Get("/auth/me", s.me)
			protected.With(requireCSRF(s.auth)).Post("/auth/logout", s.logout)

			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains", s.listDomains)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains", s.createDomain)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}", s.getDomain)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Patch("/domains/{domainID}", s.patchDomain)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains/{domainID}/archive", s.archiveDomain)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains/{domainID}/restore", s.restoreDomain)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains/{domainID}/check", s.createManualCheck)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains/{domainID}/isp-check", s.createISPCheck)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}/monitoring-runs", s.listMonitoringRuns)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}/monitoring-history", s.getMonitoringHistory)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}/rdap", s.getRDAP)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains/{domainID}/rdap-check", s.checkRDAP)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}/costs", s.listDomainCosts)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains/{domainID}/costs", s.addDomainCost)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}/overrides", s.listDomainOverrides)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Post("/domains/{domainID}/overrides", s.createDomainOverride)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Delete("/domains/{domainID}/overrides/{overrideID}", s.revokeDomainOverride)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}/provenance", s.getDomainProvenance)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}/recommendation", s.getDomainRecommendation)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains/{domainID}/recommendation", s.generateDomainRecommendation)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/domains/{domainID}/recommendations", s.getDomainRecommendation)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/domains/{domainID}/recommendations/recompute", s.generateDomainRecommendation)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/monitoring-runs/{runID}", s.getMonitoringRun)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/incidents", s.listIncidents)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/finance/summary", s.getFinanceSummary)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Post("/finance/exchange-rates", s.addExchangeRate)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/google-sheets/config", s.getSheetConfig)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Put("/google-sheets/config", s.saveSheetConfig)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/google-sheets/previews", s.previewSheetImport)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/google-sheets/excel/previews", s.previewExcelImport)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/google-sheets/imports", s.listSheetImports)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/google-sheets/imports/{importID}", s.getSheetImport)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Post("/google-sheets/imports/{importID}/apply", s.applySheetImport)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Post("/google-sheets/imports/{importID}/reject", s.rejectSheetImport)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Post("/google-drive/connect", s.connectGoogleDrive)
			protected.With(requireRoles("ADMIN")).Get("/google-drive/callback", s.googleDriveCallback)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/google-drive/connection", s.getGoogleDriveConnection)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/google-drive/files", s.listGoogleDriveFiles)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Delete("/google-drive/connection", s.disconnectGoogleDrive)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/recommendations", s.listRecommendations)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/recommendations/run", s.runRecommendations)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/reports/summary", s.getReportSummary)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/reports/dashboard", s.getReportDashboard)
			protected.With(requireRoles("ADMIN", "STAFF"), requireCSRF(s.auth)).Post("/reports", s.createReport)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/reports/{reportID}", s.getReport)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/reports/{reportID}/download", s.downloadReport)
			protected.With(requireRoles("ADMIN", "STAFF", "VIEWER")).Get("/probes", s.listProbes)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Post("/probes/registration-tokens", s.createProbeRegistrationToken)
			protected.With(requireRoles("ADMIN"), requireCSRF(s.auth)).Post("/probes/{probeID}/revoke", s.revokeProbe)
		})
	})

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", nil)
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", nil)
	})
	return router
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	locale := i18n.FromContext(r.Context(), i18n.Thai)
	writeData(w, http.StatusOK, map[string]any{
		"status":   "up",
		"service":  "domain-monitor-api",
		"version":  buildinfo.Version,
		"message":  i18n.Message("SERVICE_READY", locale),
		"messages": i18n.Messages("SERVICE_READY"),
		"locale":   locale,
		"endpoints": map[string]string{
			"health":    "/health",
			"readiness": "/ready",
			"api":       "/api/v1",
			"locales":   "/api/v1/meta/locales",
		},
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, map[string]any{
		"status":     "up",
		"service":    "domain-monitor-api",
		"version":    buildinfo.Version,
		"commit":     buildinfo.Commit,
		"started_at": s.startTime,
	})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() { results <- result{name: "postgres", err: s.pool.Ping(ctx)} }()
	go func() { results <- result{name: "redis", err: s.redis.Ping(ctx).Err()} }()
	checks := map[string]string{}
	ready := true
	for range 2 {
		result := <-results
		if result.err != nil {
			ready = false
			checks[result.name] = "unavailable"
			s.logger.WarnContext(ctx, "readiness_dependency_failed", "dependency", result.name, "error", result.err)
		} else {
			checks[result.name] = "ok"
		}
	}
	if !ready {
		writeError(w, r, http.StatusServiceUnavailable, "READINESS_FAILED", map[string]any{"checks": checks})
		return
	}
	writeData(w, http.StatusOK, map[string]any{"status": "ready", "checks": checks})
}

func (s *Server) locales(w http.ResponseWriter, _ *http.Request) {
	writeData(w, http.StatusOK, map[string]any{
		"default":   s.cfg.DefaultLocale,
		"supported": i18n.Supported(),
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	session, err := s.auth.Login(r.Context(), request.Email, request.Password, RequestID(r.Context()))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", nil)
			return
		}
		s.logger.ErrorContext(r.Context(), "login_failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
	})
	writeData(w, http.StatusOK, map[string]any{
		"user":       publicUser(session.User),
		"csrf_token": session.CSRFToken,
		"expires_at": session.ExpiresAt,
		"message":    i18n.Message("SESSION_CREATED", i18n.FromContext(r.Context(), i18n.Thai)),
		"messages":   i18n.Messages("SESSION_CREATED"),
	})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	roles := make([]string, 0, len(principal.Roles))
	for role := range principal.Roles {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	writeData(w, http.StatusOK, map[string]any{
		"id":           principal.UserID,
		"email":        principal.Email,
		"display_name": principal.DisplayName,
		"locale":       principal.Locale,
		"roles":        roles,
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := s.auth.Logout(r.Context(), principal, RequestID(r.Context())); err != nil {
		s.logger.ErrorContext(r.Context(), "logout_failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeData(w, http.StatusOK, map[string]any{
		"message":  i18n.Message("SESSION_REVOKED", i18n.FromContext(r.Context(), i18n.Thai)),
		"messages": i18n.Messages("SESSION_REVOKED"),
	})
}

type createDomainRequest struct {
	Domain              string  `json:"domain"`
	RegistrarID         *string `json:"registrar_id"`
	BusinessPriority    string  `json:"business_priority"`
	MonitoringEnabled   *bool   `json:"monitoring_enabled"`
	ExpectedContentMode string  `json:"expected_content_mode"`
	ExpirationAt        *string `json:"expiration_at"`
	Notes               string  `json:"notes"`
}

func (s *Server) createDomain(w http.ResponseWriter, r *http.Request) {
	var request createDomainRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	registrarID, ok := parseOptionalUUID(w, r, "registrar_id", request.RegistrarID)
	if !ok {
		return
	}
	expirationAt, ok := parseOptionalTime(w, r, "expiration_at", request.ExpirationAt)
	if !ok {
		return
	}
	monitoringEnabled := true
	if request.MonitoringEnabled != nil {
		monitoringEnabled = *request.MonitoringEnabled
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	created, err := s.domains.Create(r.Context(), domain.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, domain.CreateInput{
		Domain:              request.Domain,
		RegistrarID:         registrarID,
		BusinessPriority:    request.BusinessPriority,
		MonitoringEnabled:   monitoringEnabled,
		ExpectedContentMode: request.ExpectedContentMode,
		ExpirationAt:        expirationAt,
		Notes:               request.Notes,
	})
	if !s.handleDomainError(w, r, err) {
		return
	}
	w.Header().Set("Location", "/api/v1/domains/"+created.ID.String())
	w.Header().Set("ETag", etag(created.Version))
	writeData(w, http.StatusCreated, created)
}

func (s *Server) getDomain(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	item, err := s.domains.Get(r.Context(), id)
	if !s.handleDomainError(w, r, err) {
		return
	}
	w.Header().Set("ETag", etag(item.Version))
	writeData(w, http.StatusOK, item)
}

func (s *Server) listDomains(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	result, err := s.domains.List(r.Context(), domain.ListFilter{
		Query:           strings.TrimSpace(r.URL.Query().Get("query")),
		LifecycleStatus: strings.TrimSpace(r.URL.Query().Get("lifecycle_status")),
		SourceStatus:    strings.TrimSpace(r.URL.Query().Get("source_status")),
		Page:            page,
		PageSize:        pageSize,
		Sort:            strings.TrimSpace(r.URL.Query().Get("sort")),
		Direction:       strings.TrimSpace(r.URL.Query().Get("direction")),
	})
	if err != nil {
		var validationError *domain.ValidationError
		if errors.As(err, &validationError) {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", validationDetails(validationError))
			return
		}
		s.logger.ErrorContext(r.Context(), "list_domains_failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
		return
	}
	writeData(w, http.StatusOK, result)
}

type patchDomainRequest struct {
	Version             int64   `json:"version"`
	Domain              *string `json:"domain"`
	RegistrarID         *string `json:"registrar_id"`
	ClearRegistrar      bool    `json:"clear_registrar"`
	BusinessPriority    *string `json:"business_priority"`
	MonitoringEnabled   *bool   `json:"monitoring_enabled"`
	ExpectedContentMode *string `json:"expected_content_mode"`
	ExpirationAt        *string `json:"expiration_at"`
	ClearExpiration     bool    `json:"clear_expiration"`
	Notes               *string `json:"notes"`
	RenewalDecision     *string `json:"renewal_decision"`
	Reason              string  `json:"reason"`
}

func (s *Server) patchDomain(w http.ResponseWriter, r *http.Request) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	var request patchDomainRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	registrarID, ok := parseOptionalUUID(w, r, "registrar_id", request.RegistrarID)
	if !ok {
		return
	}
	expirationAt, ok := parseOptionalTime(w, r, "expiration_at", request.ExpirationAt)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	updated, err := s.domains.Patch(r.Context(), domain.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, id, domain.PatchInput{
		Version:             request.Version,
		Domain:              request.Domain,
		RegistrarID:         registrarID,
		ClearRegistrar:      request.ClearRegistrar,
		BusinessPriority:    request.BusinessPriority,
		MonitoringEnabled:   request.MonitoringEnabled,
		ExpectedContentMode: request.ExpectedContentMode,
		ExpirationAt:        expirationAt,
		ClearExpiration:     request.ClearExpiration,
		Notes:               request.Notes,
		RenewalDecision:     request.RenewalDecision,
		Reason:              request.Reason,
	})
	if !s.handleDomainError(w, r, err) {
		return
	}
	w.Header().Set("ETag", etag(updated.Version))
	writeData(w, http.StatusOK, updated)
}

type lifecycleRequest struct {
	Version int64  `json:"version"`
	Reason  string `json:"reason"`
}

func (s *Server) archiveDomain(w http.ResponseWriter, r *http.Request) {
	s.changeLifecycle(w, r, s.domains.Archive)
}

func (s *Server) restoreDomain(w http.ResponseWriter, r *http.Request) {
	s.changeLifecycle(w, r, s.domains.Restore)
}

func (s *Server) changeLifecycle(
	w http.ResponseWriter,
	r *http.Request,
	change func(context.Context, domain.Actor, uuid.UUID, int64, string) (domain.Domain, error),
) {
	id, ok := parseDomainID(w, r)
	if !ok {
		return
	}
	var request lifecycleRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	updated, err := change(r.Context(), domain.Actor{UserID: principal.UserID, RequestID: RequestID(r.Context())}, id, request.Version, request.Reason)
	if !s.handleDomainError(w, r, err) {
		return
	}
	w.Header().Set("ETag", etag(updated.Version))
	writeData(w, http.StatusOK, updated)
}

func (s *Server) handleDomainError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return true
	}
	var validationError *domain.ValidationError
	switch {
	case errors.As(err, &validationError):
		code := "VALIDATION_FAILED"
		if validationError.Field == "domain" {
			code = "INVALID_DOMAIN"
		}
		writeError(w, r, http.StatusUnprocessableEntity, code, validationDetails(validationError))
	case errors.Is(err, domain.ErrDuplicate):
		writeError(w, r, http.StatusConflict, "DOMAIN_ALREADY_EXISTS", nil)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "DOMAIN_NOT_FOUND", nil)
	case errors.Is(err, domain.ErrConflict):
		writeError(w, r, http.StatusConflict, "VERSION_CONFLICT", nil)
	default:
		s.logger.ErrorContext(r.Context(), "domain_operation_failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
	}
	return false
}

func parseDomainID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "domainID"))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", map[string]any{"field": "domain_id", "reason": "must be a UUID"})
		return uuid.Nil, false
	}
	return id, true
}

func parseOptionalUUID(w http.ResponseWriter, r *http.Request, field string, value *string) (*uuid.UUID, bool) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, true
	}
	parsed, err := uuid.Parse(*value)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", map[string]any{"field": field, "reason": "must be a UUID"})
		return nil, false
	}
	return &parsed, true
}

func parseOptionalTime(w http.ResponseWriter, r *http.Request, field string, value *string) (*time.Time, bool) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		if date, dateErr := time.Parse("2006-01-02", *value); dateErr == nil {
			parsed = date
		} else {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", map[string]any{"field": field, "reason": "must be RFC3339 or YYYY-MM-DD"})
			return nil, false
		}
	}
	parsed = parsed.UTC()
	return &parsed, true
}

func etag(version int64) string {
	return fmt.Sprintf("W/\"%d\"", version)
}

func publicUser(user auth.User) map[string]any {
	return map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"locale":       user.Locale,
		"roles":        user.Roles,
	}
}

func validationDetails(validationError *domain.ValidationError) map[string]any {
	reasonCode := validationError.ReasonCode
	if reasonCode == "" {
		reasonCode = "INVALID_VALUE"
	}
	return map[string]any{
		"field":       validationError.Field,
		"reason_code": reasonCode,
		"reason":      validationError.Reason,
	}
}
