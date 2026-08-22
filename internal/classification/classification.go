package classification

import (
	"strings"

	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/httpcheck"
	"domainmonitor/internal/netcheck"
)

type Decision struct {
	Availability    string   `json:"availability_status"`
	DNS             string   `json:"dns_status"`
	HTTP            string   `json:"http_status"`
	Redirect        string   `json:"redirect_status"`
	ISP             string   `json:"isp_status"`
	TLS             string   `json:"tls_status"`
	Content         string   `json:"content_status"`
	InitialHTTP     int      `json:"initial_http_status_code,omitempty"`
	FinalHTTP       int      `json:"final_http_status_code,omitempty"`
	FinalTarget     string   `json:"final_target_status,omitempty"`
	FailureStage    string   `json:"failure_stage,omitempty"`
	ErrorCode       string   `json:"error_code,omitempty"`
	ErrorMessage    string   `json:"error_message,omitempty"`
	Confidence      int16    `json:"confidence_score"`
	ConfidenceLevel string   `json:"confidence_level"`
	Reasons         []string `json:"reason_codes"`
}

func Classify(localDNS, alternateDNS []dnscheck.Result, origin httpcheck.OriginResult) Decision {
	comparison := dnscheck.Compare(localDNS, alternateDNS)
	decision := Decision{
		Availability: "UNKNOWN", DNS: classifyDNS(localDNS, comparison), HTTP: "UNKNOWN",
		Redirect: "NONE", ISP: "UNKNOWN", TLS: "UNKNOWN", Content: "UNKNOWN", Reasons: []string{},
	}
	attempt, found := effectiveAttempt(origin)
	if !found {
		decision.FailureStage, decision.ErrorCode, decision.ErrorMessage = "http", string(netcheck.HTTPTimeout), "HTTP checker returned no attempts"
		decision.Confidence = 20
		decision.Reasons = append(decision.Reasons, "HTTP_EVIDENCE_MISSING")
		decision.ConfidenceLevel = confidenceLevel(decision.Confidence)
		return decision
	}

	decision.InitialHTTP = attempt.InitialStatusCode
	decision.FinalHTTP = attempt.FinalStatusCode
	decision.Content = defaultString(attempt.Content.Status, "UNKNOWN")
	decision.Redirect = classifyRedirect(attempt)
	decision.HTTP = classifyHTTP(attempt)
	decision.TLS = classifyTLS(attempt)
	decision.Availability = classifyAvailability(attempt)
	if decision.Availability == "ACTIVE" {
		decision.FinalTarget = "ACTIVE"
	}
	decision.ErrorCode = string(attempt.ErrorCode)
	decision.ErrorMessage = attempt.ErrorMessage
	decision.FailureStage = failureStage(attempt.ErrorCode)
	decision.Reasons = append(decision.Reasons, evidenceReasons(decision, comparison)...)
	decision.Confidence = score(decision, localDNS, alternateDNS)
	decision.ConfidenceLevel = confidenceLevel(decision.Confidence)
	return decision
}

func effectiveAttempt(origin httpcheck.OriginResult) (httpcheck.Attempt, bool) {
	if origin.UsedHTTPFallback && len(origin.HTTP) > 0 {
		return origin.HTTP[len(origin.HTTP)-1], true
	}
	if len(origin.HTTPS) > 0 {
		return origin.HTTPS[len(origin.HTTPS)-1], true
	}
	return httpcheck.Attempt{}, false
}

func classifyDNS(results []dnscheck.Result, comparison dnscheck.Comparison) string {
	if comparison.Discrepancy {
		return "DISCREPANCY"
	}
	latest := map[dnscheck.QueryType]dnscheck.Result{}
	for _, result := range results {
		current, exists := latest[result.QueryType]
		if !exists || result.Attempt >= current.Attempt {
			latest[result.QueryType] = result
		}
	}
	for _, queryType := range []dnscheck.QueryType{dnscheck.TypeA, dnscheck.TypeAAAA, dnscheck.TypeCNAME} {
		result, exists := latest[queryType]
		if !exists || result.ErrorCode != "" {
			continue
		}
		if result.RCode == "NOERROR" {
			return "OK"
		}
	}
	for _, code := range []netcheck.Code{netcheck.DNSNXDomain, netcheck.DNSServFail, netcheck.DNSRefused, netcheck.DNSTimeout, netcheck.DNSNetworkError} {
		for _, result := range latest {
			if result.ErrorCode == code {
				return map[netcheck.Code]string{
					netcheck.DNSNXDomain: "NXDOMAIN", netcheck.DNSServFail: "SERVFAIL", netcheck.DNSRefused: "REFUSED",
					netcheck.DNSTimeout: "TIMEOUT", netcheck.DNSNetworkError: "NETWORK_ERROR",
				}[code]
			}
		}
	}
	return "UNKNOWN"
}

func classifyHTTP(attempt httpcheck.Attempt) string {
	if len(attempt.Redirects) > 0 && attempt.FinalStatusCode >= 200 && attempt.FinalStatusCode < 400 {
		return "REDIRECT"
	}
	if attempt.FinalStatusCode >= 200 && attempt.FinalStatusCode < 400 {
		return "OK"
	}
	if attempt.FinalStatusCode >= 400 && attempt.FinalStatusCode < 500 {
		return "CLIENT_ERROR"
	}
	if attempt.FinalStatusCode >= 500 {
		return "SERVER_ERROR"
	}
	switch attempt.ErrorCode {
	case netcheck.HTTPTimeout, netcheck.TCPTimeout:
		return "TIMEOUT"
	case netcheck.HTTPRedirectLoop, netcheck.HTTPRedirectLimit, netcheck.HTTPMalformedLocation, netcheck.HTTPHTTPSDowngrade:
		return "REDIRECT"
	case netcheck.TCPRefused, netcheck.TCPReset, netcheck.TCPNetworkError, netcheck.DNSTimeout,
		netcheck.DNSNetworkError, netcheck.DNSNXDomain, netcheck.DNSServFail, netcheck.DNSRefused,
		netcheck.TLSErrorExpired, netcheck.TLSErrorHostname, netcheck.TLSErrorUnknownAuthority, netcheck.TLSErrorHandshake:
		return "CONNECTION_ERROR"
	default:
		return "UNKNOWN"
	}
}

