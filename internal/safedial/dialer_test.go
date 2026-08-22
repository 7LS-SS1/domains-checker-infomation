package safedial

import (
	"context"
	"net/netip"
	"testing"

	"domainmonitor/internal/netcheck"
)

type staticResolver map[string][]netip.Addr

func (s staticResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	return s[host], nil
}

func TestDialerRejectsBlockedAddressBeforeDial(t *testing.T) {
	dialer := New(staticResolver{"internal.test": {netip.MustParseAddr("169.254.169.254")}}, Policy{})
	_, err := dialer.DialContext(context.Background(), "tcp", "internal.test:80")
	checkError, ok := netcheck.As(err)
	if !ok || checkError.Code != netcheck.SSRFBlockedAddress {
		t.Fatalf("DialContext() error = %v", err)
	}
}

func TestDialerRejectsEntireMixedAnswerSet(t *testing.T) {
	dialer := New(staticResolver{"mixed.test": {
		netip.MustParseAddr("93.184.216.34"),
		netip.MustParseAddr("127.0.0.1"),
	}}, Policy{})
	_, err := dialer.DialContext(context.Background(), "tcp", "mixed.test:443")
	checkError, ok := netcheck.As(err)
	if !ok || checkError.Code != netcheck.SSRFBlockedAddress {
		t.Fatalf("DialContext() error = %v", err)
	}
}

func TestDialerRejectsNonWebPort(t *testing.T) {
	dialer := New(staticResolver{}, Policy{})
	_, err := dialer.DialContext(context.Background(), "tcp", "example.com:22")
	checkError, ok := netcheck.As(err)
	if !ok || checkError.Code != netcheck.SSRFBlockedPort {
		t.Fatalf("DialContext() error = %v", err)
	}
}
