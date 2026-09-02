package monitor

import (
	"encoding/json"
	"time"

	"domainmonitor/internal/classification"
	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/httpcheck"
	"github.com/google/uuid"
)

type Run struct {
	ID                uuid.UUID       `json:"id"`
	DomainID          uuid.UUID       `json:"domain_id"`
	DomainASCII       string          `json:"domain_ascii,omitempty"`
	TriggerType       string          `json:"trigger_type"`
	Status            string          `json:"status"`
	Priority          string          `json:"priority"`
	DeduplicationKey  string          `json:"deduplication_key,omitempty"`
	PolicyVersion     string          `json:"policy_version"`
	PolicySnapshot    json.RawMessage `json:"policy_snapshot,omitempty"`
	RequestedBy       *uuid.UUID      `json:"requested_by,omitempty"`
	ScheduledFor      time.Time       `json:"scheduled_for"`
	DeadlineAt        time.Time       `json:"deadline_at"`
	StartedAt         *time.Time      `json:"started_at,omitempty"`
	CompletedAt       *time.Time      `json:"completed_at,omitempty"`
	LastErrorCode     *string         `json:"last_error_code,omitempty"`
	LastErrorMessage  *string         `json:"last_error_message,omitempty"`
	ExecutionAttempts int             `json:"execution_attempts"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type Target struct {
	DomainASCII         string
	ExpectedContentMode string
	CurrentAvailability string
	CurrentDNS          string
	CurrentHTTP         string
	CurrentRedirect     string
	CurrentISP          string
	CurrentTLS          string
	CurrentContent      string
	CurrentConfidence   int16
	FailureStreak       int
	SuccessStreak       int
}

type Execution struct {
	Run    Run
	Target Target
}

type Evidence struct {
	LocalDNS     []dnscheck.Result
	AlternateDNS []dnscheck.Result
	HTTP         httpcheck.OriginResult
	PinnedHTTP   *httpcheck.OriginResult
	Decision     classification.Decision
	CheckedAt    time.Time
}

type Result struct {
	ID              uuid.UUID  `json:"id"`
	RunID           uuid.UUID  `json:"monitoring_run_id"`
	DomainID        uuid.UUID  `json:"domain_id"`
	VantageType     string     `json:"vantage_type"`
	VantageKey      string     `json:"vantage_key"`
	VantageCountry  *string    `json:"vantage_country,omitempty"`
	VantageNetwork  *string    `json:"vantage_network,omitempty"`
	Availability    string     `json:"availability_status"`
	DNS             string     `json:"dns_status"`
	HTTP            string     `json:"http_status"`
	Redirect        string     `json:"redirect_status"`
	ISP             string     `json:"isp_status"`
	TLS             string     `json:"tls_status"`
	Content         string     `json:"content_status"`
	InitialHTTP     *int       `json:"initial_http_status_code,omitempty"`
	FinalHTTP       *int       `json:"final_http_status_code,omitempty"`
	FinalTarget     *string    `json:"final_target_status,omitempty"`
	FailureStage    *string    `json:"failure_stage,omitempty"`
	ErrorCode       *string    `json:"error_code,omitempty"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
	Confidence      int16      `json:"confidence_score"`
	ConfidenceLevel string     `json:"confidence_level"`
	ReasonCodes     []string   `json:"reason_codes"`
	PolicyVersion   string     `json:"policy_version"`
	CheckedAt       time.Time  `json:"checked_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

type DNSCheck struct {
	ID            uuid.UUID       `json:"id"`
	ResultID      uuid.UUID       `json:"monitoring_result_id"`
	ResolverType  string          `json:"resolver_type"`
	Endpoint      string          `json:"resolver_endpoint"`
	QueryName     string          `json:"query_name"`
	QueryType     string          `json:"query_type"`
	Attempt       int             `json:"attempt"`
	RCode         *string         `json:"rcode,omitempty"`
	Truncated     bool            `json:"truncated"`
	Authoritative bool            `json:"authoritative"`
	DurationUS    int64           `json:"duration_us"`
	ErrorCode     *string         `json:"error_code,omitempty"`
	ErrorMessage  *string         `json:"error_message,omitempty"`
	RawEvidence   json.RawMessage `json:"raw_evidence"`
	CheckedAt     time.Time       `json:"checked_at"`
	Answers       []DNSAnswer     `json:"answers"`
}

type DNSAnswer struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   uint32 `json:"ttl_seconds"`
}

type HTTPCheck struct {
	ID                    uuid.UUID       `json:"id"`
	ResultID              uuid.UUID       `json:"monitoring_result_id"`
	Scheme                string          `json:"scheme"`
	ResolverMode          string          `json:"resolver_mode"`
	RequestURL            string          `json:"request_url"`
	EffectiveURL          *string         `json:"effective_url,omitempty"`
	Protocol              *string         `json:"protocol,omitempty"`
	Attempt               int             `json:"attempt"`
	InitialStatus         *int            `json:"initial_status_code,omitempty"`
	FinalStatus           *int            `json:"final_status_code,omitempty"`
	DNSDurationUS         *int64          `json:"dns_duration_us,omitempty"`
	ConnectDurationUS     *int64          `json:"connect_duration_us,omitempty"`
	TLSDurationUS         *int64          `json:"tls_duration_us,omitempty"`
	TTFBDurationUS        *int64          `json:"ttfb_duration_us,omitempty"`
	TotalDurationUS       int64           `json:"total_duration_us"`
	ContentType           *string         `json:"content_type,omitempty"`
	DeclaredContentLength *int64          `json:"declared_content_length,omitempty"`
	BodySize              int64           `json:"body_size"`
	BodySHA256            []byte          `json:"body_sha256,omitempty"`
	HashComplete          bool            `json:"hash_complete"`
	BodyExcerpt           []byte          `json:"body_excerpt,omitempty"`
	Title                 *string         `json:"title,omitempty"`
	ContentStatus         string          `json:"content_status"`
	ServerHeader          *string         `json:"server_header,omitempty"`
	SelectedHeaders       json.RawMessage `json:"selected_headers"`
	ErrorCode             *string         `json:"error_code,omitempty"`
	ErrorMessage          *string         `json:"error_message,omitempty"`
	CheckedAt             time.Time       `json:"checked_at"`
	Redirects             []RedirectHop   `json:"redirects"`
	TLSCheck              *TLSCheck       `json:"tls_check,omitempty"`
}

type RedirectHop struct {
	Hop            int     `json:"hop"`
	SourceURL      string  `json:"source_url"`
	StatusCode     int     `json:"status_code"`
	Location       string  `json:"location"`
	ResolvedTarget *string `json:"resolved_target,omitempty"`
	CrossDomain    bool    `json:"cross_domain"`
	HTTPSDowngrade bool    `json:"https_downgrade"`
	DurationUS     int64   `json:"duration_us"`
	ErrorCode      *string `json:"error_code,omitempty"`
}

type TLSCheck struct {
	ServerName            string          `json:"server_name"`
	RemoteAddress         *string         `json:"remote_address,omitempty"`
	TLSVersion            *string         `json:"tls_version,omitempty"`
	CipherSuite           *string         `json:"cipher_suite,omitempty"`
	CertificateSubject    *string         `json:"certificate_subject,omitempty"`
	CertificateIssuer     *string         `json:"certificate_issuer,omitempty"`
	CertificateSerialHash []byte          `json:"certificate_serial_hash,omitempty"`
	SANs                  json.RawMessage `json:"sans"`
	ValidFrom             *time.Time      `json:"valid_from,omitempty"`
	ValidUntil            *time.Time      `json:"valid_until,omitempty"`
	ExpirationDays        *int            `json:"certificate_expiration_days,omitempty"`
	HostnameValid         *bool           `json:"hostname_valid,omitempty"`
	ChainValid            *bool           `json:"chain_valid,omitempty"`
	Status                string          `json:"tls_status"`
	DiagnosticOnly        bool            `json:"diagnostic_only"`
	ErrorCode             *string         `json:"error_code,omitempty"`
	ErrorMessage          *string         `json:"error_message,omitempty"`
	CheckedAt             time.Time       `json:"checked_at"`
}

type RunDetail struct {
	Run        Run         `json:"run"`
	Results    []Result    `json:"results"`
	DNSChecks  []DNSCheck  `json:"dns_checks"`
	HTTPChecks []HTTPCheck `json:"http_checks"`
}

type RunPage struct {
	Items      []Run `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type HistoryEntry struct {
	ID               uuid.UUID       `json:"id"`
	Dimension        string          `json:"dimension"`
	PreviousValue    *string         `json:"previous_value,omitempty"`
	CurrentValue     string          `json:"current_value"`
	Confidence       int16           `json:"confidence_score"`
	PolicyVersion    string          `json:"policy_version"`
	ReasonCodes      json.RawMessage `json:"reason_codes"`
	SupportingRunIDs []uuid.UUID     `json:"supporting_run_ids"`
	EffectiveAt      time.Time       `json:"effective_at"`
}

type HistoryAggregate struct {
	WindowStart        time.Time `json:"window_start"`
	WindowEnd          time.Time `json:"window_end"`
	UptimePercentage   *float64  `json:"uptime_percentage,omitempty"`
	MonitoringCoverage float64   `json:"monitoring_coverage"`
	KnownSeconds       float64   `json:"known_seconds"`
	ActiveSeconds      float64   `json:"active_seconds"`
	DegradedSeconds    float64   `json:"degraded_seconds"`
	UnavailableSeconds float64   `json:"unavailable_seconds"`
	StatusChangeCount  int       `json:"status_change_count"`
	IncidentCount      int       `json:"incident_count"`
	AverageResponseMS  *float64  `json:"average_response_ms,omitempty"`
}

type History struct {
	DomainID  uuid.UUID        `json:"domain_id"`
	Timeline  []HistoryEntry   `json:"timeline"`
	Aggregate HistoryAggregate `json:"aggregate"`
}

type Incident struct {
	ID                uuid.UUID  `json:"id"`
	DomainID          uuid.UUID  `json:"domain_id"`
	DomainASCII       string     `json:"domain_ascii"`
	Status            string     `json:"status"`
	FailureStage      *string    `json:"failure_stage,omitempty"`
	ErrorCode         *string    `json:"error_code,omitempty"`
	OpenFailureCount  int        `json:"open_failure_count"`
	CloseSuccessCount int        `json:"close_success_count"`
	OpenedAt          time.Time  `json:"opened_at"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	OpenedByRunID     uuid.UUID  `json:"opened_by_run_id"`
	ClosedByRunID     *uuid.UUID `json:"closed_by_run_id,omitempty"`
}

type IncidentPage struct {
	Items      []Incident `json:"items"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalItems int64      `json:"total_items"`
	TotalPages int        `json:"total_pages"`
}
