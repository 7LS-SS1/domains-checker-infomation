package httpcheck

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"domainmonitor/internal/netcheck"
	"domainmonitor/internal/retry"
	"domainmonitor/internal/safedial"
)

func TestCheckerMeaningfulHTMLAndBoundedEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "Phase2-Test/1.0" || request.Header.Get("Accept-Language") != "th,en;q=0.8" {
			t.Errorf("unexpected request headers: %v", request.Header)
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		writer.Header().Set("Server", "fixture")
		writer.Header().Set("X-Request-Id", strings.Repeat("r", 200))
		writer.Header().Set("Set-Cookie", "secret=session")
		_, _ = io.WriteString(writer, `<html><head><title> Phase 2 Home </title></head><body>Domain monitoring content token=super-secret-value is meaningful and available.</body></html>`)
	}))
	defer server.Close()
	config := testHTTPConfig()
	config.UserAgent = "Phase2-Test/1.0"
	config.HeaderBytes = 96
	checker := New(server.Client().Transport, config)
	defer checker.CloseIdleConnections()

	attempt := onlyAttempt(t, checker.CheckURL(context.Background(), server.URL, ContentHTML))
	if attempt.ErrorCode != "" || attempt.FinalStatusCode != http.StatusOK || attempt.Content.Status != "VALID_HTML" || attempt.Content.Title != "Phase 2 Home" || !attempt.Content.HashComplete {
		t.Fatalf("attempt = %#v", attempt)
	}
	if len(attempt.Content.Excerpt) > config.ExcerptBytes || strings.Contains(string(attempt.Content.Excerpt), "super-secret-value") || !strings.Contains(string(attempt.Content.Excerpt), "[REDACTED]") {
		t.Fatalf("excerpt was not bounded/redacted: %q", attempt.Content.Excerpt)
	}
	if _, exists := attempt.Headers["Set-Cookie"]; exists {
		t.Fatalf("sensitive header persisted: %#v", attempt.Headers)
	}
	headerBytes := 0
	for key, value := range attempt.Headers {
		headerBytes += len(key) + len(value) + 2
	}
	if headerBytes > config.HeaderBytes {
		t.Fatalf("selected headers = %d bytes, limit = %d", headerBytes, config.HeaderBytes)
	}
}

func TestCheckerRedirectChains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/one":
			http.Redirect(writer, request, "/final", http.StatusMovedPermanently)
		case "/two":
			http.Redirect(writer, request, "/middle", http.StatusMovedPermanently)
		case "/middle":
			http.Redirect(writer, request, "/final", http.StatusPermanentRedirect)
		case "/final":
			writer.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(writer, meaningfulHTML("final"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	checker := New(server.Client().Transport, testHTTPConfig())
	defer checker.CloseIdleConnections()

	one := onlyAttempt(t, checker.CheckURL(context.Background(), server.URL+"/one", ContentHTML))
	if one.InitialStatusCode != 301 || one.FinalStatusCode != 200 || len(one.Redirects) != 1 || one.Redirects[0].ResolvedTarget != server.URL+"/final" {
		t.Fatalf("301 -> 200 attempt = %#v", one)
	}
	two := onlyAttempt(t, checker.CheckURL(context.Background(), server.URL+"/two", ContentHTML))
	if two.InitialStatusCode != 301 || two.FinalStatusCode != 200 || len(two.Redirects) != 2 || two.Redirects[1].StatusCode != 308 {
		t.Fatalf("301 -> 308 -> 200 attempt = %#v", two)
	}
}

func TestCheckerDetectsRedirectLoopLimitAndMalformedLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/loop-a":
			http.Redirect(writer, request, "/loop-b", http.StatusFound)
		case "/loop-b":
			http.Redirect(writer, request, "/loop-a", http.StatusMovedPermanently)
		case "/limit-a":
			http.Redirect(writer, request, "/limit-b", http.StatusMovedPermanently)
		case "/limit-b":
			http.Redirect(writer, request, "/final", http.StatusMovedPermanently)
		case "/malformed":
			writer.Header().Set("Location", "http://[::1")
			writer.WriteHeader(http.StatusMovedPermanently)
		default:
			_, _ = io.WriteString(writer, meaningfulHTML("final"))
		}
	}))
	defer server.Close()
	checker := New(server.Client().Transport, testHTTPConfig())
	defer checker.CloseIdleConnections()

	loop := onlyAttempt(t, checker.CheckURL(context.Background(), server.URL+"/loop-a", ContentHTML))
	if loop.ErrorCode != netcheck.HTTPRedirectLoop || len(loop.Redirects) != 2 || loop.Redirects[1].ErrorCode != netcheck.HTTPRedirectLoop {
		t.Fatalf("loop attempt = %#v", loop)
	}
	limitConfig := testHTTPConfig()
	limitConfig.MaxRedirects = 1
	limitChecker := New(server.Client().Transport, limitConfig)
	limit := onlyAttempt(t, limitChecker.CheckURL(context.Background(), server.URL+"/limit-a", ContentHTML))
	if limit.ErrorCode != netcheck.HTTPRedirectLimit || len(limit.Redirects) != 1 {
		t.Fatalf("limit attempt = %#v", limit)
	}
	malformed := onlyAttempt(t, checker.CheckURL(context.Background(), server.URL+"/malformed", ContentHTML))
	if malformed.ErrorCode != netcheck.HTTPMalformedLocation || len(malformed.Redirects) != 1 {
		t.Fatalf("malformed attempt = %#v", malformed)
	}
}

