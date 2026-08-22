package safedial

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
)

// PinnedResolver returns an explicitly approved address set for selected hosts.
// Redirect hosts can still use the safe fallback resolver and are re-validated by Dialer.
type PinnedResolver struct {
	Hosts    map[string][]netip.Addr
	Fallback Resolver
}

func (r PinnedResolver) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	key := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if values, ok := r.Hosts[key]; ok {
		result := append([]netip.Addr(nil), values...)
		if len(result) == 0 {
			return nil, fmt.Errorf("pinned resolver has no addresses for %s", key)
		}
		return result, nil
	}
	if r.Fallback == nil {
		return nil, fmt.Errorf("host %s is not in the pinned resolver set", key)
	}
	return r.Fallback.LookupNetIP(ctx, network, host)
}
