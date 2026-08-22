package dnscheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"domainmonitor/internal/netcheck"
	"domainmonitor/internal/retry"
	"github.com/miekg/dns"
)

type Checker struct {
	AttemptTimeout time.Duration
	Retry          retry.Policy
	Now            func() time.Time
}

func (c Checker) QueryAll(ctx context.Context, resolver Resolver, name string, maxConcurrency int) []Result {
	types := []QueryType{TypeA, TypeAAAA, TypeCNAME, TypeNS}
	if maxConcurrency < 1 {
		maxConcurrency = 2
	}
	if maxConcurrency > len(types) {
		maxConcurrency = len(types)
	}
	groups := make([][]Result, len(types))
	semaphore := make(chan struct{}, maxConcurrency)
	var waitGroup sync.WaitGroup
	for index, queryType := range types {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				groups[index] = c.Query(ctx, resolver, name, queryType)
				return
			}
			groups[index] = c.Query(ctx, resolver, name, queryType)
		}()
	}
	waitGroup.Wait()
	results := make([]Result, 0, len(types))
	for _, group := range groups {
		results = append(results, group...)
	}
	return results
}

func (c Checker) TraceCNAME(ctx context.Context, resolver Resolver, name string, maxDepth int) CNAMETrace {
	if maxDepth < 1 {
		maxDepth = 8
	}
	current := dns.Fqdn(strings.ToLower(strings.TrimSpace(name)))
	trace := CNAMETrace{Chain: []string{current}, Queries: []Result{}}
	seen := map[string]struct{}{current: {}}
	for depth := 0; depth < maxDepth; depth++ {
		results := c.Query(ctx, resolver, current, TypeCNAME)
		trace.Queries = append(trace.Queries, results...)
		if len(results) == 0 {
			return trace
		}
		latest := results[len(results)-1]
		if latest.ErrorCode != "" {
			trace.ErrorCode = latest.ErrorCode
			return trace
		}
		target := ""
		for _, answer := range latest.Answers {
			if answer.Type == TypeCNAME && strings.EqualFold(answer.Name, current) {
				target = dns.Fqdn(strings.ToLower(answer.Value))
				break
			}
		}
		if target == "" {
			return trace
		}
		trace.Chain = append(trace.Chain, target)
		if _, exists := seen[target]; exists {
			trace.Loop = true
			trace.ErrorCode = netcheck.DNSCNAMELoop
			return trace
		}
		seen[target] = struct{}{}
		current = target
	}
	trace.LimitReached = true
	trace.ErrorCode = netcheck.DNSCNAMELimit
	return trace
}

func (c Checker) Query(ctx context.Context, resolver Resolver, name string, queryType QueryType) []Result {
	if c.AttemptTimeout <= 0 {
		c.AttemptTimeout = 3 * time.Second
	}
	policy := c.Retry.Normalized()
	if c.Now == nil {
		c.Now = time.Now
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if _, valid := dns.IsDomainName(name); !valid || name == "." {
		return []Result{{
			Resolver: resolver.Type(), Endpoint: resolver.Endpoint(), QueryName: name, QueryType: queryType, Attempt: 1,
			ErrorCode: netcheck.NormalizationInvalid, ErrorMessage: "invalid DNS query name", CheckedAt: c.Now().UTC(),
		}}
	}
	dnsType, valid := wireType(queryType)
	if !valid {
		return []Result{{
			Resolver: resolver.Type(), Endpoint: resolver.Endpoint(), QueryName: name, QueryType: queryType, Attempt: 1,
			ErrorCode: netcheck.NormalizationInvalid, ErrorMessage: "unsupported DNS query type", CheckedAt: c.Now().UTC(),
		}}
	}
	name = dns.Fqdn(name)
	results := make([]Result, 0, policy.MaxAttempts)
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		message := new(dns.Msg)
		message.SetQuestion(name, dnsType)
		message.RecursionDesired = true
		message.SetEdns0(1232, false)

		attemptCtx, cancel := context.WithTimeout(ctx, c.AttemptTimeout)
		exchange, err := resolver.Exchange(attemptCtx, message)
		attemptContextError := attemptCtx.Err()
		cancel()
		result := Result{
			Resolver: resolver.Type(), Endpoint: resolver.Endpoint(), QueryName: name, QueryType: queryType,
			Attempt: attempt, Transport: exchange.Transport, Duration: exchange.Duration, Truncated: exchange.Truncated,
			Answers: []Answer{}, CheckedAt: c.Now().UTC(),
		}
		if err != nil {
			classified := classifyExchangeError(ctx, attemptContextError, err)
			result.ErrorCode = classified.Code
			result.ErrorMessage = classified.Error()
			results = append(results, result)
			if attempt >= policy.MaxAttempts || !classified.Retryable {
				break
			}
			if waitErr := policy.Wait(ctx, attempt); waitErr != nil {
				break
			}
			continue
		}
		if exchange.Message == nil {
			classified := netcheck.New(netcheck.DNSMalformedResponse, "dns", "query", false, errors.New("resolver returned no message"))
			result.ErrorCode, result.ErrorMessage = classified.Code, classified.Error()
			results = append(results, result)
			break
		}
		result.RCode = dns.RcodeToString[exchange.Message.Rcode]
		result.Authoritative = exchange.Message.Authoritative
		result.Truncated = result.Truncated || exchange.Message.Truncated
		result.Answers = parseAnswers(exchange.Message.Answer)
		if rcodeError := classifyRCode(exchange.Message.Rcode); rcodeError != nil {
			result.ErrorCode, result.ErrorMessage = rcodeError.Code, rcodeError.Error()
		}
		results = append(results, result)
		if rcodeError := classifyRCode(exchange.Message.Rcode); rcodeError == nil || !rcodeError.Retryable || attempt >= policy.MaxAttempts {
			break
		}
		if waitErr := policy.Wait(ctx, attempt); waitErr != nil {
			break
		}
	}
	return results
}

