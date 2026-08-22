package dnscheck

import (
	"net/netip"
	"sort"
	"strings"
)

func Compare(local, alternate []Result) Comparison {
	localLatest := latestByType(local)
	alternateLatest := latestByType(alternate)
	comparison := Comparison{Reasons: []string{}}
	for _, queryType := range []QueryType{TypeA, TypeAAAA, TypeCNAME, TypeNS} {
		left, leftExists := localLatest[queryType]
		right, rightExists := alternateLatest[queryType]
		if !leftExists || !rightExists {
			continue
		}
		if left.RCode != right.RCode || left.ErrorCode != right.ErrorCode {
			if queryType == TypeNS {
				comparison.NSDiscrepancy = true
				addReason(&comparison, "DNS_NS_DISCREPANCY")
			} else {
				comparison.Discrepancy = true
				addReason(&comparison, "DNS_RCODE_DISCREPANCY")
			}
			continue
		}
		if !equalAnswerSets(left.Answers, right.Answers, queryType) {
			if queryType == TypeNS {
				comparison.NSDiscrepancy = true
				addReason(&comparison, "DNS_NS_DISCREPANCY")
			} else {
				comparison.Discrepancy = true
				addReason(&comparison, "DNS_ANSWER_DISCREPANCY")
			}
		}
	}
	return comparison
}

func latestByType(results []Result) map[QueryType]Result {
	latest := make(map[QueryType]Result)
	for _, result := range results {
		current, exists := latest[result.QueryType]
		if !exists || result.Attempt >= current.Attempt {
			latest[result.QueryType] = result
		}
	}
	return latest
}

func equalAnswerSets(left, right []Answer, queryType QueryType) bool {
	leftValues := canonicalValues(left, queryType)
	rightValues := canonicalValues(right, queryType)
	if len(leftValues) != len(rightValues) {
		return false
	}
	for index := range leftValues {
		if leftValues[index] != rightValues[index] {
			return false
		}
	}
	return true
}

func canonicalValues(answers []Answer, queryType QueryType) []string {
	set := map[string]struct{}{}
	for _, answer := range answers {
		includeCNAME := (queryType == TypeA || queryType == TypeAAAA) && answer.Type == TypeCNAME
		if answer.Type != queryType && !includeCNAME {
			continue
		}
		value := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(answer.Value), "."))
		if parsed, err := netip.ParseAddr(value); err == nil {
			value = parsed.Unmap().String()
		}
		set[string(answer.Type)+":"+value] = struct{}{}
	}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func addReason(comparison *Comparison, reason string) {
	for _, existing := range comparison.Reasons {
		if existing == reason {
			return
		}
	}
	comparison.Reasons = append(comparison.Reasons, reason)
}