func TestCheckerRejectsHTTPSDowngrade(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", "http://example.com/insecure")
		writer.WriteHeader(http.StatusMovedPermanently)
	}))
	defer server.Close()
	checker := New(server.Client().Transport, testHTTPConfig())
	defer checker.CloseIdleConnections()
	attempt := onlyAttempt(t, checker.CheckURL(context.Background(), server.URL, ContentHTML))
	if attempt.ErrorCode != netcheck.HTTPHTTPSDowngrade || len(attempt.Redirects) != 1 || !attempt.Redirects[0].HTTPSDowngrade {
		t.Fatalf("attempt = %#v", attempt)
	}
}

func TestCheckerClassifiesHTTPStatusesWithoutOpaqueFallback(t *testing.T) {
	tests := []struct {
		status int
		code   netcheck.Code
	}{
		{status: 403, code: netcheck.HTTPClientError},
		{status: 451, code: netcheck.HTTPClientError},
		{status: 500, code: netcheck.HTTPServerError},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("status-%d", test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "text/html")
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, meaningfulHTML("error evidence"))
			}))
			defer server.Close()
			attempt := onlyAttempt(t, New(server.Client().Transport, testHTTPConfig()).CheckURL(context.Background(), server.URL, ContentHTML))
			if attempt.FinalStatusCode != test.status || attempt.ErrorCode != test.code {
				t.Fatalf("attempt = %#v", attempt)
			}
		})
	}
}

func TestCheckerContentValidationMatrix(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		mode        ContentMode
		minimum     int
		wantStatus  string
		wantCode    netcheck.Code
	}{
		{name: "empty", contentType: "text/html", body: "", mode: ContentHTML, minimum: 8, wantStatus: "EMPTY", wantCode: netcheck.ContentEmpty},
		{name: "small", contentType: "text/html", body: "<p>x", mode: ContentHTML, minimum: 8, wantStatus: "TOO_SMALL", wantCode: netcheck.ContentTooSmall},
		{name: "blank-html", contentType: "text/html", body: "<html><body>       </body></html>", mode: ContentHTML, minimum: 8, wantStatus: "NOT_MEANINGFUL", wantCode: netcheck.ContentNotMeaningful},
		{name: "unsupported", contentType: "application/json", body: `{"status":"healthy and sufficiently descriptive"}`, mode: ContentHTML, minimum: 8, wantStatus: "UNSUPPORTED_CONTENT", wantCode: netcheck.ContentUnsupported},
		{name: "any-json", contentType: "application/json", body: `{"status":"healthy and sufficiently descriptive"}`, mode: ContentAny, minimum: 8, wantStatus: "VALID_NON_HTML"},
		{name: "status-only-empty", contentType: "application/octet-stream", body: "", mode: ContentStatusOnly, minimum: 8, wantStatus: "EMPTY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", test.contentType)
				_, _ = io.WriteString(writer, test.body)
			}))
			defer server.Close()
			config := testHTTPConfig()
			config.MinMeaningfulBytes = test.minimum
			attempt := onlyAttempt(t, New(server.Client().Transport, config).CheckURL(context.Background(), server.URL, test.mode))
			if attempt.Content.Status != test.wantStatus || attempt.ErrorCode != test.wantCode {
				t.Fatalf("attempt = %#v", attempt)
			}
		})
	}
}

