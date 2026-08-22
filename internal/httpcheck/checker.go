package httpcheck

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"domainmonitor/internal/netcheck"
	"domainmonitor/internal/safedial"
	"domainmonitor/internal/tlscheck"
	"golang.org/x/net/publicsuffix"
)

type Checker struct {
	transport http.RoundTripper
	config    Config
	now       func() time.Time
	closer    interface{ CloseIdleConnections() }
}

func NewSafe(resolver safedial.Resolver, dialPolicy safedial.Policy, config Config) *Checker {
	config = normalizeConfig(config)
	if dialPolicy.ConnectTimeout <= 0 {
		dialPolicy.ConnectTimeout = config.ConnectTimeout
	}
	dialer := safedial.New(resolver, dialPolicy)
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           200,
		MaxIdleConnsPerHost:    20,
		IdleConnTimeout:        90 * time.Second,
		TLSHandshakeTimeout:    config.TLSHandshakeTimeout,
		ResponseHeaderTimeout:  config.ResponseHeaderTimeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: int64(config.HeaderBytes),
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return New(transport, config)
}

func New(roundTripper http.RoundTripper, config Config) *Checker {
	config = normalizeConfig(config)
	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}
	checker := &Checker{transport: roundTripper, config: config, now: time.Now}
	if transport, ok := roundTripper.(interface{ CloseIdleConnections() }); ok {
		checker.closer = transport
	}
	return checker
}

func (c *Checker) CloseIdleConnections() {
	if c.closer != nil {
		c.closer.CloseIdleConnections()
	}
}

func (c *Checker) CheckURL(ctx context.Context, rawURL string, mode ContentMode) []Attempt {
	parsed, err := validateURL(rawURL)
	if err != nil {
		return []Attempt{{
			Attempt: 1, RequestURL: rawURL, Redirects: []RedirectHop{}, Headers: map[string]string{},
			Content: ContentEvidence{Status: "UNKNOWN"}, ErrorCode: netcheck.NormalizationInvalid,
			ErrorMessage: err.Error(), CheckedAt: c.now().UTC(),
		}}
	}
	if mode != ContentHTML && mode != ContentAny && mode != ContentStatusOnly {
		mode = ContentHTML
	}
	policy := c.config.Retry.Normalized()
	attempts := make([]Attempt, 0, policy.MaxAttempts)
	for attemptNumber := 1; attemptNumber <= policy.MaxAttempts; attemptNumber++ {
		attemptCtx, cancel := context.WithTimeout(ctx, c.config.ChainTimeout)
		attempt := c.checkAttempt(attemptCtx, parsed, mode, attemptNumber)
		cancel()
		attempts = append(attempts, attempt)
		if !attempt.Retryable || attemptNumber >= policy.MaxAttempts {
			break
		}
		delay := policy.Delay(attemptNumber)
		if attempt.RetryAfter > delay {
			delay = min(attempt.RetryAfter, policy.MaxDelay)
		}
		if err := wait(ctx, delay); err != nil {
			break
		}
	}
	return attempts
}

func (c *Checker) CheckDomain(ctx context.Context, hostname string, mode ContentMode) OriginResult {
	hostname = strings.TrimSpace(strings.TrimSuffix(hostname, "."))
	result := OriginResult{HTTPS: c.CheckURL(ctx, "https://"+hostname+"/", mode)}
	if len(result.HTTPS) == 0 || !shouldFallbackToHTTP(result.HTTPS[len(result.HTTPS)-1].ErrorCode) {
		return result
	}
	result.UsedHTTPFallback = true
	result.HTTP = c.CheckURL(ctx, "http://"+hostname+"/", mode)
	return result
}

