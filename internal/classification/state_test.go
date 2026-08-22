package classification

import "testing"

func TestAdvanceUsesFailureAndRecoveryHysteresis(t *testing.T) {
	current := EffectiveInput{
		CurrentAvailability: "ACTIVE", CurrentDNS: "OK", CurrentHTTP: "OK", CurrentRedirect: "NONE",
		CurrentISP: "UNKNOWN", CurrentTLS: "VALID", CurrentContent: "VALID_HTML", CurrentConfidence: 85,
		Qualifying: true, OpenFailures: 3, CloseSuccesses: 2,
		Observed: Decision{Availability: "UNAVAILABLE", DNS: "TIMEOUT", HTTP: "TIMEOUT", Redirect: "NONE", ISP: "UNKNOWN", TLS: "UNKNOWN", Content: "UNKNOWN", FailureStage: "dns", ErrorCode: "DNS_TIMEOUT", Confidence: 40},
	}
	first := Advance(current)
	if first.Availability != "DEGRADED" || first.OpenedIncident || first.FailureStreak != 1 {
		t.Fatalf("first failure = %#v", first)
	}
	current.CurrentAvailability, current.FailureStreak = first.Availability, first.FailureStreak
	second := Advance(current)
	if second.Availability != "DEGRADED" || second.OpenedIncident {
		t.Fatalf("second failure = %#v", second)
	}
	current.CurrentAvailability, current.FailureStreak = second.Availability, second.FailureStreak
	third := Advance(current)
	if third.Availability != "UNAVAILABLE" || !third.OpenedIncident || third.FailureStreak != 3 {
		t.Fatalf("third failure = %#v", third)
	}

	recovery := EffectiveInput{
		CurrentAvailability: third.Availability, CurrentDNS: third.DNS, CurrentHTTP: third.HTTP,
		CurrentRedirect: third.Redirect, CurrentISP: third.ISP, CurrentTLS: third.TLS, CurrentContent: third.Content,
		FailureStreak: third.FailureStreak, Qualifying: true, OpenFailures: 3, CloseSuccesses: 2,
		Observed: Decision{Availability: "ACTIVE", DNS: "OK", HTTP: "OK", Redirect: "NONE", ISP: "UNKNOWN", TLS: "VALID", Content: "VALID_HTML", Confidence: 85},
	}
	firstRecovery := Advance(recovery)
	if firstRecovery.Availability != "DEGRADED" || firstRecovery.ClosedIncident {
		t.Fatalf("first recovery = %#v", firstRecovery)
	}
	recovery.CurrentAvailability, recovery.SuccessStreak, recovery.FailureStreak = firstRecovery.Availability, firstRecovery.SuccessStreak, firstRecovery.FailureStreak
	secondRecovery := Advance(recovery)
	if secondRecovery.Availability != "ACTIVE" || secondRecovery.SuccessStreak != 2 {
		t.Fatalf("second recovery = %#v", secondRecovery)
	}
}

func TestManualObservationDoesNotMutateEffectiveState(t *testing.T) {
	state := Advance(EffectiveInput{
		CurrentAvailability: "ACTIVE", CurrentDNS: "OK", CurrentHTTP: "OK", CurrentRedirect: "NONE",
		CurrentISP: "UNKNOWN", CurrentTLS: "VALID", CurrentContent: "VALID_HTML", CurrentConfidence: 90,
		FailureStreak: 2, Qualifying: false,
		Observed: Decision{Availability: "UNAVAILABLE", DNS: "TIMEOUT", HTTP: "TIMEOUT", Confidence: 40},
	})
	if state.Availability != "ACTIVE" || state.FailureStreak != 2 || len(state.ChangedDimensions) != 0 {
		t.Fatalf("manual observation changed effective state: %#v", state)
	}
}
