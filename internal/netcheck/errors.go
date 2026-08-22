package netcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
)

type Code string

const (
	NormalizationInvalid        Code = "NORMALIZATION_INVALID"
	DNSNXDomain                 Code = "DNS_NXDOMAIN"
	DNSServFail                 Code = "DNS_SERVFAIL"
	DNSRefused                  Code = "DNS_REFUSED"
	DNSTimeout                  Code = "DNS_TIMEOUT"
	DNSNetworkError             Code = "DNS_NETWORK_ERROR"
	DNSMalformedResponse        Code = "DNS_MALFORMED_RESPONSE"
	DNSCNAMELoop                Code = "DNS_CNAME_LOOP"
	DNSCNAMELimit               Code = "DNS_CNAME_LIMIT"
	TCPTimeout                  Code = "TCP_TIMEOUT"
	TCPRefused                  Code = "TCP_REFUSED"
	TCPReset                    Code = "TCP_RESET"
	TCPNetworkError             Code = "TCP_NETWORK_ERROR"
	TLSErrorExpired             Code = "TLS_EXPIRED"
	TLSErrorHostname            Code = "TLS_HOSTNAME"
	TLSErrorUnknownAuthority    Code = "TLS_UNKNOWN_AUTHORITY"
	TLSErrorHandshake           Code = "TLS_HANDSHAKE"
	HTTPTimeout                 Code = "HTTP_TIMEOUT"
	HTTPBodyRead                Code = "HTTP_BODY_READ"
	HTTPClientError             Code = "HTTP_4XX"
	HTTPServerError             Code = "HTTP_5XX"
	HTTPRedirectLoop            Code = "HTTP_REDIRECT_LOOP"
	HTTPRedirectLimit           Code = "HTTP_REDIRECT_LIMIT"
	HTTPMalformedLocation       Code = "HTTP_MALFORMED_LOCATION"
	HTTPHTTPSDowngrade          Code = "HTTP_HTTPS_DOWNGRADE"
	ContentEmpty                Code = "CONTENT_EMPTY"
	ContentTooSmall             Code = "CONTENT_TOO_SMALL"
	ContentNotMeaningful        Code = "CONTENT_NOT_MEANINGFUL"
	ContentTooLarge             Code = "CONTENT_TOO_LARGE"
	ContentUnsupported          Code = "CONTENT_UNSUPPORTED"
	SSRFBlockedAddress          Code = "SSRF_BLOCKED_ADDRESS"
	SSRFBlockedPort             Code = "SSRF_BLOCKED_PORT"
	RemoteProbeUnavailable      Code = "REMOTE_PROBE_UNAVAILABLE"
	RemoteProbeStale            Code = "REMOTE_PROBE_STALE"
	RemoteProbeInvalidSignature Code = "REMOTE_PROBE_INVALID_SIGNATURE"
	RunCancelled                Code = "RUN_CANCELLED"
	RunDeadlineExceeded         Code = "RUN_DEADLINE_EXCEEDED"
	InternalPersistenceError    Code = "INTERNAL_PERSISTENCE_ERROR"
)

type Error struct {
	Code      Code
	Stage     string
	Operation string
	Retryable bool
	Cause     error
}

func New(code Code, stage, operation string, retryable bool, cause error) *Error {
	return &Error{Code: code, Stage: stage, Operation: operation, Retryable: retryable, Cause: cause}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	message := string(e.Code)
	if e.Operation != "" {
		message += " during " + e.Operation
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func As(err error) (*Error, bool) {
	var checkError *Error
	if errors.As(err, &checkError) {
		return checkError, true
	}
	return nil, false
}

func IsRetryable(err error) bool {
	checkError, ok := As(err)
	return ok && checkError.Retryable
}

func FromContext(ctx context.Context, stage, operation string, timeoutCode Code, err error) *Error {
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled):
		return New(RunCancelled, stage, operation, false, context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return New(timeoutCode, stage, operation, true, context.DeadlineExceeded)
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return New(RunDeadlineExceeded, stage, operation, false, context.DeadlineExceeded)
	default:
		return nil
	}
}

func ClassifyTCP(ctx context.Context, operation string, err error) *Error {
	if classified := FromContext(ctx, "tcp", operation, TCPTimeout, err); classified != nil {
		return classified
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return New(TCPRefused, "tcp", operation, true, err)
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return New(TCPReset, "tcp", operation, true, err)
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return New(TCPTimeout, "tcp", operation, true, err)
	}
	var operationError *net.OpError
	if errors.As(err, &operationError) {
		return New(TCPNetworkError, "tcp", operation, operationError.Temporary(), err)
	}
	var syscallError *os.SyscallError
	if errors.As(err, &syscallError) {
		return New(TCPNetworkError, "tcp", operation, true, err)
	}
	return New(TCPNetworkError, "tcp", operation, false, fmt.Errorf("transport failure: %w", err))
}