func wireType(queryType QueryType) (uint16, bool) {
	switch queryType {
	case TypeA:
		return dns.TypeA, true
	case TypeAAAA:
		return dns.TypeAAAA, true
	case TypeCNAME:
		return dns.TypeCNAME, true
	case TypeNS:
		return dns.TypeNS, true
	default:
		return 0, false
	}
}

func classifyExchangeError(parent context.Context, attemptErr error, err error) *netcheck.Error {
	if existing, ok := netcheck.As(err); ok {
		return existing
	}
	if errors.Is(parent.Err(), context.Canceled) {
		return netcheck.New(netcheck.RunCancelled, "dns", "query", false, context.Canceled)
	}
	if errors.Is(parent.Err(), context.DeadlineExceeded) {
		return netcheck.New(netcheck.RunDeadlineExceeded, "dns", "query", false, context.DeadlineExceeded)
	}
	if errors.Is(attemptErr, context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return netcheck.New(netcheck.DNSTimeout, "dns", "query", true, context.DeadlineExceeded)
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return netcheck.New(netcheck.DNSTimeout, "dns", "query", true, err)
	}
	return netcheck.New(netcheck.DNSNetworkError, "dns", "query", true, err)
}

func classifyRCode(rcode int) *netcheck.Error {
	switch rcode {
	case dns.RcodeSuccess:
		return nil
	case dns.RcodeNameError:
		return netcheck.New(netcheck.DNSNXDomain, "dns", "query", false, fmt.Errorf("DNS response code NXDOMAIN"))
	case dns.RcodeServerFailure:
		return netcheck.New(netcheck.DNSServFail, "dns", "query", true, fmt.Errorf("DNS response code SERVFAIL"))
	case dns.RcodeRefused:
		return netcheck.New(netcheck.DNSRefused, "dns", "query", false, fmt.Errorf("DNS response code REFUSED"))
	default:
		return netcheck.New(netcheck.DNSNetworkError, "dns", "query", false, fmt.Errorf("DNS response code %s", dns.RcodeToString[rcode]))
	}
}

func parseAnswers(records []dns.RR) []Answer {
	answers := make([]Answer, 0, len(records))
	for _, record := range records {
		header := record.Header()
		answer := Answer{Name: strings.ToLower(header.Name), TTL: header.Ttl}
		switch typed := record.(type) {
		case *dns.A:
			answer.Type, answer.Value = TypeA, typed.A.String()
		case *dns.AAAA:
			answer.Type, answer.Value = TypeAAAA, typed.AAAA.String()
		case *dns.CNAME:
			answer.Type, answer.Value = TypeCNAME, strings.ToLower(typed.Target)
		case *dns.NS:
			answer.Type, answer.Value = TypeNS, strings.ToLower(typed.Ns)
		default:
			continue
		}
		answers = append(answers, answer)
	}
	return answers
}