func (c *Checker) checkAttempt(ctx context.Context, initial *url.URL, mode ContentMode, attemptNumber int) Attempt {
	startedAt := time.Now()
	attempt := Attempt{
		Attempt: attemptNumber, RequestURL: initial.String(), Redirects: []RedirectHop{}, Headers: map[string]string{},
		Content: ContentEvidence{Status: "UNKNOWN"}, CheckedAt: c.now().UTC(),
	}
	current := cloneURL(initial)
	seen := map[string]struct{}{canonicalURL(current): {}}
	for {
		response, timing, err := c.doRequest(ctx, current)
		accumulateTiming(&attempt.Timing, timing)
		if err != nil {
			classified := classifyRequestError(ctx, err)
			attempt.ErrorCode, attempt.ErrorMessage, attempt.Retryable = classified.Code, classified.Error(), classified.Retryable
			attempt.EffectiveURL = current.String()
			attempt.Timing.Total = time.Since(startedAt)
			return attempt
		}
		if attempt.InitialStatusCode == 0 {
			attempt.InitialStatusCode = response.StatusCode
		}
		if isRedirect(response.StatusCode) {
			if len(attempt.Redirects) >= c.config.MaxRedirects {
				drainAndClose(response.Body)
				attempt.ErrorCode, attempt.ErrorMessage = netcheck.HTTPRedirectLimit, fmt.Sprintf("redirect chain exceeds %d hops", c.config.MaxRedirects)
				attempt.EffectiveURL, attempt.Timing.Total = current.String(), time.Since(startedAt)
				return attempt
			}
			location := response.Header.Get("Location")
			target, resolveErr := resolveRedirect(current, location)
			hop := RedirectHop{Hop: len(attempt.Redirects), SourceURL: current.String(), StatusCode: response.StatusCode, Location: location, Duration: timing.Total}
			drainAndClose(response.Body)
			if resolveErr != nil {
				hop.ErrorCode = netcheck.HTTPMalformedLocation
				attempt.Redirects = append(attempt.Redirects, hop)
				attempt.ErrorCode, attempt.ErrorMessage = hop.ErrorCode, resolveErr.Error()
				attempt.EffectiveURL, attempt.Timing.Total = current.String(), time.Since(startedAt)
				return attempt
			}
			hop.ResolvedTarget = target.String()
			hop.CrossDomain = crossDomain(current.Hostname(), target.Hostname())
			hop.HTTPSDowngrade = current.Scheme == "https" && target.Scheme == "http"
			if hop.HTTPSDowngrade {
				hop.ErrorCode = netcheck.HTTPHTTPSDowngrade
				attempt.Redirects = append(attempt.Redirects, hop)
				attempt.ErrorCode, attempt.ErrorMessage = hop.ErrorCode, "HTTPS redirect target downgrades to HTTP"
				attempt.EffectiveURL, attempt.Timing.Total = current.String(), time.Since(startedAt)
				return attempt
			}
			canonicalTarget := canonicalURL(target)
			if _, exists := seen[canonicalTarget]; exists {
				hop.ErrorCode = netcheck.HTTPRedirectLoop
				attempt.Redirects = append(attempt.Redirects, hop)
				attempt.ErrorCode, attempt.ErrorMessage = hop.ErrorCode, "redirect loop detected"
				attempt.EffectiveURL, attempt.Timing.Total = target.String(), time.Since(startedAt)
				return attempt
			}
			attempt.Redirects = append(attempt.Redirects, hop)
			seen[canonicalTarget] = struct{}{}
			current = target
			continue
		}

		attempt.FinalStatusCode = response.StatusCode
		attempt.EffectiveURL = current.String()
		attempt.Protocol = response.Proto
		attempt.ServerHeader = response.Header.Get("Server")
		attempt.Headers = selectHeaders(response.Header, c.config.HeaderBytes)
		attempt.RetryAfter = parseRetryAfter(response.Header.Get("Retry-After"), c.now())
		if response.TLS != nil {
			tlsResult := tlscheck.FromConnectionState(*response.TLS, current.Hostname(), c.config.TLSExpiringSoon, c.now(), c.config.MaxCertificateSANs)
			attempt.TLS = &tlsResult
		}
		attempt.Content = readContent(response, mode, c.config)
		if statusError := classifyStatus(response.StatusCode); statusError != nil {
			attempt.ErrorCode, attempt.ErrorMessage, attempt.Retryable = statusError.Code, statusError.Error(), statusError.Retryable
		} else if attempt.Content.ErrorCode != "" {
			attempt.ErrorCode, attempt.ErrorMessage = attempt.Content.ErrorCode, attempt.Content.ErrorMessage
		}
		attempt.Timing.Total = time.Since(startedAt)
		return attempt
	}
}

