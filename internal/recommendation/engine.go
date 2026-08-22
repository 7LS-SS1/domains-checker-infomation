package recommendation

import (
	"strings"
	"time"
)

const PolicyVersion = "recommendation-2026-08-v1"

type Input struct {
	DomainID         string     `json:"domain_id"`
	Domain           string     `json:"domain"`
	Lifecycle        string     `json:"lifecycle_status"`
	SourceStatus     string     `json:"source_status"`
	Priority         string     `json:"business_priority"`
	Monitoring       bool       `json:"monitoring_enabled"`
	Availability     string     `json:"availability_status"`
	DNS              string     `json:"dns_status"`
	HTTP             string     `json:"http_status"`
	Redirect         string     `json:"redirect_status"`
	ISP              string     `json:"isp_status"`
	TLS              string     `json:"tls_status"`
	StatusConfidence int        `json:"status_confidence"`
	LastCheckedAt    *time.Time `json:"last_checked_at,omitempty"`
	ExpirationAt     *time.Time `json:"expiration_at,omitempty"`
	RenewalAmount    string     `json:"renewal_amount,omitempty"`
	RenewalCurrency  string     `json:"renewal_currency,omitempty"`
	OpenIncidents    int        `json:"open_incidents"`
	Incidents90Days  int        `json:"incidents_90_days"`
	DowntimeChanges  int        `json:"downtime_changes_90_days"`
}

type Result struct {
	Action           string   `json:"action"`
	OpportunityLevel string   `json:"opportunity_level"`
	ConfidenceScore  int      `json:"confidence_score"`
	ConfidenceLevel  string   `json:"confidence_level"`
	PolicyVersion    string   `json:"policy_version"`
	ReasonCodes      []string `json:"reason_codes"`
	ReasonsTH        []string `json:"reasons_th"`
	ReasonsEN        []string `json:"reasons_en"`
	EvidenceRefs     []string `json:"evidence_refs"`
}

type reason struct{ code, th, en, evidence string }

func Evaluate(input Input, now time.Time) Result {
	level, opportunityReasons := opportunity(input)
	result := Result{Action: "REVIEW", OpportunityLevel: level, ConfidenceScore: 55, PolicyVersion: PolicyVersion, ReasonCodes: []string{}, ReasonsTH: []string{}, ReasonsEN: []string{}, EvidenceRefs: []string{}}
	add := func(items ...reason) {
		for _, item := range items {
			result.ReasonCodes = append(result.ReasonCodes, item.code)
			result.ReasonsTH = append(result.ReasonsTH, item.th)
			result.ReasonsEN = append(result.ReasonsEN, item.en)
			if item.evidence != "" {
				result.EvidenceRefs = append(result.EvidenceRefs, item.evidence)
			}
		}
	}

	unknown := input.LastCheckedAt == nil || input.Availability == "UNKNOWN" || input.DNS == "UNKNOWN" || input.ISP == "UNKNOWN"
	if unknown {
		result.Action, result.ConfidenceScore = "REVIEW", 42
		add(reason{"DATA_INCOMPLETE", "ข้อมูลการตรวจสอบยังไม่ครบ จึงต้องให้ผู้ดูแลทบทวน", "Monitoring evidence is incomplete, so a human review is required.", "domains.current_status"})
		if input.ExpirationAt == nil {
			add(reason{"EXPIRATION_UNKNOWN", "ยังไม่ทราบวันหมดอายุของโดเมน", "The domain expiration date is unknown.", "domains.expiration_at"})
		}
		add(opportunityReasons...)
		result.ConfidenceLevel = confidenceLevel(result.ConfidenceScore)
		return result
	}

	if input.ISP == "HIGH_CONFIDENCE_BLOCK" || input.ISP == "SUSPECTED" {
		result.Action, result.ConfidenceScore = "REVIEW", max(68, input.StatusConfidence)
		add(reason{"ISP_BLOCK_REVIEW", "พบสัญญาณการบล็อกจากเครือข่าย ต้องตรวจสอบผลกระทบทางธุรกิจ", "Network blocking evidence requires a business-impact review.", "domains.current_isp_status"})
		add(opportunityReasons...)
		result.ConfidenceLevel = confidenceLevel(result.ConfidenceScore)
		return result
	}

	if input.OpenIncidents > 0 || input.Availability == "UNAVAILABLE" || badDNS(input.DNS) || badTLS(input.TLS) {
		result.Action, result.ConfidenceScore = "REVIEW", max(65, input.StatusConfidence)
		add(reason{"TECHNICAL_RISK_REVIEW", "มี incident หรือปัญหา DNS/TLS/การเข้าถึงที่ยังต้องวิเคราะห์", "An incident or DNS, TLS, or availability problem still requires analysis.", "domains.current_status"})
		if input.DowntimeChanges > 0 {
			add(reason{"DOWNTIME_HISTORY", "มีประวัติเปลี่ยนเป็นสถานะใช้งานไม่ได้ใน 90 วันที่ผ่านมา", "The domain changed to an unavailable state during the past 90 days.", "domain_status_history"})
		}
		add(opportunityReasons...)
		result.ConfidenceLevel = confidenceLevel(result.ConfidenceScore)
		return result
	}

	if strings.TrimSpace(input.RenewalAmount) == "" || strings.TrimSpace(input.RenewalCurrency) == "" {
		result.Action, result.ConfidenceScore = "REVIEW", 48
		add(reason{"RENEWAL_COST_UNKNOWN", "ยังไม่มีต้นทุนต่ออายุที่เชื่อถือได้ จึงไม่ควรตัดสินใจต่ออายุหรือยกเลิกอัตโนมัติ", "No reliable renewal cost is available, so renew or drop should not be decided automatically.", "domain_costs"})
		add(opportunityReasons...)
		result.ConfidenceLevel = confidenceLevel(result.ConfidenceScore)
		return result
	}

	if input.Lifecycle == "inactive" && input.Priority == "low" && input.Redirect == "PERMANENT" && input.SourceStatus != "missing_from_source" {
		result.Action, result.ConfidenceScore = "DROP", 82
		add(reason{"INACTIVE_LOW_PRIORITY", "โดเมนไม่ได้ใช้งานและมีความสำคัญทางธุรกิจต่ำ", "The domain is inactive and has low business priority.", "domains.lifecycle_status"}, reason{"PERMANENT_REDIRECT", "โดเมนเปลี่ยนเส้นทางแบบถาวรอยู่แล้ว", "The domain already uses a permanent redirect.", "domains.current_redirect_status"})
		add(opportunityReasons...)
		result.ConfidenceLevel = confidenceLevel(result.ConfidenceScore)
		return result
	}

	if level == "HIGH" && (input.Priority == "low" || input.Priority == "medium") && input.Lifecycle != "archived" {
		result.Action, result.ConfidenceScore = "PROFIT_OPPORTUNITY", 72
		add(reason{"PROFIT_INDICATORS_HIGH", "ตัวชี้วัดด้านชื่อโดเมนอยู่ในระดับสูง ควรประเมินโอกาสเชิงพาณิชย์โดยมนุษย์", "Domain-name indicators are high; a human should assess the commercial opportunity.", "recommendation.indicators"})
		add(opportunityReasons...)
		result.ConfidenceLevel = confidenceLevel(result.ConfidenceScore)
		return result
	}

	expiringSoon := input.ExpirationAt != nil && !input.ExpirationAt.Before(now) && input.ExpirationAt.Before(now.Add(90*24*time.Hour))
	if (input.Priority == "high" || input.Priority == "critical") || (input.Lifecycle == "active" && input.Availability == "ACTIVE" && expiringSoon) {
		result.Action, result.ConfidenceScore = "RENEW", 84
		if input.Priority == "high" || input.Priority == "critical" {
			add(reason{"BUSINESS_IMPORTANCE", "โดเมนมีความสำคัญทางธุรกิจสูง", "The domain has high business importance.", "domains.business_priority"})
		}
		if expiringSoon {
			add(reason{"EXPIRING_SOON", "โดเมนจะหมดอายุภายใน 90 วัน", "The domain expires within 90 days.", "domains.expiration_at"})
		}
		add(reason{"HEALTHY_MONITORING", "ผลตรวจล่าสุดไม่พบความเสี่ยงสำคัญ", "The latest monitoring evidence shows no material technical risk.", "domains.current_status"})
		add(opportunityReasons...)
		result.ConfidenceLevel = confidenceLevel(result.ConfidenceScore)
		return result
	}

	result.Action, result.ConfidenceScore = "REVIEW", 60
	add(reason{"POLICY_REVIEW_REQUIRED", "หลักฐานยังไม่เพียงพอสำหรับการต่ออายุหรือยกเลิกอย่างอัตโนมัติ", "Evidence is not strong enough for an automatic renew or drop decision.", "recommendation.policy"})
	add(opportunityReasons...)
	result.ConfidenceLevel = confidenceLevel(result.ConfidenceScore)
	return result
}

