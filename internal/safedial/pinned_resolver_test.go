package safedial

import (
	"context"
	"net/netip"
	"testing"
)

func TestPinnedResolverNormalizesHost(t *testing.T) {
	resolver := PinnedResolver{Hosts: map[string][]netip.Addr{"example.com": {netip.MustParseAddr("93.184.216.34")}}}
	addresses, err := resolver.LookupNetIP(context.Background(), "ip", "EXAMPLE.COM.")
	if err != nil {
		t.Fatal(err)
	}
	if len(addresses) != 1 || addresses[0].String() != "93.184.216.34" {
		t.Fatalf("addresses = %v", addresses)
	}
}
