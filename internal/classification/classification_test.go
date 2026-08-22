package classification

import (
	"testing"
	"time"

	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/httpcheck"
	"domainmonitor/internal/netcheck"
)

func TestClassifyPermanentRedirectWithValidContentAsActive(t *testing.T) {
	now := time.Now().UTC()
	dnsResults := []dnscheck.Result{{
		Resolver: dnscheck.ResolverLocalSystem, QueryType: dnscheck.TypeA, RCode: "NOERROR",
		Answers: []dnscheck.Answer{{Type: dnscheck.TypeA, Value: "203.0.113.10"}}, CheckedAt: now,
	}}
	alternate := []dnscheck.Result{{
		Resolver: dnscheck.ResolverCloudflareDoH, QueryType: dnscheck.TypeA, RCode: "NOERROR",
		Answers: []dnscheck.Answer{{Type: dnscheck.TypeA, Value: "203.0.113.10"}}, CheckedAt: now,
	}}
	origin := httpcheck.OriginResult{HTTPS: []httpcheck.Attempt{{
		Attempt: 1, RequestURL: "https://example.com/", EffectiveURL: "https://www.example.com/",
		InitialStatusCode: 301, FinalStatusCode: 200,
		Redirects: []httpcheck.RedirectHop{{Hop: 0, StatusCode: 301}},
		Content:   httpcheck.ContentEvidence{Status: "VALID_HTML", BodySize: 128}, CheckedAt: now,
	}}}
	decision := Classify(dnsResults, alternate, origin)
	if decision.Availability != "ACTIVE" || decision.HTTP != "REDIRECT" || decision.Redirect != "PERMANENT" {
		t.Fatalf("decision = %#v", decision)
	}
	if decision.ConfidenceLevel != "HIGH" {
		t.Fatalf("confidence = %d/%s", decision.Confidence, decision.ConfidenceLevel)
	}
}

func TestClassifyLocalFailureWithAlternateDNSSuccessAsEvidenceNotISPClaim(t *testing.T) {
	now := time.Now().UTC()
	local := []dnscheck.Result{{
		Resolver: dnscheck.ResolverLocalSystem, QueryType: dnscheck.TypeA, Attempt: 3,
		ErrorCode: netcheck.DNSTimeout, CheckedAt: now,
	}}
	alternate := []dnscheck.Result{{
		Resolver: dnscheck.ResolverCloudflareDoH, QueryType: dnscheck.TypeA, RCode: "NOERROR",
		Answers: []dnscheck.Answer{{Type: dnscheck.TypeA, Value: "203.0.113.10"}}, CheckedAt: now,
	}}
	origin := httpcheck.OriginResult{HTTPS: []httpcheck.Attempt{{
		Attempt: 3, RequestURL: "https://example.com/", Content: httpcheck.ContentEvidence{Status: "UNKNOWN"},
		ErrorCode: netcheck.DNSTimeout, ErrorMessage: "DNS timeout", CheckedAt: now,
	}}}
	decision := Classify(local, alternate, origin)
	if decision.Availability != "UNAVAILABLE" || decision.DNS != "DISCREPANCY" {
		t.Fatalf("decision = %#v", decision)
	}
	if decision.ISP != "UNKNOWN" {
		t.Fatalf("ISP status = %q; Phase 3 must not infer ISP blocking", decision.ISP)
	}
}
