//go:build live

package dnscheck

import (
	"context"
	"testing"
	"time"
)

func TestLiveCloudflareDoH(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := (Checker{AttemptTimeout: 5 * time.Second}).Query(ctx, NewDoHResolver(nil, DefaultCloudflareEndpoint, 64<<10), "example.com", TypeA)
	if len(results) == 0 || results[len(results)-1].ErrorCode != "" || len(results[len(results)-1].Answers) == 0 {
		t.Fatalf("live DoH results = %#v", results)
	}
}
