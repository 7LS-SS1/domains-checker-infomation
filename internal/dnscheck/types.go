package dnscheck

import (
	"context"
	"time"

	"domainmonitor/internal/netcheck"
	"github.com/miekg/dns"
)

type QueryType string

const (
	TypeA     QueryType = "A"
	TypeAAAA  QueryType = "AAAA"
	TypeCNAME QueryType = "CNAME"
	TypeNS    QueryType = "NS"
)

type ResolverType string

const (
	ResolverLocalSystem   ResolverType = "LOCAL_SYSTEM"
	ResolverCloudflareDoH ResolverType = "CLOUDFLARE_DOH"
	ResolverRemoteSystem  ResolverType = "REMOTE_SYSTEM"
)

type Answer struct {
	Name  string    `json:"name"`
	Type  QueryType `json:"type"`
	Value string    `json:"value"`
	TTL   uint32    `json:"ttl_seconds"`
}

type Result struct {
	Resolver      ResolverType  `json:"resolver"`
	Endpoint      string        `json:"resolver_endpoint"`
	QueryName     string        `json:"query_name"`
	QueryType     QueryType     `json:"query_type"`
	Attempt       int           `json:"attempt"`
	RCode         string        `json:"rcode,omitempty"`
	Authoritative bool          `json:"authoritative"`
	Truncated     bool          `json:"truncated"`
	Transport     string        `json:"transport"`
	Duration      time.Duration `json:"duration"`
	Answers       []Answer      `json:"answers"`
	ErrorCode     netcheck.Code `json:"error_code,omitempty"`
	ErrorMessage  string        `json:"error_message,omitempty"`
	CheckedAt     time.Time     `json:"checked_at"`
}

func (r Result) Err() error {
	if r.ErrorCode == "" {
		return nil
	}
	return netcheck.New(r.ErrorCode, "dns", "query", retryableCode(r.ErrorCode), errorMessage(r.ErrorMessage))
}

type ExchangeResult struct {
	Message   *dns.Msg
	Duration  time.Duration
	Transport string
	Truncated bool
}

type Resolver interface {
	Type() ResolverType
	Endpoint() string
	Exchange(ctx context.Context, message *dns.Msg) (ExchangeResult, error)
}

type Comparison struct {
	Discrepancy   bool     `json:"dns_discrepancy"`
	NSDiscrepancy bool     `json:"ns_discrepancy"`
	Reasons       []string `json:"reasons"`
}

type CNAMETrace struct {
	Chain        []string      `json:"chain"`
	Queries      []Result      `json:"queries"`
	Loop         bool          `json:"loop"`
	LimitReached bool          `json:"limit_reached"`
	ErrorCode    netcheck.Code `json:"error_code,omitempty"`
}

type resultError string

func (e resultError) Error() string { return string(e) }

func errorMessage(message string) error {
	if message == "" {
		return nil
	}
	return resultError(message)
}

func retryableCode(code netcheck.Code) bool {
	return code == netcheck.DNSTimeout || code == netcheck.DNSNetworkError || code == netcheck.DNSServFail
}
