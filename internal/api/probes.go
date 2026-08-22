package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"domainmonitor/internal/auth"
	"domainmonitor/internal/i18n"
	"domainmonitor/internal/probe"
	"domainmonitor/internal/probeprotocol"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (s *Server) registerProbe(w http.ResponseWriter, r *http.Request) {
	var request probeprotocol.RegisterRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := s.probes.Register(r.Context(), request)
	if !s.handleProbeError(w, r, err) {
		return
	}
	writeData(w, http.StatusCreated, map[string]any{
		"registration": result, "message": i18n.Message("PROBE_REGISTERED", i18n.FromContext(r.Context(), i18n.Thai)),
		"messages": i18n.Messages("PROBE_REGISTERED"),
	})
}

func (s *Server) probeToken(w http.ResponseWriter, r *http.Request) {
	var request probeprotocol.TokenRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.ChallengeID == uuid.Nil && strings.TrimSpace(request.Signature) == "" {
		challenge, err := s.probes.CreateChallenge(r.Context(), request.ProbeID)
		if !s.handleProbeError(w, r, err) {
			return
		}
		writeData(w, http.StatusOK, map[string]any{"challenge": challenge})
		return
	}
	if request.ChallengeID == uuid.Nil || strings.TrimSpace(request.Signature) == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "PROBE_INVALID_REQUEST", nil)
		return
	}
	token, err := s.probes.IssueToken(r.Context(), request)
	if !s.handleProbeError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, token)
}

func (s *Server) probeHeartbeat(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticateProbe(w, r)
	if !ok {
		return
	}
	var request probeprotocol.HeartbeatRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := s.probes.Heartbeat(r.Context(), principal, request)
	if !s.handleProbeError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, result)
}

func (s *Server) claimProbeJob(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticateProbe(w, r)
	if !ok {
		return
	}
	var request probeprotocol.ClaimRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.MaxWaitMS < 0 || request.MaxWaitMS > 10000 {
		writeError(w, r, http.StatusUnprocessableEntity, "PROBE_INVALID_REQUEST", nil)
		return
	}
	deadline := time.Now().Add(time.Duration(request.MaxWaitMS) * time.Millisecond)
	for {
		job, err := s.probes.Claim(r.Context(), principal)
		if err == nil {
			writeData(w, http.StatusOK, map[string]any{"job": job})
			return
		}
		if !errors.Is(err, probe.ErrNoJob) {
			s.handleProbeError(w, r, err)
			return
		}
		if request.MaxWaitMS == 0 || time.Now().After(deadline) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-r.Context().Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Server) submitProbeResult(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.authenticateProbe(w, r)
	if !ok {
		return
	}
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "PROBE_INVALID_REQUEST", nil)
		return
	}
	var envelope probeprotocol.ResultEnvelope
	if !decodeJSON(w, r, &envelope) {
		return
	}
	result, err := s.probes.SubmitResult(r.Context(), principal, jobID, envelope)
	if !s.handleProbeError(w, r, err) {
		return
	}
	writeData(w, http.StatusAccepted, result)
}

type createProbeTokenRequest struct {
	Name        string `json:"name"`
	RegionCode  string `json:"region_code"`
	CountryCode string `json:"country_code"`
	NetworkName string `json:"network_name"`
	TTLSeconds  int    `json:"ttl_seconds"`
}

func (s *Server) createProbeRegistrationToken(w http.ResponseWriter, r *http.Request) {
	var request createProbeTokenRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := s.probes.CreateRegistrationToken(r.Context(), probe.RegistrationSpec{
		Name: request.Name, Region: request.RegionCode, Country: request.CountryCode, Network: request.NetworkName,
		TTL: time.Duration(request.TTLSeconds) * time.Second, CreatedBy: principal.UserID, RequestID: RequestID(r.Context()),
	})
	if !s.handleProbeError(w, r, err) {
		return
	}
	writeData(w, http.StatusCreated, map[string]any{
		"registration_token": result, "message": i18n.Message("PROBE_TOKEN_CREATED", i18n.FromContext(r.Context(), i18n.Thai)),
		"messages": i18n.Messages("PROBE_TOKEN_CREATED"),
	})
}

func (s *Server) listProbes(w http.ResponseWriter, r *http.Request) {
	items, err := s.probes.ListNodes(r.Context())
	if !s.handleProbeError(w, r, err) {
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": items})
}

type revokeProbeRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) revokeProbe(w http.ResponseWriter, r *http.Request) {
	probeID, err := uuid.Parse(chi.URLParam(r, "probeID"))
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "PROBE_INVALID_REQUEST", nil)
		return
	}
	var request revokeProbeRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if !s.handleProbeError(w, r, s.probes.Revoke(r.Context(), probeID, principal.UserID, RequestID(r.Context()), request.Reason)) {
		return
	}
	writeData(w, http.StatusOK, map[string]any{
		"message": i18n.Message("PROBE_REVOKED", i18n.FromContext(r.Context(), i18n.Thai)), "messages": i18n.Messages("PROBE_REVOKED"),
	})
}

func (s *Server) authenticateProbe(w http.ResponseWriter, r *http.Request) (probe.Principal, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writeError(w, r, http.StatusUnauthorized, "PROBE_UNAUTHORIZED", nil)
		return probe.Principal{}, false
	}
	principal, err := s.probes.Authenticate(r.Context(), parts[1])
	if !s.handleProbeError(w, r, err) {
		return probe.Principal{}, false
	}
	return principal, true
}

func (s *Server) handleProbeError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return true
	}
	switch {
	case errors.Is(err, probe.ErrUnauthorized):
		writeError(w, r, http.StatusUnauthorized, "PROBE_UNAUTHORIZED", nil)
	case errors.Is(err, probe.ErrSignature):
		writeError(w, r, http.StatusUnauthorized, "PROBE_SIGNATURE_INVALID", nil)
	case errors.Is(err, probe.ErrForbidden):
		writeError(w, r, http.StatusForbidden, "PROBE_FORBIDDEN", nil)
	case errors.Is(err, probe.ErrExpired):
		writeError(w, r, http.StatusGone, "PROBE_EXPIRED", nil)
	case errors.Is(err, probe.ErrReplay), errors.Is(err, probe.ErrConflict):
		writeError(w, r, http.StatusConflict, "PROBE_REPLAY", nil)
	case errors.Is(err, probe.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "PROBE_NOT_FOUND", nil)
	case errors.Is(err, probe.ErrPayloadTooLarge):
		writeError(w, r, http.StatusRequestEntityTooLarge, "PROBE_PAYLOAD_TOO_LARGE", nil)
	case errors.Is(err, probe.ErrClockSkew):
		writeError(w, r, http.StatusUnprocessableEntity, "PROBE_CLOCK_SKEW", nil)
	case errors.Is(err, probe.ErrInvalidRequest):
		writeError(w, r, http.StatusUnprocessableEntity, "PROBE_INVALID_REQUEST", nil)
	default:
		s.logger.ErrorContext(r.Context(), "probe_operation_failed", "request_id", RequestID(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
	}
	return false
}
