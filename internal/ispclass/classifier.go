package ispclass

import "slices"

const (
	Unknown             = "UNKNOWN"
	NotDetected         = "NOT_DETECTED"
	Suspected           = "SUSPECTED"
	HighConfidenceBlock = "HIGH_CONFIDENCE_BLOCK"
)

type Input struct {
	LocalAvailability    string
	LocalFailureCount    int
	LocalFailureBuckets  int
	LocalDNSDiscrepancy  bool
	DoHStable            bool
	PinnedAvailability   string
	PinnedFailureStage   string
	RemoteAvailability   string
	RemoteSuccessCount   int
	RemoteBodyHashStable bool
	RemoteFresh          bool
	ProbeHealthy         bool
	ClockHealthy         bool
	PossibleGeoPolicy    bool
	PossibleWAF          bool
	MaintenanceWindow    bool
}

type Decision struct {
	Status     string   `json:"isp_status"`
	Confidence int16    `json:"confidence_score"`
	Reasons    []string `json:"reason_codes"`
}

func Classify(input Input) Decision {
	if input.LocalAvailability == "ACTIVE" {
		return decision(NotDetected, 90, "LOCAL_CONTENT_VALID")
	}
	if !input.RemoteFresh {
		return decision(Unknown, 20, "REMOTE_EVIDENCE_STALE")
	}
	if !input.ProbeHealthy || !input.ClockHealthy {
		return decision(Unknown, 20, "REMOTE_PROBE_UNHEALTHY")
	}
	if input.RemoteAvailability != "ACTIVE" {
		if input.LocalAvailability == "UNAVAILABLE" || input.LocalAvailability == "DEGRADED" {
			return decision(Unknown, 55, "ORIGIN_GLOBAL_FAILURE")
		}
		return decision(Unknown, 25, "REMOTE_EVIDENCE_INCONCLUSIVE")
	}

	reasons := []string{"REMOTE_SG_CONTENT_VALID"}
	if input.LocalFailureCount > 0 {
		reasons = append(reasons, "LOCAL_SYSTEM_HTTP_FAILED")
	}
	if input.LocalDNSDiscrepancy {
		reasons = append(reasons, "LOCAL_DNS_ANSWER_SET_DIFFERS")
	}
	if input.DoHStable {
		reasons = append(reasons, "DOH_ORIGIN_IP_STABLE")
	}
	if input.PinnedAvailability == "ACTIVE" && input.LocalDNSDiscrepancy {
		return decision(Suspected, 75, append(reasons, "LOCAL_DOH_PINNED_HTTP_SUCCEEDED", "DNS_INTERFERENCE_SUSPECTED")...)
	}
	if input.PinnedAvailability != "ACTIVE" {
		reasons = append(reasons, "LOCAL_DOH_PINNED_HTTP_FAILED")
	}
	if input.PossibleGeoPolicy {
		return decision(Unknown, 45, append(reasons, "POSSIBLE_GEO_POLICY")...)
	}
	if input.PossibleWAF {
		return decision(Unknown, 45, append(reasons, "POSSIBLE_WAF_BLOCK")...)
	}
	if input.MaintenanceWindow {
		return decision(Unknown, 30, append(reasons, "MAINTENANCE_WINDOW_ACTIVE")...)
	}
	if input.LocalFailureCount < 3 || input.LocalFailureBuckets < 2 {
		return decision(Suspected, 55, append(reasons, "LOCAL_FAILURE_NOT_YET_REPEATED")...)
	}
	if input.RemoteSuccessCount < 2 || !input.RemoteBodyHashStable {
		return decision(Suspected, 65, append(reasons, "REMOTE_SUCCESS_NOT_YET_REPEATED")...)
	}
	if !input.DoHStable {
		return decision(Unknown, 40, append(reasons, "DOH_EVIDENCE_UNSTABLE")...)
	}
	if input.PinnedFailureStage == "" {
		return decision(Suspected, 60, append(reasons, "LOCAL_FAILURE_STAGE_INCONSISTENT")...)
	}
	return decision(HighConfidenceBlock, 95, append(reasons,
		"LOCAL_FAILURE_REPEATED", "REMOTE_SG_BODY_HASH_STABLE", "NETWORK_SCOPED_HIGH_CONFIDENCE_BLOCK")...)
}

func decision(status string, confidence int16, reasons ...string) Decision {
	reasons = slices.Compact(reasons)
	if confidence > 99 {
		confidence = 99
	}
	return Decision{Status: status, Confidence: confidence, Reasons: reasons}
}
