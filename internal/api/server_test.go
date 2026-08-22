package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"domainmonitor/internal/config"
)

func TestRootEndpointIsBilingualServiceDiscovery(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewServer(cfg, logger, nil, nil, nil, nil, nil, nil, IntelligenceServices{}).Handler()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept-Language", "en")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if response.Header().Get("Content-Language") != "en" ||
		!strings.Contains(body, `"locale":"en"`) ||
		!strings.Contains(body, `"messages":{"th":`) ||
		!strings.Contains(body, `"api":"/api/v1"`) {
		t.Fatalf("unexpected root response: %s", body)
	}
}

func TestLocalesEndpoint(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewServer(cfg, logger, nil, nil, nil, nil, nil, nil, IntelligenceServices{}).Handler()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/meta/locales", nil)
	request.Header.Set("Accept-Language", "en-US")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Language") != "en" {
		t.Fatalf("Content-Language = %q", response.Header().Get("Content-Language"))
	}
	if !strings.Contains(response.Body.String(), `"th"`) || !strings.Contains(response.Body.String(), `"en"`) {
		t.Fatalf("response does not contain both locales: %s", response.Body.String())
	}
}

func TestErrorResponseIsBilingual(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewServer(cfg, logger, nil, nil, nil, nil, nil, nil, IntelligenceServices{}).Handler()
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	request.Header.Set("Accept-Language", "th")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `"messages":{"th":`) || !strings.Contains(body, `"en":`) {
		t.Fatalf("response is not bilingual: %s", body)
	}
}
