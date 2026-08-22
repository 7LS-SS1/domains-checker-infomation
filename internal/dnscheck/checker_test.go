package dnscheck

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"domainmonitor/internal/netcheck"
	"domainmonitor/internal/retry"
	"github.com/miekg/dns"
)

func TestWireResolverRCodesAndRetry(t *testing.T) {
	var servfailCalls atomic.Int32
	endpoint := startDNSServer(t, dns.HandlerFunc(func(writer dns.ResponseWriter, request *dns.Msg) {
		response := new(dns.Msg)
		response.SetReply(request)
		switch request.Question[0].Name {
		case "missing.test.":
			response.Rcode = dns.RcodeNameError
		case "failure.test.":
			servfailCalls.Add(1)
			response.Rcode = dns.RcodeServerFailure
		case "refused.test.":
			response.Rcode = dns.RcodeRefused
		default:
			response.Answer = []dns.RR{&dns.A{
				Hdr: dns.RR_Header{Name: request.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120},
				A:   net.ParseIP("93.184.216.34"),
			}}
		}
		_ = writer.WriteMsg(response)
	}))
	checker := Checker{AttemptTimeout: 200 * time.Millisecond, Retry: noWaitRetry(2)}
	resolver := NewWireResolver(ResolverLocalSystem, endpoint, 200*time.Millisecond)

	nxResults := checker.Query(context.Background(), resolver, "missing.test", TypeA)
	if len(nxResults) != 1 || nxResults[0].ErrorCode != netcheck.DNSNXDomain || nxResults[0].RCode != "NXDOMAIN" {
		t.Fatalf("NXDOMAIN results = %#v", nxResults)
	}
	servfailResults := checker.Query(context.Background(), resolver, "failure.test", TypeA)
	if len(servfailResults) != 2 || servfailResults[1].ErrorCode != netcheck.DNSServFail || servfailCalls.Load() != 2 {
		t.Fatalf("SERVFAIL results = %#v, calls = %d", servfailResults, servfailCalls.Load())
	}
	refusedResults := checker.Query(context.Background(), resolver, "refused.test", TypeA)
	if len(refusedResults) != 1 || refusedResults[0].ErrorCode != netcheck.DNSRefused {
		t.Fatalf("REFUSED results = %#v", refusedResults)
	}
}

func TestWireResolverRetriesTruncatedUDPOverTCP(t *testing.T) {
	endpoint := startDNSServer(t, dns.HandlerFunc(func(writer dns.ResponseWriter, request *dns.Msg) {
		response := new(dns.Msg)
		response.SetReply(request)
		if strings.HasPrefix(writer.LocalAddr().Network(), "udp") {
			response.Truncated = true
		} else {
			response.Answer = []dns.RR{&dns.A{
				Hdr: dns.RR_Header{Name: request.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP("93.184.216.34"),
			}}
		}
		_ = writer.WriteMsg(response)
	}))
	checker := Checker{AttemptTimeout: time.Second, Retry: noWaitRetry(1)}
	results := checker.Query(context.Background(), NewWireResolver(ResolverLocalSystem, endpoint, time.Second), "truncated.test", TypeA)
	if len(results) != 1 || results[0].ErrorCode != "" || !results[0].Truncated || results[0].Transport != "udp+tcp" || len(results[0].Answers) != 1 {
		t.Fatalf("results = %#v", results)
	}
}

func TestWireResolverTimeoutAndCancellation(t *testing.T) {
	endpoint := startDNSServer(t, dns.HandlerFunc(func(writer dns.ResponseWriter, request *dns.Msg) {
		time.Sleep(150 * time.Millisecond)
		response := new(dns.Msg)
		response.SetReply(request)
		_ = writer.WriteMsg(response)
	}))
	checker := Checker{AttemptTimeout: 20 * time.Millisecond, Retry: noWaitRetry(2)}
	results := checker.Query(context.Background(), NewWireResolver(ResolverLocalSystem, endpoint, 20*time.Millisecond), "timeout.test", TypeA)
	if len(results) != 2 || results[1].ErrorCode != netcheck.DNSTimeout {
		t.Fatalf("timeout results = %#v", results)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := checker.Query(ctx, NewWireResolver(ResolverLocalSystem, endpoint, time.Second), "cancelled.test", TypeA)
	if len(cancelled) != 1 || cancelled[0].ErrorCode != netcheck.RunCancelled {
		t.Fatalf("cancelled results = %#v", cancelled)
	}
}

func TestDoHResolverUsesPOSTWireformat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/dns-message" || request.Header.Get("Accept") != "application/dns-message" {
			t.Errorf("unexpected DoH request: method=%s headers=%v", request.Method, request.Header)
		}
		payload, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
			return
		}
		query := new(dns.Msg)
		if err := query.Unpack(payload); err != nil {
			t.Error(err)
			return
		}
		response := new(dns.Msg)
		response.SetReply(query)
		response.Answer = []dns.RR{&dns.A{
			Hdr: dns.RR_Header{Name: query.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
			A:   net.ParseIP("93.184.216.34"),
		}}
		wire, _ := response.Pack()
		writer.Header().Set("Content-Type", "application/dns-message")
		_, _ = writer.Write(wire)
	}))
	defer server.Close()

	resolver := NewDoHResolver(server.Client(), server.URL, 64<<10)
	results := (Checker{AttemptTimeout: time.Second, Retry: noWaitRetry(1)}).Query(context.Background(), resolver, "example.com", TypeA)
	if len(results) != 1 || results[0].ErrorCode != "" || results[0].Transport != "doh" || len(results[0].Answers) != 1 {
		t.Fatalf("DoH results = %#v", results)
	}
}

