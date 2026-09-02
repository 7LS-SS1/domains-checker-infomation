package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Domain struct {
	ID                    uuid.UUID  `json:"id"`
	OriginalInput         string     `json:"original_input"`
	ASCII                 string     `json:"domain_ascii"`
	Unicode               string     `json:"domain_unicode"`
	RegistrableDomain     string     `json:"registrable_domain"`
	RegistrarID           *uuid.UUID `json:"registrar_id,omitempty"`
	LifecycleStatus       string     `json:"lifecycle_status"`
	SourceStatus          string     `json:"source_status"`
	SourceType            string     `json:"source_type"`
	BusinessPriority      string     `json:"business_priority"`
	MonitoringEnabled     bool       `json:"monitoring_enabled"`
	ExpectedContentMode   string     `json:"expected_content_mode"`
	ExpirationAt          *time.Time `json:"expiration_at,omitempty"`
	Notes                 string     `json:"notes"`
	RenewalDecision       string     `json:"renewal_decision"`
	RenewalDecisionReason string     `json:"renewal_decision_reason"`
	RenewalDecidedAt      *time.Time `json:"renewal_decided_at,omitempty"`
	RenewalPrice          *string    `json:"renewal_price,omitempty"`
	RenewalCurrency       *string    `json:"renewal_currency,omitempty"`
	RedirectTargetURL     *string    `json:"redirect_target_url,omitempty"`
	LatestHTTPCode        *int       `json:"latest_http_status_code,omitempty"`
	AvailabilityStatus    string     `json:"availability_status"`
	DNSStatus             string     `json:"dns_status"`
	HTTPStatus            string     `json:"http_status"`
	RedirectStatus        string     `json:"redirect_status"`
	ISPStatus             string     `json:"isp_status"`
	TLSStatus             string     `json:"tls_status"`
	ContentStatus         string     `json:"content_status"`
	ConfidenceScore       int16      `json:"confidence_score"`
	ConsecutiveFailures   int        `json:"consecutive_failures"`
	ConsecutiveSuccesses  int        `json:"consecutive_successes"`
	CurrentFailureStage   *string    `json:"current_failure_stage,omitempty"`
	CurrentErrorCode      *string    `json:"current_error_code,omitempty"`
	LastCheckedAt         *time.Time `json:"last_checked_at,omitempty"`
	ArchivedAt            *time.Time `json:"archived_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	Version               int64      `json:"version"`
}

type CreateInput struct {
	Domain              string
	RegistrarID         *uuid.UUID
	BusinessPriority    string
	MonitoringEnabled   bool
	ExpectedContentMode string
	ExpirationAt        *time.Time
	Notes               string
}

type PatchInput struct {
	Version             int64
	Domain              *string
	RegistrarID         *uuid.UUID
	ClearRegistrar      bool
	BusinessPriority    *string
	MonitoringEnabled   *bool
	ExpectedContentMode *string
	ExpirationAt        *time.Time
	ClearExpiration     bool
	Notes               *string
	RenewalDecision     *string
	Reason              string
}

type ListFilter struct {
	Query           string
	LifecycleStatus string
	SourceStatus    string
	Page            int
	PageSize        int
	Sort            string
	Direction       string
}

type Page struct {
	Items      []Domain `json:"items"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	TotalItems int64    `json:"total_items"`
	TotalPages int      `json:"total_pages"`
}

type Actor struct {
	UserID    uuid.UUID
	RequestID string
}

type Provenance struct {
	FieldName       string          `json:"field_name"`
	Value           json.RawMessage `json:"value"`
	Source          string          `json:"source"`
	SourceReference string          `json:"source_reference"`
	ObservedAt      time.Time       `json:"observed_at"`
	IsCurrent       bool            `json:"is_current"`
}
