package rdap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrRateLimited = errors.New("RDAP rate limited")
	ErrUnavailable = errors.New("RDAP unavailable")
	ErrNoBootstrap = errors.New("RDAP bootstrap service not found")
)

const DefaultBootstrapURL = "https://data.iana.org/rdap/dns.json"

type Config struct {
	BootstrapURL   string
	UserAgent      string
	MaxBytes       int64
	RequestTimeout time.Duration
	RetryMaxDelay  time.Duration
	MinInterval    time.Duration
}

type HTTPResult struct {
	URL          string
	Status       int
	Body         []byte
	ETag         string
	LastModified string
	Headers      map[string]string
	FetchedAt    time.Time
}

type Client struct {
	http        *http.Client
	config      Config
	now         func() time.Time
	mu          sync.Mutex
	lastRequest map[string]time.Time
}

func NewClient(client *http.Client, config Config) *Client {
	if client == nil {
		client = &http.Client{}
	}
	if config.BootstrapURL == "" {
		config.BootstrapURL = DefaultBootstrapURL
	}
	if config.UserAgent == "" {
		config.UserAgent = "DomainMonitor/phase5"
	}
	if config.MaxBytes <= 0 {
		config.MaxBytes = 1 << 20
	}
	if config.RequestTimeout <= 0 {
		config.RequestTimeout = 15 * time.Second
	}
	if config.RetryMaxDelay <= 0 {
		config.RetryMaxDelay = 2 * time.Second
	}
	if config.MinInterval <= 0 {
		config.MinInterval = 200 * time.Millisecond
	}
	return &Client{http: client, config: config, now: time.Now, lastRequest: map[string]time.Time{}}
}

func (c *Client) Bootstrap(ctx context.Context, etag, lastModified string) (HTTPResult, error) {
	return c.get(ctx, c.config.BootstrapURL, etag, lastModified)
}

func (c *Client) Domain(ctx context.Context, baseURL, domain string) (HTTPResult, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return HTTPResult{}, fmt.Errorf("%w: unsafe RDAP base URL", ErrUnavailable)
	}
	return c.get(ctx, baseURL+"/domain/"+url.PathEscape(domain), "", "")
}

func (c *Client) get(ctx context.Context, target, etag, lastModified string) (HTTPResult, error) {
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return HTTPResult{}, fmt.Errorf("%w: invalid HTTPS target", ErrUnavailable)
	}
	if err := c.waitForRateSlot(ctx, parsed.Hostname()); err != nil {
		return HTTPResult{}, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		requestCtx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
		request, requestErr := http.NewRequestWithContext(requestCtx, http.MethodGet, target, nil)
		if requestErr != nil {
			cancel()
			return HTTPResult{}, requestErr
		}
		request.Header.Set("Accept", "application/rdap+json, application/json")
		request.Header.Set("User-Agent", c.config.UserAgent)
		if etag != "" {
			request.Header.Set("If-None-Match", etag)
		}
		if lastModified != "" {
			request.Header.Set("If-Modified-Since", lastModified)
		}
		response, requestErr := c.http.Do(request)
		if requestErr != nil {
			cancel()
			if attempt == 0 {
				continue
			}
			return HTTPResult{}, fmt.Errorf("%w: %v", ErrUnavailable, requestErr)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, c.config.MaxBytes+1))
		_ = response.Body.Close()
		cancel()
		result := HTTPResult{URL: target, Status: response.StatusCode, Body: body, ETag: response.Header.Get("ETag"), LastModified: response.Header.Get("Last-Modified"), Headers: selectedHeaders(response.Header), FetchedAt: c.now().UTC()}
		if readErr != nil {
			return result, fmt.Errorf("%w: read response", ErrUnavailable)
		}
		if int64(len(body)) > c.config.MaxBytes {
			return result, fmt.Errorf("%w: response exceeds %d bytes", ErrUnavailable, c.config.MaxBytes)
		}
		if response.StatusCode == http.StatusTooManyRequests {
			if attempt == 0 {
				delay := retryAfter(response.Header.Get("Retry-After"), c.now(), c.config.RetryMaxDelay)
				if delay > 0 {
					if waitErr := wait(ctx, delay); waitErr == nil {
						continue
					}
				}
			}
			return result, ErrRateLimited
		}
		if response.StatusCode == http.StatusNotModified || (response.StatusCode >= 200 && response.StatusCode < 300) {
			return result, nil
		}
		if response.StatusCode >= 500 && attempt == 0 {
			if waitErr := wait(ctx, 100*time.Millisecond); waitErr == nil {
				continue
			}
		}
		return result, fmt.Errorf("%w: HTTP %d", ErrUnavailable, response.StatusCode)
	}
	return HTTPResult{}, ErrUnavailable
}

func (c *Client) waitForRateSlot(ctx context.Context, host string) error {
	c.mu.Lock()
	now := c.now()
	delay := c.config.MinInterval - now.Sub(c.lastRequest[host])
	if delay <= 0 {
		c.lastRequest[host] = now
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	if err := wait(ctx, delay); err != nil {
		return err
	}
	c.mu.Lock()
	c.lastRequest[host] = c.now()
	c.mu.Unlock()
	return nil
}

func selectedHeaders(headers http.Header) map[string]string {
	result := map[string]string{}
	for _, key := range []string{"Content-Type", "ETag", "Last-Modified", "Retry-After", "Cache-Control"} {
		if value := headers.Get(key); value != "" {
			result[key] = value
		}
	}
	return result
}

func retryAfter(raw string, now time.Time, maximum time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return min(time.Duration(seconds)*time.Second, maximum)
	}
	if parsed, err := http.ParseTime(raw); err == nil {
		return min(max(parsed.Sub(now), 0), maximum)
	}
	return 0
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Bootstrap struct {
	Services []json.RawMessage `json:"services"`
}

func ResolveBase(payload []byte, tld string) (string, error) {
	var registry Bootstrap
	if err := json.Unmarshal(payload, &registry); err != nil {
		return "", fmt.Errorf("%w: invalid bootstrap JSON", ErrNoBootstrap)
	}
	tld = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(tld), "."))
	for _, raw := range registry.Services {
		var service []json.RawMessage
		if json.Unmarshal(raw, &service) != nil || len(service) != 2 {
			continue
		}
		var tlds, urls []string
		if json.Unmarshal(service[0], &tlds) != nil || json.Unmarshal(service[1], &urls) != nil {
			continue
		}
		for _, candidateTLD := range tlds {
			if !strings.EqualFold(candidateTLD, tld) {
				continue
			}
			for _, candidateURL := range urls {
				parsed, err := url.Parse(candidateURL)
				if err == nil && parsed.Scheme == "https" && parsed.Hostname() != "" && parsed.User == nil {
					return strings.TrimRight(candidateURL, "/"), nil
				}
			}
		}
	}
	return "", fmt.Errorf("%w: TLD %s", ErrNoBootstrap, tld)
}

func payloadHash(payload []byte) []byte {
	digest := sha256.Sum256(payload)
	return digest[:]
}