func TestQueryAllCapturesA_AAAA_CNAME_NSAndTTL(t *testing.T) {
	endpoint := startDNSServer(t, dns.HandlerFunc(func(writer dns.ResponseWriter, request *dns.Msg) {
		response := new(dns.Msg)
		response.SetReply(request)
		header := dns.RR_Header{Name: request.Question[0].Name, Rrtype: request.Question[0].Qtype, Class: dns.ClassINET, Ttl: 321}
		switch request.Question[0].Qtype {
		case dns.TypeA:
			response.Answer = []dns.RR{&dns.A{Hdr: header, A: net.ParseIP("93.184.216.34")}}
		case dns.TypeAAAA:
			response.Answer = []dns.RR{&dns.AAAA{Hdr: header, AAAA: net.ParseIP("2606:2800:220:1:248:1893:25c8:1946")}}
		case dns.TypeCNAME:
			response.Answer = []dns.RR{&dns.CNAME{Hdr: header, Target: "target.test."}}
		case dns.TypeNS:
			response.Answer = []dns.RR{&dns.NS{Hdr: header, Ns: "ns1.test."}}
		}
		_ = writer.WriteMsg(response)
	}))
	checker := Checker{AttemptTimeout: time.Second, Retry: noWaitRetry(1)}
	results := checker.QueryAll(context.Background(), NewWireResolver(ResolverLocalSystem, endpoint, time.Second), "records.test", 2)
	if len(results) != 4 {
		t.Fatalf("results = %#v", results)
	}
	for index, queryType := range []QueryType{TypeA, TypeAAAA, TypeCNAME, TypeNS} {
		if results[index].QueryType != queryType || len(results[index].Answers) != 1 || results[index].Answers[0].TTL != 321 {
			t.Fatalf("result %d = %#v", index, results[index])
		}
	}
}

func TestTraceCNAMEDetectsLoopAndDepthLimit(t *testing.T) {
	endpoint := startDNSServer(t, dns.HandlerFunc(func(writer dns.ResponseWriter, request *dns.Msg) {
		response := new(dns.Msg)
		response.SetReply(request)
		targets := map[string]string{"a.test.": "b.test.", "b.test.": "a.test.", "limit.test.": "next.test."}
		if target := targets[request.Question[0].Name]; target != "" {
			response.Answer = []dns.RR{&dns.CNAME{
				Hdr:    dns.RR_Header{Name: request.Question[0].Name, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60},
				Target: target,
			}}
		}
		_ = writer.WriteMsg(response)
	}))
	checker := Checker{AttemptTimeout: time.Second, Retry: noWaitRetry(1)}
	resolver := NewWireResolver(ResolverLocalSystem, endpoint, time.Second)
	loop := checker.TraceCNAME(context.Background(), resolver, "a.test", 8)
	if !loop.Loop || loop.ErrorCode != netcheck.DNSCNAMELoop || len(loop.Chain) != 3 {
		t.Fatalf("loop trace = %#v", loop)
	}
	limit := checker.TraceCNAME(context.Background(), resolver, "limit.test", 1)
	if !limit.LimitReached || limit.ErrorCode != netcheck.DNSCNAMELimit || len(limit.Chain) != 2 {
		t.Fatalf("limit trace = %#v", limit)
	}
}

func TestCompareIgnoresOrderAndTTLButDetectsAnswerDifference(t *testing.T) {
	local := []Result{{QueryType: TypeA, Attempt: 1, RCode: "NOERROR", Answers: []Answer{
		{Type: TypeA, Value: "93.184.216.34", TTL: 10},
		{Type: TypeA, Value: "93.184.216.35", TTL: 20},
	}}}
	alternate := []Result{{QueryType: TypeA, Attempt: 1, RCode: "NOERROR", Answers: []Answer{
		{Type: TypeA, Value: "93.184.216.35", TTL: 999},
		{Type: TypeA, Value: "93.184.216.34", TTL: 999},
	}}}
	if comparison := Compare(local, alternate); comparison.Discrepancy {
		t.Fatalf("equal sets marked discrepant: %#v", comparison)
	}
	alternate[0].Answers[1].Value = "93.184.216.36"
	comparison := Compare(local, alternate)
	if !comparison.Discrepancy || len(comparison.Reasons) != 1 || comparison.Reasons[0] != "DNS_ANSWER_DISCREPANCY" {
		t.Fatalf("comparison = %#v", comparison)
	}
}

func noWaitRetry(attempts int) retry.Policy {
	return retry.Policy{MaxAttempts: attempts, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, Jitter: func(time.Duration) time.Duration { return 0 }}
}

func startDNSServer(t *testing.T, handler dns.Handler) string {
	t.Helper()
	packetConnection, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := packetConnection.LocalAddr().(*net.UDPAddr).Port
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		_ = packetConnection.Close()
		t.Fatal(err)
	}
	udpServer := &dns.Server{PacketConn: packetConnection, Handler: handler}
	tcpServer := &dns.Server{Listener: listener, Handler: handler}
	go func() { _ = udpServer.ActivateAndServe() }()
	go func() { _ = tcpServer.ActivateAndServe() }()
	t.Cleanup(func() {
		_ = udpServer.Shutdown()
		_ = tcpServer.Shutdown()
	})
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}