func opportunity(input Input) (string, []reason) {
	registrable := strings.ToLower(strings.TrimSuffix(input.Domain, "."))
	parts := strings.Split(registrable, ".")
	label, tld := parts[0], ""
	if len(parts) > 1 {
		tld = parts[len(parts)-1]
	}
	score := 0
	reasons := []reason{}
	if len(label) <= 8 {
		score += 2
		reasons = append(reasons, reason{"SHORT_DOMAIN", "ชื่อหลักของโดเมนสั้นและจดจำง่าย", "The primary domain label is short and easier to remember.", "domains.domain_ascii"})
	} else if len(label) <= 12 {
		score++
	}
	if !strings.Contains(label, "-") && !strings.ContainsAny(label, "0123456789") {
		score += 2
		reasons = append(reasons, reason{"BRANDABLE_PATTERN", "รูปแบบชื่อไม่มีขีดกลางหรือตัวเลข จึงมีความเป็นแบรนด์ได้ดีขึ้น", "The name has no hyphen or digits, which improves brandability indicators.", "domains.domain_ascii"})
	}
	switch tld {
	case "com":
		score += 2
		reasons = append(reasons, reason{"STRONG_TLD", "ใช้ TLD .com ซึ่งเป็นสัญญาณเชิงพาณิชย์ที่แข็งแรง", "The .com TLD is a strong commercial indicator.", "domains.domain_ascii"})
	case "net", "org", "co", "th":
		score++
	}
	if input.LastCheckedAt != nil && input.Availability != "UNKNOWN" {
		score++
	}
	level := "LOW"
	if score >= 6 {
		level = "HIGH"
	} else if score >= 3 {
		level = "MEDIUM"
	}
	return level, reasons
}

func badDNS(value string) bool { return value != "OK" && value != "UNKNOWN" }
func badTLS(value string) bool {
	return value != "VALID" && value != "NOT_APPLICABLE" && value != "UNKNOWN" && value != "EXPIRING"
}
func confidenceLevel(score int) string {
	if score >= 70 {
		return "HIGH"
	}
	if score >= 40 {
		return "MEDIUM"
	}
	return "LOW"
}
