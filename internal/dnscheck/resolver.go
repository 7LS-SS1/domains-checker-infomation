package dnscheck

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"domainmonitor/internal/netcheck"
	"github.com/miekg/dns"
)

const DefaultCloudflareEndpoint = "https://cloudflare-dns.com/dns-query"

type WireResolver struct {
	resolverType ResolverType
	endpoint     string
	timeout      time.Duration
}

func NewWireResolver(resolverType ResolverType, endpoint string, timeout time.Duration) *WireResolver {
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		endpoint = net.JoinHostPort(strings.Trim(endpoint, "[]"), "53")
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &WireResolver{resolverType: resolverType, endpoint: endpoint, timeout: timeout}
}

func NewSystemResolver(timeout time.Duration) (*WireResolver, error) {
	configuration, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil, fmt.Errorf("read /etc/resolv.conf: %w", err)
	}
	if len(configuration.Servers) == 0 {
		return nil, fmt.Errorf("/etc/resolv.conf contains no nameservers")
	}
	return NewWireResolver(ResolverLocalSystem, net.JoinHostPort(configuration.Servers[0], configuration.Port), timeout), nil
}

func (r *WireResolver) Type() ResolverType { return r.resolverType }
func (r *WireResolver) Endpoint() string   { return r.endpoint }

func (r *WireResolver) Exchange(ctx context.Context, message *dns.Msg) (ExchangeResult, error) {
	startedAt := time.Now()
	udpClient := &dns.Client{Net: "udp", Timeout: r.timeout}
	response, _, err := udpClient.ExchangeContext(ctx, message, r.endpoint)
	if err != nil {
		return ExchangeResult{Duration: time.Since(startedAt), Transport: "udp"}, err
	}
	if response == nil {
		return ExchangeResult{Duration: time.Since(startedAt), Transport: "udp"}, netcheck.New(netcheck.DNSMalformedResponse, "dns", "udp exchange", false, fmt.Errorf("empty DNS response"))
	}
	if !response.Truncated {
		return ExchangeResult{Message: response, Duration: time.Since(startedAt), Transport: "udp"}, nil
	}

	tcpClient := &dns.Client{Net: "tcp", Timeout: r.timeout}
	tcpResponse, _, err := tcpClient.ExchangeContext(ctx, message, r.endpoint)
	if err != nil {
		return ExchangeResult{Duration: time.Since(startedAt), Transport: "udp+tcp", Truncated: true}, err
	}
	if tcpResponse == nil {
		return ExchangeResult{Duration: time.Since(startedAt), Transport: "udp+tcp", Truncated: true}, netcheck.New(netcheck.DNSMalformedResponse, "dns", "tcp retry", false, fmt.Errorf("empty DNS response"))
	}
	return ExchangeResult{Message: tcpResponse, Duration: time.Since(startedAt), Transport: "udp+tcp", Truncated: true}, nil
}

type DoHResolver struct {
	client   *http.Client
	endpoint string
	maxBytes int64
}

func NewDoHResolver(client *http.Client, endpoint string, maxResponseBytes int64) *DoHResolver {
	if client == nil {
		client = &http.Client{
			Transport: http.DefaultTransport,
			Timeout:   10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	if endpoint == "" {
		endpoint = DefaultCloudflareEndpoint
	}
	if maxResponseBytes <= 0 {
		maxResponseBytes = 64 << 10
	}
	return &DoHResolver{client: client, endpoint: endpoint, maxBytes: maxResponseBytes}
}

func (r *DoHResolver) Type() ResolverType { return ResolverCloudflareDoH }
func (r *DoHResolver) Endpoint() string   { return r.endpoint }

func (r *DoHResolver) Exchange(ctx context.Context, message *dns.Msg) (ExchangeResult, error) {
	wire, err := message.Pack()
	if err != nil {
		return ExchangeResult{}, netcheck.New(netcheck.DNSMalformedResponse, "dns", "pack DoH query", false, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(wire))
	if err != nil {
		return ExchangeResult{}, netcheck.New(netcheck.DNSNetworkError, "dns", "create DoH request", false, err)
	}
	request.Header.Set("Accept", "application/dns-message")
	request.Header.Set("Content-Type", "application/dns-message")
	request.Header.Set("User-Agent", "DomainMonitor/phase3")

	startedAt := time.Now()
	response, err := r.client.Do(request)
	duration := time.Since(startedAt)
	if err != nil {
		if classified := netcheck.FromContext(ctx, "dns", "DoH request", netcheck.DNSTimeout, err); classified != nil {
			return ExchangeResult{Duration: duration, Transport: "doh"}, classified
		}
		return ExchangeResult{Duration: duration, Transport: "doh"}, netcheck.New(netcheck.DNSNetworkError, "dns", "DoH request", true, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		code := netcheck.DNSNetworkError
		retryable := response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
		if response.StatusCode == http.StatusGatewayTimeout {
			code = netcheck.DNSTimeout
		}
		return ExchangeResult{Duration: duration, Transport: "doh"}, netcheck.New(code, "dns", "DoH response", retryable, fmt.Errorf("unexpected HTTP status %d", response.StatusCode))
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/dns-message") {
		return ExchangeResult{Duration: duration, Transport: "doh"}, netcheck.New(netcheck.DNSMalformedResponse, "dns", "DoH response", false, fmt.Errorf("unexpected content type %q", response.Header.Get("Content-Type")))
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, r.maxBytes+1))
	if err != nil {
		return ExchangeResult{Duration: duration, Transport: "doh"}, netcheck.New(netcheck.DNSNetworkError, "dns", "read DoH response", true, err)
	}
	if int64(len(payload)) > r.maxBytes {
		return ExchangeResult{Duration: duration, Transport: "doh"}, netcheck.New(netcheck.DNSMalformedResponse, "dns", "read DoH response", false, fmt.Errorf("DNS response exceeds %d bytes", r.maxBytes))
	}
	decoded := new(dns.Msg)
	if err := decoded.Unpack(payload); err != nil {
		return ExchangeResult{Duration: duration, Transport: "doh"}, netcheck.New(netcheck.DNSMalformedResponse, "dns", "unpack DoH response", false, err)
	}
	if !decoded.Response || decoded.Id != message.Id {
		return ExchangeResult{Duration: duration, Transport: "doh"}, netcheck.New(netcheck.DNSMalformedResponse, "dns", "validate DoH response", false, fmt.Errorf("response ID or QR flag mismatch"))
	}
	return ExchangeResult{Message: decoded, Duration: duration, Transport: "doh", Truncated: decoded.Truncated}, nil
}
