package safedial

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"domainmonitor/internal/netcheck"
)

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type Policy struct {
	AllowedPorts    map[uint16]struct{}
	AllowedPrefixes []netip.Prefix
	ConnectTimeout  time.Duration
}

type Dialer struct {
	resolver Resolver
	policy   Policy
}

func New(resolver Resolver, policy Policy) *Dialer {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	if len(policy.AllowedPorts) == 0 {
		policy.AllowedPorts = map[uint16]struct{}{80: {}, 443: {}}
	}
	if policy.ConnectTimeout <= 0 {
		policy.ConnectTimeout = 5 * time.Second
	}
	return &Dialer{resolver: resolver, policy: policy}
}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, netcheck.New(netcheck.SSRFBlockedAddress, "tcp", "dial", false, fmt.Errorf("network %q is not allowed", network))
	}
	host, rawPort, err := net.SplitHostPort(address)
	if err != nil {
		return nil, netcheck.New(netcheck.SSRFBlockedPort, "tcp", "dial", false, err)
	}
	portValue, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return nil, netcheck.New(netcheck.SSRFBlockedPort, "tcp", "dial", false, err)
	}
	port := uint16(portValue)
	if _, allowed := d.policy.AllowedPorts[port]; !allowed {
		return nil, netcheck.New(netcheck.SSRFBlockedPort, "tcp", "dial", false, fmt.Errorf("port %d is not allowed", port))
	}

	addresses, err := d.resolve(ctx, host)
	if err != nil {
		if checkError, ok := netcheck.As(err); ok {
			return nil, checkError
		}
		return nil, netcheck.New(netcheck.DNSNetworkError, "dns", "safe resolve", true, err)
	}
	if len(addresses) == 0 {
		return nil, netcheck.New(netcheck.DNSNetworkError, "dns", "safe resolve", false, errors.New("resolver returned no addresses"))
	}
	for _, candidate := range addresses {
		if err := d.validate(candidate); err != nil {
			return nil, err
		}
	}

	networkDialer := net.Dialer{Timeout: d.policy.ConnectTimeout, KeepAlive: 30 * time.Second}
	var lastError error
	for _, candidate := range addresses {
		connection, dialErr := networkDialer.DialContext(ctx, network, net.JoinHostPort(candidate.String(), rawPort))
		if dialErr == nil {
			return connection, nil
		}
		lastError = dialErr
	}
	return nil, netcheck.ClassifyTCP(ctx, "dial "+host, lastError)
}

func (d *Dialer) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if parsed, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{parsed.Unmap()}, nil
	}
	addresses, err := d.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	unique := make([]netip.Addr, 0, len(addresses))
	seen := map[netip.Addr]struct{}{}
	for _, address := range addresses {
		address = address.Unmap()
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		unique = append(unique, address)
	}
	return unique, nil
}

func (d *Dialer) validate(address netip.Addr) error {
	address = address.Unmap()
	for _, allowed := range d.policy.AllowedPrefixes {
		if allowed.Contains(address) {
			return nil
		}
	}
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return blocked(address)
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(address) {
			return blocked(address)
		}
	}
	return nil
}

func blocked(address netip.Addr) error {
	return netcheck.New(netcheck.SSRFBlockedAddress, "tcp", "validate target", false, fmt.Errorf("address %s is blocked by policy", address))
}

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
	"64:ff9b:1::/48", "100::/64", "2001:db8::/32",
)

func mustPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}