func TestCheckerBoundsOversizedChunkedAndCompressedBodies(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "chunked", handler: func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "text/html")
			flusher := writer.(http.Flusher)
			for range 8 {
				_, _ = io.WriteString(writer, strings.Repeat("meaningful", 8))
				flusher.Flush()
			}
		}},
		{name: "compressed", handler: func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "text/html")
			writer.Header().Set("Content-Encoding", "gzip")
			compressor := gzip.NewWriter(writer)
			_, _ = io.WriteString(compressor, strings.Repeat("meaningful", 100))
			_ = compressor.Close()
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			config := testHTTPConfig()
			config.MaxBodyBytes = 128
			config.ExcerptBytes = 32
			attempt := onlyAttempt(t, New(server.Client().Transport, config).CheckURL(context.Background(), server.URL, ContentHTML))
			if attempt.Content.Status != "OVERSIZED_TRUNCATED" || attempt.ErrorCode != netcheck.ContentTooLarge || attempt.Content.HashComplete || attempt.Content.BodySize != 129 || len(attempt.Content.Excerpt) > 32 {
				t.Fatalf("attempt = %#v", attempt)
			}
		})
	}
}

func TestCheckerTimeoutCancellationAndConnectionRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = io.WriteString(writer, meaningfulHTML("late"))
	}))
	defer server.Close()
	config := testHTTPConfig()
	config.ChainTimeout = 20 * time.Millisecond
	timedOut := onlyAttempt(t, New(server.Client().Transport, config).CheckURL(context.Background(), server.URL, ContentHTML))
	if timedOut.ErrorCode != netcheck.HTTPTimeout {
		t.Fatalf("timeout attempt = %#v", timedOut)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := onlyAttempt(t, New(server.Client().Transport, testHTTPConfig()).CheckURL(ctx, server.URL, ContentHTML))
	if cancelled.ErrorCode != netcheck.RunCancelled {
		t.Fatalf("cancelled attempt = %#v", cancelled)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	refused := onlyAttempt(t, New(http.DefaultTransport.(*http.Transport).Clone(), testHTTPConfig()).CheckURL(context.Background(), "http://"+address, ContentHTML))
	if refused.ErrorCode != netcheck.TCPRefused {
		t.Fatalf("refused attempt = %#v", refused)
	}
}

func TestCheckerRetriesSelectedTransientStatus(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		if calls.Add(1) == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
		}
		_, _ = io.WriteString(writer, meaningfulHTML("retry"))
	}))
	defer server.Close()
	config := testHTTPConfig()
	config.Retry.MaxAttempts = 2
	attempts := New(server.Client().Transport, config).CheckURL(context.Background(), server.URL, ContentHTML)
	if len(attempts) != 2 || attempts[0].ErrorCode != netcheck.HTTPServerError || attempts[1].ErrorCode != "" || calls.Load() != 2 {
		t.Fatalf("attempts = %#v calls=%d", attempts, calls.Load())
	}
}

func TestCheckerReusesConnectionsAndCapturesTLS(t *testing.T) {
	var newConnections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(writer, meaningfulHTML("reusable connection"))
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConnections.Add(1)
		}
	}
	server.StartTLS()
	defer server.Close()
	checker := New(server.Client().Transport, testHTTPConfig())
	defer checker.CloseIdleConnections()
	first := onlyAttempt(t, checker.CheckURL(context.Background(), server.URL, ContentHTML))
	second := onlyAttempt(t, checker.CheckURL(context.Background(), server.URL, ContentHTML))
	if first.TLS == nil || first.TLS.Status != "VALID" || second.TLS == nil || !second.Timing.Reused || newConnections.Load() != 1 {
		t.Fatalf("first=%#v second=%#v connections=%d", first, second, newConnections.Load())
	}
}