func classifyRedirect(attempt httpcheck.Attempt) string {
	switch attempt.ErrorCode {
	case netcheck.HTTPRedirectLoop:
		return "LOOP"
	case netcheck.HTTPRedirectLimit, netcheck.HTTPMalformedLocation:
		return "INVALID"
	case netcheck.HTTPHTTPSDowngrade:
		return "HTTPS_DOWNGRADE"
	}
	if len(attempt.Redirects) == 0 {
		return "NONE"
	}
	for _, hop := range attempt.Redirects {
		if hop.StatusCode == 301 || hop.StatusCode == 308 {
			return "PERMANENT"
		}
	}
	return "TEMPORARY"
}

func classifyTLS(attempt httpcheck.Attempt) string {
	if attempt.TLS != nil {
		return defaultString(attempt.TLS.Status, "UNKNOWN")
	}
	switch attempt.ErrorCode {
	case netcheck.TLSErrorExpired:
		return "EXPIRED"
	case netcheck.TLSErrorHostname:
		return "HOSTNAME_MISMATCH"
	case netcheck.TLSErrorUnknownAuthority:
		return "INVALID"
	case netcheck.TLSErrorHandshake:
		return "ERROR"
	}
	if strings.HasPrefix(attempt.RequestURL, "http://") {
		return "NOT_APPLICABLE"
	}
	return "UNKNOWN"
}

func classifyAvailability(attempt httpcheck.Attempt) string {
	if attempt.FinalStatusCode >= 200 && attempt.FinalStatusCode < 300 {
		switch attempt.Content.Status {
		case "VALID_HTML", "VALID_NON_HTML":
			return "ACTIVE"
		default:
			return "DEGRADED"
		}
	}
	if attempt.FinalStatusCode >= 400 && attempt.FinalStatusCode < 500 {
		return "DEGRADED"
	}
	if attempt.FinalStatusCode >= 500 {
		return "UNAVAILABLE"
	}
	switch attempt.ErrorCode {
	case netcheck.RunCancelled, netcheck.RunDeadlineExceeded:
		return "UNKNOWN"
	case netcheck.ContentEmpty, netcheck.ContentTooSmall, netcheck.ContentNotMeaningful,
		netcheck.ContentTooLarge, netcheck.ContentUnsupported:
		return "DEGRADED"
	default:
		if attempt.ErrorCode != "" {
			return "UNAVAILABLE"
		}
	}
	return "UNKNOWN"
}

func failureStage(code netcheck.Code) string {
	switch {
	case strings.HasPrefix(string(code), "DNS_"):
		return "dns"
	case strings.HasPrefix(string(code), "TCP_") || strings.HasPrefix(string(code), "SSRF_"):
		return "tcp"
	case strings.HasPrefix(string(code), "TLS_"):
		return "tls"
	case strings.HasPrefix(string(code), "HTTP_"):
		return "http"
	case strings.HasPrefix(string(code), "CONTENT_"):
		return "content"
	case code == netcheck.RunCancelled || code == netcheck.RunDeadlineExceeded:
		return "persistence"
	default:
		return ""
	}
}

func evidenceReasons(decision Decision, comparison dnscheck.Comparison) []string {
	reasons := append([]string{}, comparison.Reasons...)
	switch decision.Availability {
	case "ACTIVE":
		reasons = append(reasons, "HTTP_FINAL_SUCCESS", "CONTENT_VALID")
	case "DEGRADED":
		reasons = append(reasons, "PARTIAL_AVAILABILITY")
	case "UNAVAILABLE":
		reasons = append(reasons, "QUALIFYING_FAILURE")
	default:
		reasons = append(reasons, "EVIDENCE_INCOMPLETE")
	}
	if decision.Redirect == "PERMANENT" {
		reasons = append(reasons, "PERMANENT_REDIRECT")
	}
	if decision.ErrorCode != "" {
		reasons = append(reasons, decision.ErrorCode)
	}
	return unique(reasons)
}

func score(decision Decision, localDNS, alternateDNS []dnscheck.Result) int16 {
	value := 20
	switch decision.Availability {
	case "ACTIVE":
		value = 65
	case "DEGRADED":
		value = 50
	case "UNAVAILABLE":
		value = 35
	}
	if decision.DNS == "OK" {
		value += 10
	}
	if hasConclusiveDNS(alternateDNS) {
		value += 10
	}
	if repeatedFailure(localDNS, decision.ErrorCode) {
		value += 5
	}
	if decision.DNS == "DISCREPANCY" {
		value -= 10
	}
	if value < 0 {
		value = 0
	}
	if value > 99 {
		value = 99
	}
	return int16(value)
}

func hasConclusiveDNS(results []dnscheck.Result) bool {
	for _, result := range results {
		if result.ErrorCode == "" && result.RCode == "NOERROR" {
			return true
		}
	}
	return false
}

func repeatedFailure(results []dnscheck.Result, code string) bool {
	count := 0
	for _, result := range results {
		if string(result.ErrorCode) == code && code != "" {
			count++
		}
	}
	return count >= 2
}

func confidenceLevel(score int16) string {
	if score >= 80 {
		return "HIGH"
	}
	if score >= 50 {
		return "MEDIUM"
	}
	return "LOW"
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
