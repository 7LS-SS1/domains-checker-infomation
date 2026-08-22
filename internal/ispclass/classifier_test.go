package ispclass

import "testing"

func TestClassifyCrossVantageScenarios(t *testing.T) {
	tests := []struct {
		name  string
		input Input
		want  string
	}{
		{name: "local success", input: Input{LocalAvailability: "ACTIVE"}, want: NotDetected},
		{name: "global origin failure", input: Input{LocalAvailability: "UNAVAILABLE", RemoteAvailability: "UNAVAILABLE", RemoteFresh: true, ProbeHealthy: true, ClockHealthy: true}, want: Unknown},
		{name: "dns interference", input: Input{LocalAvailability: "UNAVAILABLE", LocalFailureCount: 3, LocalDNSDiscrepancy: true, DoHStable: true, PinnedAvailability: "ACTIVE", RemoteAvailability: "ACTIVE", RemoteFresh: true, ProbeHealthy: true, ClockHealthy: true}, want: Suspected},
		{name: "high confidence path failure", input: Input{LocalAvailability: "UNAVAILABLE", LocalFailureCount: 3, LocalFailureBuckets: 2, DoHStable: true, PinnedAvailability: "UNAVAILABLE", PinnedFailureStage: "tcp", RemoteAvailability: "ACTIVE", RemoteSuccessCount: 2, RemoteBodyHashStable: true, RemoteFresh: true, ProbeHealthy: true, ClockHealthy: true}, want: HighConfidenceBlock},
		{name: "stale remote", input: Input{LocalAvailability: "UNAVAILABLE", RemoteAvailability: "ACTIVE", RemoteFresh: false, ProbeHealthy: true, ClockHealthy: true}, want: Unknown},
		{name: "waf is not a block conclusion", input: Input{LocalAvailability: "UNAVAILABLE", LocalFailureCount: 3, LocalFailureBuckets: 2, DoHStable: true, PinnedAvailability: "UNAVAILABLE", PinnedFailureStage: "http", RemoteAvailability: "ACTIVE", RemoteSuccessCount: 2, RemoteBodyHashStable: true, RemoteFresh: true, ProbeHealthy: true, ClockHealthy: true, PossibleWAF: true}, want: Unknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Classify(test.input)
			if got.Status != test.want {
				t.Fatalf("status = %s, want %s, reasons = %v", got.Status, test.want, got.Reasons)
			}
			if got.Confidence > 99 {
				t.Fatalf("confidence = %d", got.Confidence)
			}
		})
	}
}