func TestSafeCheckerPinsAllowedFixtureAndBlocksLoopbackByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.Host, "fixture.test:") {
			t.Errorf("Host header was not preserved: %q", request.Host)
		}
		writer.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(writer, meaningfulHTML("safe fixture"))
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	_, rawPort, _ := net.SplitHostPort(parsed.Host)
	portValue, _ := net.LookupPort("tcp", rawPort)
	resolver := fixtureResolver{"fixture.test": {netip.MustParseAddr("127.0.0.1")}}
	allowedPolicy := safedial.Policy{
		AllowedPorts:    map[uint16]struct{}{uint16(portValue): {}},
		AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
	}
	allowed := onlyAttempt(t, NewSafe(resolver, allowedPolicy, testHTTPConfig()).CheckURL(context.Background(), "http://fixture.test:"+rawPort, ContentHTML))
	if allowed.ErrorCode != "" || allowed.FinalStatusCode != 200 {
		t.Fatalf("allowed attempt = %#v", allowed)
	}
	blocked := onlyAttempt(t, NewSafe(resolver, safedial.Policy{AllowedPorts: map[uint16]struct{}{uint16(portValue): {}}}, testHTTPConfig()).CheckURL(context.Background(), "http://fixture.test:"+rawPort, ContentHTML))
	if blocked.ErrorCode != netcheck.SSRFBlockedAddress {
		t.Fatalf("blocked attempt = %#v", blocked)
	}
}

func TestCheckDomainFallsBackOnlyForConnectionStageFailure(t *testing.T) {
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme == "https" {
			return nil, netcheck.New(netcheck.TLSErrorHandshake, "tls", "fixture", false, fmt.Errorf("handshake failed"))
		}
		return fixtureResponse(request, 200, meaningfulHTML("HTTP fallback")), nil
	})
	result := New(transport, testHTTPConfig()).CheckDomain(context.Background(), "example.test", ContentHTML)
	if !result.UsedHTTPFallback || len(result.HTTP) != 1 || result.HTTP[0].FinalStatusCode != 200 {
		t.Fatalf("result = %#v", result)
	}

	noFallbackTransport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return fixtureResponse(request, 451, meaningfulHTML("legal restriction")), nil
	})
	noFallback := New(noFallbackTransport, testHTTPConfig()).CheckDomain(context.Background(), "example.test", ContentHTML)
	if noFallback.UsedHTTPFallback || len(noFallback.HTTP) != 0 || noFallback.HTTPS[0].FinalStatusCode != 451 {
		t.Fatalf("noFallback = %#v", noFallback)
	}
}

func TestCrossDomainUsesRegistrableDomain(t *testing.T) {
	if crossDomain("www.example.com", "cdn.example.com") {
		t.Fatal("subdomains of the same registrable domain marked cross-domain")
	}
	if !crossDomain("example.com", "example.net") {
		t.Fatal("different registrable domains were not marked cross-domain")
	}
}

type fixtureResolver map[string][]netip.Addr

func (f fixtureResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	return f[host], nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func fixtureResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": {"text/html"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       request,
	}
}

func testHTTPConfig() Config {
	return Config{
		MaxRedirects: 10, MaxBodyBytes: 4 << 10, ExcerptBytes: 512, HeaderBytes: 2 << 10,
		MinMeaningfulBytes: 16, ChainTimeout: time.Second,
		Retry: retry.Policy{MaxAttempts: 1, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, Jitter: func(time.Duration) time.Duration { return 0 }},
	}
}

func onlyAttempt(t *testing.T, attempts []Attempt) Attempt {
	t.Helper()
	if len(attempts) != 1 {
		t.Fatalf("got %d attempts, want 1: %#v", len(attempts), attempts)
	}
	return attempts[0]
}

func meaningfulHTML(label string) string {
	return "<html><head><title>" + label + "</title></head><body>Meaningful domain monitoring content for " + label + " with enough visible text.</body></html>"
}
