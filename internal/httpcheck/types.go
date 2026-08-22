package httpcheck

import (
	"time"

	"domainmonitor/internal/netcheck"
	"domainmonitor/internal/retry"
	"domainmonitor/internal/tlscheck"
)

type ContentMode string

const (
	ContentHTML       ContentMode = "HTML"
	ContentAny        ContentMode = "ANY"
	ContentStatusOnly ContentMode = "STATUS_ONLY"
)

type Config struct {
	MaxRedirects          int
	MaxBodyBytes          int64
	ExcerptBytes          int
	HeaderBytes           int
	MinMeaningfulBytes    int
	ConnectTimeout        time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	ChainTimeout          time.Duration
	TLSExpiringSoon       time.Duration
	MaxCertificateSANs    int
	UserAgent             string
	Accept                string
	AcceptLanguage        string
	Retry                 retry.Policy
}

type Timing struct {
	DNS     time.Duration `json:"dns,omitempty"`
	Connect time.Duration `json:"connect,omitempty"`
	TLS     time.Duration `json:"tls,omitempty"`
	TTFB    time.Duration `json:"ttfb,omitempty"`
	Total   time.Duration `json:"total"`
	Reused  bool          `json:"connection_reused"`
}

type RedirectHop struct {
	Hop            int           `json:"hop"`
	SourceURL      string        `json:"source_url"`
	StatusCode     int           `json:"status_code"`
	Location       string        `json:"location"`
	ResolvedTarget string        `json:"resolved_target,omitempty"`
	CrossDomain    bool          `json:"cross_domain"`
	HTTPSDowngrade bool          `json:"https_downgrade"`
	Duration       time.Duration `json:"duration"`
	ErrorCode      netcheck.Code `json:"error_code,omitempty"`
}

type ContentEvidence struct {
	Status                string        `json:"status"`
	ContentType           string        `json:"content_type,omitempty"`
	DeclaredContentLength int64         `json:"declared_content_length"`
	BodySize              int64         `json:"body_size"`
	BodySHA256            []byte        `json:"body_sha256,omitempty"`
	HashComplete          bool          `json:"hash_complete"`
	Excerpt               []byte        `json:"excerpt,omitempty"`
	Title                 string        `json:"title,omitempty"`
	ErrorCode             netcheck.Code `json:"error_code,omitempty"`
	ErrorMessage          string        `json:"error_message,omitempty"`
}

type Attempt struct {
	Attempt           int               `json:"attempt"`
	RequestURL        string            `json:"request_url"`
	EffectiveURL      string            `json:"effective_url,omitempty"`
	Protocol          string            `json:"protocol,omitempty"`
	InitialStatusCode int               `json:"initial_status_code,omitempty"`
	FinalStatusCode   int               `json:"final_status_code,omitempty"`
	Redirects         []RedirectHop     `json:"redirects"`
	Timing            Timing            `json:"timing"`
	Content           ContentEvidence   `json:"content"`
	ServerHeader      string            `json:"server_header,omitempty"`
	Headers           map[string]string `json:"selected_headers"`
	TLS               *tlscheck.Result  `json:"tls,omitempty"`
	ErrorCode         netcheck.Code     `json:"error_code,omitempty"`
	ErrorMessage      string            `json:"error_message,omitempty"`
	CheckedAt         time.Time         `json:"checked_at"`
	RetryAfter        time.Duration     `json:"-"`
	Retryable         bool              `json:"-"`
}

type OriginResult struct {
	HTTPS            []Attempt `json:"https"`
	HTTP             []Attempt `json:"http,omitempty"`
	UsedHTTPFallback bool      `json:"used_http_fallback"`
}