func (c *Checker) doRequest(ctx context.Context, target *url.URL) (*http.Response, Timing, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, Timing{}, err
	}
	request.Header.Set("User-Agent", c.config.UserAgent)
	request.Header.Set("Accept", c.config.Accept)
	request.Header.Set("Accept-Language", c.config.AcceptLanguage)
	request.Header.Set("Connection", "keep-alive")
	recorder := newTraceRecorder()
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), recorder.trace()))
	startedAt := time.Now()
	response, err := c.transport.RoundTrip(request)
	if err != nil && response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	timing := recorder.timing()
	timing.Total = time.Since(startedAt)
	return response, timing, err
}

func normalizeConfig(config Config) Config {
	if config.MaxRedirects <= 0 {
		config.MaxRedirects = 10
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = 2 << 20
	}
	if config.ExcerptBytes <= 0 {
		config.ExcerptBytes = 32 << 10
	}
	if config.ExcerptBytes > 64<<10 {
		config.ExcerptBytes = 64 << 10
	}
	if config.HeaderBytes <= 0 {
		config.HeaderBytes = 16 << 10
	}
	if config.MinMeaningfulBytes <= 0 {
		config.MinMeaningfulBytes = 64
	}
	if config.ChainTimeout <= 0 {
		config.ChainTimeout = 25 * time.Second
	}
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 5 * time.Second
	}
	if config.TLSHandshakeTimeout <= 0 {
		config.TLSHandshakeTimeout = 7 * time.Second
	}
	if config.ResponseHeaderTimeout <= 0 {
		config.ResponseHeaderTimeout = 10 * time.Second
	}
	if config.TLSExpiringSoon <= 0 {
		config.TLSExpiringSoon = 30 * 24 * time.Hour
	}
	if config.MaxCertificateSANs <= 0 {
		config.MaxCertificateSANs = 100
	}
	if config.UserAgent == "" {
		config.UserAgent = "DomainMonitor/phase3"
	}
	if config.Accept == "" {
		config.Accept = "text/html,application/xhtml+xml,application/json;q=0.8,*/*;q=0.5"
	}
	if config.AcceptLanguage == "" {
		config.AcceptLanguage = "th,en;q=0.8"
	}
	config.Retry = config.Retry.Normalized()
	return config
}

func validateURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("URL scheme must be http or https")
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("URL hostname is required")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("credential-bearing URLs are not allowed")
	}
	parsed.Fragment = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed, nil
}

func resolveRedirect(source *url.URL, location string) (*url.URL, error) {
	if strings.TrimSpace(location) == "" {
		return nil, fmt.Errorf("redirect response has no Location header")
	}
	reference, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("parse Location: %w", err)
	}
	target := source.ResolveReference(reference)
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("redirect target scheme %q is not allowed", target.Scheme)
	}
	if target.Hostname() == "" || target.User != nil {
		return nil, fmt.Errorf("redirect target is invalid or contains credentials")
	}
	target.Fragment = ""
	if target.Path == "" {
		target.Path = "/"
	}
	return target, nil
}

func classifyRequestError(ctx context.Context, err error) *netcheck.Error {
	if existing, ok := netcheck.As(err); ok {
		return existing
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return netcheck.New(netcheck.RunCancelled, "http", "request", false, context.Canceled)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return netcheck.New(netcheck.HTTPTimeout, "http", "request", true, context.DeadlineExceeded)
	}
	var hostnameError x509.HostnameError
	if errors.As(err, &hostnameError) {
		return netcheck.New(netcheck.TLSErrorHostname, "tls", "HTTP handshake", false, err)
	}
	var invalidError x509.CertificateInvalidError
	if errors.As(err, &invalidError) && invalidError.Reason == x509.Expired {
		return netcheck.New(netcheck.TLSErrorExpired, "tls", "HTTP handshake", false, err)
	}
	var authorityError x509.UnknownAuthorityError
	if errors.As(err, &authorityError) {
		return netcheck.New(netcheck.TLSErrorUnknownAuthority, "tls", "HTTP handshake", false, err)
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return netcheck.New(netcheck.HTTPTimeout, "http", "request", true, err)
	}
	return netcheck.ClassifyTCP(ctx, "HTTP request", err)
}

func classifyStatus(status int) *netcheck.Error {
	switch {
	case status >= 400 && status <= 499:
		retryable := status == 408 || status == 425 || status == 429
		return netcheck.New(netcheck.HTTPClientError, "http", "response status", retryable, fmt.Errorf("HTTP status %d", status))
	case status >= 500:
		retryable := status == 502 || status == 503 || status == 504
		return netcheck.New(netcheck.HTTPServerError, "http", "response status", retryable, fmt.Errorf("HTTP status %d", status))
	default:
		return nil
	}
}

