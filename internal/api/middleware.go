package api

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"domainmonitor/internal/auth"
	"domainmonitor/internal/i18n"
	"github.com/google/uuid"
)

type requestIDContextKey struct{}

func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if _, err := uuid.Parse(requestID); err != nil {
			requestID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID)))
	})
}

func localeMiddleware(defaultLocale i18n.Locale) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := i18n.Parse(r.Header.Get("Accept-Language"), defaultLocale)
			w.Header().Set("Content-Language", string(locale))
			next.ServeHTTP(w, r.WithContext(i18n.WithContext(r.Context(), locale)))
		})
	}
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (w *responseRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func accessLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			recorder := &responseRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			logger.InfoContext(r.Context(), "http_request",
				"request_id", RequestID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

func recoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(r.Context(), "http_panic",
						"request_id", RequestID(r.Context()),
						"error", recovered,
						"stack", string(debug.Stack()),
					)
					writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func maxBodyMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return requestBodyLimitMiddleware(maxBytes, maxBytes)
}

func requestBodyLimitMiddleware(defaultMaxBytes, excelMaxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				limit := defaultMaxBytes
				if r.URL.Path == "/api/v1/google-sheets/excel/previews" {
					limit = excelMaxBytes + (1 << 20)
				}
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authenticationMiddleware(service *auth.Service, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", nil)
				return
			}
			principal, err := service.Authenticate(r.Context(), cookie.Value)
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", nil)
				return
			}
			ctx := auth.WithPrincipal(r.Context(), principal)
			if strings.TrimSpace(r.Header.Get("Accept-Language")) == "" {
				ctx = i18n.WithContext(ctx, i18n.Parse(principal.Locale, i18n.Thai))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func requireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", nil)
				return
			}
			if !principal.HasAnyRole(roles...) {
				writeError(w, r, http.StatusForbidden, "FORBIDDEN", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireCSRF(service *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", nil)
				return
			}
			if err := service.ValidateCSRF(principal, r.Header.Get("X-CSRF-Token")); err != nil {
				writeError(w, r, http.StatusForbidden, "CSRF_INVALID", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
