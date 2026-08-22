package protocolcheck

import (
	"testing"

	"domainmonitor/internal/config"
)

func TestNewWiresConfiguredProtocolSuite(t *testing.T) {
	t.Setenv("LOCAL_DNS_ADDR", "127.0.0.1:5353")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	suite, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer suite.CloseIdleConnections()
	if suite.LocalResolver.Endpoint() != "127.0.0.1:5353" || suite.DoHResolver.Endpoint() != cfg.DoHEndpoint || suite.HTTP == nil {
		t.Fatalf("suite is incomplete: %#v", suite)
	}
}