func shouldFallbackToHTTP(code netcheck.Code) bool {
	switch code {
	case netcheck.DNSNetworkError, netcheck.DNSTimeout, netcheck.TCPTimeout, netcheck.TCPRefused, netcheck.TCPReset, netcheck.TCPNetworkError,
		netcheck.TLSErrorExpired, netcheck.TLSErrorHostname, netcheck.TLSErrorUnknownAuthority, netcheck.TLSErrorHandshake:
		return true
	default:
		return false
	}
}

func isRedirect(status int) bool {
	return status == 301 || status == 302 || status == 303 || status == 307 || status == 308
}

func crossDomain(source, target string) bool {
	source = strings.ToLower(source)
	target = strings.ToLower(target)
	left, leftErr := publicsuffix.EffectiveTLDPlusOne(source)
	right, rightErr := publicsuffix.EffectiveTLDPlusOne(target)
	if leftErr != nil || rightErr != nil {
		return source != target
	}
	return left != right
}

func canonicalURL(value *url.URL) string {
	clone := cloneURL(value)
	clone.Scheme = strings.ToLower(clone.Scheme)
	clone.Host = strings.ToLower(clone.Host)
	if (clone.Scheme == "http" && clone.Port() == "80") || (clone.Scheme == "https" && clone.Port() == "443") {
		clone.Host = clone.Hostname()
	}
	clone.Fragment = ""
	if clone.Path == "" {
		clone.Path = "/"
	}
	return clone.String()
}

func cloneURL(value *url.URL) *url.URL {
	clone := *value
	return &clone
}

func drainAndClose(body io.ReadCloser) {
	_, _ = io.CopyN(io.Discard, body, 4<<10)
	_ = body.Close()
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if parsed, err := http.ParseTime(value); err == nil && parsed.After(now) {
		return parsed.Sub(now)
	}
	return 0
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func accumulateTiming(total *Timing, current Timing) {
	total.DNS += current.DNS
	total.Connect += current.Connect
	total.TLS += current.TLS
	total.TTFB += current.TTFB
	total.Reused = total.Reused || current.Reused
}

type traceRecorder struct {
	mutex                       sync.Mutex
	requestStarted, dnsStarted  time.Time
	connectStarted, tlsStarted  time.Time
	dns, connect, tlsTime, ttfb time.Duration
	reused                      bool
}

func newTraceRecorder() *traceRecorder {
	return &traceRecorder{requestStarted: time.Now()}
}

func (r *traceRecorder) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart:          func(httptrace.DNSStartInfo) { r.setTime(&r.dnsStarted) },
		DNSDone:           func(httptrace.DNSDoneInfo) { r.addDuration(&r.dns, &r.dnsStarted) },
		ConnectStart:      func(_, _ string) { r.setTime(&r.connectStarted) },
		ConnectDone:       func(_, _ string, _ error) { r.addDuration(&r.connect, &r.connectStarted) },
		TLSHandshakeStart: func() { r.setTime(&r.tlsStarted) },
		TLSHandshakeDone:  func(tls.ConnectionState, error) { r.addDuration(&r.tlsTime, &r.tlsStarted) },
		GotConn: func(info httptrace.GotConnInfo) {
			r.mutex.Lock()
			r.reused = info.Reused
			r.mutex.Unlock()
		},
		GotFirstResponseByte: func() {
			r.mutex.Lock()
			r.ttfb = time.Since(r.requestStarted)
			r.mutex.Unlock()
		},
	}
}

func (r *traceRecorder) setTime(destination *time.Time) {
	r.mutex.Lock()
	*destination = time.Now()
	r.mutex.Unlock()
}

func (r *traceRecorder) addDuration(destination *time.Duration, started *time.Time) {
	r.mutex.Lock()
	if !started.IsZero() {
		*destination += time.Since(*started)
	}
	r.mutex.Unlock()
}

func (r *traceRecorder) timing() Timing {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return Timing{DNS: r.dns, Connect: r.connect, TLS: r.tlsTime, TTFB: r.ttfb, Reused: r.reused}
}
