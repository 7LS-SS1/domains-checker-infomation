package protocolcheck

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"domainmonitor/internal/config"
	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/httpcheck"
	"domainmonitor/internal/retry"
	"domainmonitor/internal/safedial"
	"domainmonitor/internal/tlscheck"
)

type Suite struct {
	DNS           dnscheck.Checker
	LocalResolver dnscheck.Resolver
	DoHResolver   dnscheck.Resolver
	HTTP          *httpcheck.Checker
	TLS           tlscheck.Inspector
	doHTransport  *http.Transport
	dialPolicy    safedial.Policy
	httpConfig    httpcheck.Config
}

func New(cfg config.Config) (*Suite, error) {
	policy := retry.Policy{
		MaxAttempts: cfg.MonitorMaxAttempts,
		BaseDelay:   cfg.RetryBaseDelay,
		MaxDelay:    cfg.RetryMaxDelay,
	}
	var localResolver dnscheck.Resolver
	if cfg.LocalDNSAddress != "" {
		localResolver = dnscheck.NewWireResolver(dnscheck.ResolverLocalSystem, cfg.LocalDNSAddress, cfg.DNSAttemptTimeout)
	} else {
		systemResolver, err := dnscheck.NewSystemResolver(cfg.DNSAttemptTimeout)
		if err != nil {
			return nil, err
		}
		localResolver = systemResolver
	}
	doHTransport := &http.Transport{
		Proxy:                  nil,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           20,
		MaxIdleConnsPerHost:    10,
		IdleConnTimeout:        90 * time.Second,
		TLSHandshakeTimeout:    cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout:  cfg.HTTPHeaderTimeout,
		MaxResponseHeaderBytes: int64(cfg.HTTPHeaderBytes),
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
	}
	doHClient := &http.Client{
		Transport: doHTransport,
		Timeout:   cfg.DNSAttemptTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	dialPolicy := safedial.Policy{ConnectTimeout: cfg.HTTPConnectTimeout}
	safeDialer := safedial.New(net.DefaultResolver, dialPolicy)
	httpConfig := httpcheck.Config{
		MaxRedirects:          cfg.HTTPMaxRedirects,
		MaxBodyBytes:          cfg.HTTPMaxBodyBytes,
		ExcerptBytes:          cfg.HTTPExcerptBytes,
		HeaderBytes:           cfg.HTTPHeaderBytes,
		MinMeaningfulBytes:    cfg.HTTPMinBodyBytes,
		ConnectTimeout:        cfg.HTTPConnectTimeout,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.HTTPHeaderTimeout,
		ChainTimeout:          cfg.HTTPChainTimeout,
		TLSExpiringSoon:       cfg.TLSExpiringSoon,
		UserAgent:             cfg.MonitorUserAgent,
		Retry:                 policy,
	}
	httpChecker := httpcheck.NewSafe(net.DefaultResolver, dialPolicy, httpConfig)
	return &Suite{
		DNS:           dnscheck.Checker{AttemptTimeout: cfg.DNSAttemptTimeout, Retry: policy},
		LocalResolver: localResolver,
		DoHResolver:   dnscheck.NewDoHResolver(doHClient, cfg.DoHEndpoint, cfg.DoHMaxBytes),
		HTTP:          httpChecker,
		TLS: tlscheck.Inspector{
			DialContext: safeDialer.DialContext, HandshakeTimeout: cfg.TLSHandshakeTimeout,
			ExpiringSoon: cfg.TLSExpiringSoon,
		},
		doHTransport: doHTransport,
		dialPolicy:   dialPolicy,
		httpConfig:   httpConfig,
	}, nil
}

// NewDoHPinnedHTTP creates a short-lived HTTP checker which pins the original
// hostname to Cloudflare DoH A/AAAA answers while retaining Host and TLS SNI.
func (s *Suite) NewDoHPinnedHTTP(hostname string, results []dnscheck.Result) (*httpcheck.Checker, bool) {
	hostname = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(hostname), "."))
	addresses := make([]netip.Addr, 0)
	seen := map[netip.Addr]struct{}{}
	for _, result := range results {
		if result.ErrorCode != "" || result.RCode != "NOERROR" {
			continue
		}
		for _, answer := range result.Answers {
			if answer.Type != dnscheck.TypeA && answer.Type != dnscheck.TypeAAAA {
				continue
			}
			address, err := netip.ParseAddr(strings.TrimSpace(answer.Value))
			if err != nil {
				continue
			}
			address = address.Unmap()
			if _, exists := seen[address]; exists {
				continue
			}
			seen[address] = struct{}{}
			addresses = append(addresses, address)
		}
	}
	if hostname == "" || len(addresses) == 0 {
		return nil, false
	}
	resolver := safedial.PinnedResolver{
		Hosts:    map[string][]netip.Addr{hostname: addresses},
		Fallback: net.DefaultResolver,
	}
	return httpcheck.NewSafe(resolver, s.dialPolicy, s.httpConfig), true
}

func (s *Suite) CloseIdleConnections() {
	if s == nil {
		return
	}
	if s.HTTP != nil {
		s.HTTP.CloseIdleConnections()
	}
	if s.doHTransport != nil {
		s.doHTransport.CloseIdleConnections()
	}
}
