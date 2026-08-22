package tlscheck

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"domainmonitor/internal/netcheck"
)

type DialContext func(ctx context.Context, network, address string) (net.Conn, error)

type Result struct {
	ServerName         string        `json:"server_name"`
	RemoteAddress      string        `json:"remote_address,omitempty"`
	TLSVersion         string        `json:"tls_version,omitempty"`
	CipherSuite        string        `json:"cipher_suite,omitempty"`
	CertificateSubject string        `json:"certificate_subject,omitempty"`
	CertificateIssuer  string        `json:"certificate_issuer,omitempty"`
	SerialHash         []byte        `json:"certificate_serial_hash,omitempty"`
	SANs               []string      `json:"sans"`
	ValidFrom          *time.Time    `json:"valid_from,omitempty"`
	ValidUntil         *time.Time    `json:"valid_until,omitempty"`
	ExpirationDays     *int          `json:"certificate_expiration_days,omitempty"`
	HostnameValid      *bool         `json:"hostname_valid,omitempty"`
	ChainValid         bool          `json:"chain_valid"`
	Status             string        `json:"tls_status"`
	DiagnosticOnly     bool          `json:"diagnostic_only"`
	ErrorCode          netcheck.Code `json:"error_code,omitempty"`
	ErrorMessage       string        `json:"error_message,omitempty"`
	CheckedAt          time.Time     `json:"checked_at"`
}

type Inspector struct {
	DialContext      DialContext
	RootCAs          *x509.CertPool
	HandshakeTimeout time.Duration
	ExpiringSoon     time.Duration
	MinVersion       uint16
	MaxSANs          int
	Now              func() time.Time
}

func (i Inspector) Inspect(ctx context.Context, serverName, address string) Result {
	i = i.normalized()
	checkedAt := i.Now().UTC()
	result := Result{ServerName: serverName, SANs: []string{}, Status: "UNKNOWN", CheckedAt: checkedAt}

	handshakeCtx, cancel := context.WithTimeout(ctx, i.HandshakeTimeout)
	defer cancel()
	rawConnection, err := i.DialContext(handshakeCtx, "tcp", address)
	if err != nil {
		classified := netcheck.ClassifyTCP(handshakeCtx, "TLS dial", err)
		result.Status, result.ErrorCode, result.ErrorMessage = "ERROR", classified.Code, classified.Error()
		return result
	}
	remoteAddress := rawConnection.RemoteAddr().String()
	result.RemoteAddress = remoteAddress
	verifiedConnection := tls.Client(rawConnection, &tls.Config{
		ServerName: serverName,
		RootCAs:    i.RootCAs,
		MinVersion: i.MinVersion,
	})
	err = verifiedConnection.HandshakeContext(handshakeCtx)
	if err == nil {
		state := verifiedConnection.ConnectionState()
		_ = verifiedConnection.Close()
		result = FromConnectionState(state, serverName, i.ExpiringSoon, checkedAt, i.MaxSANs)
		result.RemoteAddress = remoteAddress
		result.CheckedAt = checkedAt
		return result
	}
	_ = verifiedConnection.Close()
	classified, status := classifyTLSFailure(ctx, handshakeCtx, err, checkedAt)
	result.Status, result.ErrorCode, result.ErrorMessage = status, classified.Code, classified.Error()

	diagnostic := i.diagnostic(ctx, serverName, remoteAddress, checkedAt)
	if diagnostic.Status != "UNKNOWN" {
		diagnostic.ErrorCode = result.ErrorCode
		diagnostic.ErrorMessage = result.ErrorMessage
		diagnostic.Status = result.Status
		diagnostic.ChainValid = false
		diagnostic.DiagnosticOnly = true
		return diagnostic
	}
	return result
}

func (i Inspector) diagnostic(ctx context.Context, serverName, remoteAddress string, checkedAt time.Time) Result {
	result := Result{ServerName: serverName, RemoteAddress: remoteAddress, SANs: []string{}, Status: "UNKNOWN", DiagnosticOnly: true, CheckedAt: checkedAt}
	diagnosticCtx, cancel := context.WithTimeout(ctx, i.HandshakeTimeout)
	defer cancel()
	dialer := net.Dialer{Timeout: i.HandshakeTimeout}
	rawConnection, err := dialer.DialContext(diagnosticCtx, "tcp", remoteAddress)
	if err != nil {
		return result
	}
	connection := tls.Client(rawConnection, &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
		MinVersion:         i.MinVersion,
	})
	if err := connection.HandshakeContext(diagnosticCtx); err != nil {
		_ = connection.Close()
		return result
	}
	state := connection.ConnectionState()
	_ = connection.Close()
	result = FromConnectionState(state, serverName, i.ExpiringSoon, checkedAt, i.MaxSANs)
	result.RemoteAddress = remoteAddress
	result.DiagnosticOnly = true
	result.ChainValid = false
	result.Status = "INVALID"
	return result
}

func FromConnectionState(state tls.ConnectionState, serverName string, expiringSoon time.Duration, now time.Time, maxSANs int) Result {
	if expiringSoon <= 0 {
		expiringSoon = 30 * 24 * time.Hour
	}
	if maxSANs <= 0 {
		maxSANs = 100
	}
	result := Result{
		ServerName:     serverName,
		TLSVersion:     tls.VersionName(state.Version),
		CipherSuite:    tls.CipherSuiteName(state.CipherSuite),
		SANs:           []string{},
		ChainValid:     len(state.VerifiedChains) > 0,
		Status:         "VALID",
		DiagnosticOnly: len(state.VerifiedChains) == 0,
		CheckedAt:      now.UTC(),
	}
	if len(state.PeerCertificates) == 0 {
		result.Status = "ERROR"
		result.ErrorCode = netcheck.TLSErrorHandshake
		result.ErrorMessage = "TLS peer did not present a certificate"
		return result
	}
	certificate := state.PeerCertificates[0]
	result.CertificateSubject = certificate.Subject.String()
	result.CertificateIssuer = certificate.Issuer.String()
	if certificate.SerialNumber != nil {
		serialHash := sha256.Sum256(certificate.SerialNumber.Bytes())
		result.SerialHash = serialHash[:]
	}
	validFrom, validUntil := certificate.NotBefore.UTC(), certificate.NotAfter.UTC()
	result.ValidFrom, result.ValidUntil = &validFrom, &validUntil
	days := int(math.Floor(certificate.NotAfter.Sub(now).Hours() / 24))
	result.ExpirationDays = &days
	hostnameValid := certificate.VerifyHostname(serverName) == nil
	result.HostnameValid = &hostnameValid
	result.SANs = certificateSANs(certificate, maxSANs)

	switch {
	case now.After(certificate.NotAfter):
		result.Status = "EXPIRED"
		result.ErrorCode = netcheck.TLSErrorExpired
	case !hostnameValid:
		result.Status = "HOSTNAME_MISMATCH"
		result.ErrorCode = netcheck.TLSErrorHostname
	case certificate.NotAfter.Sub(now) <= expiringSoon:
		result.Status = "EXPIRING"
	}
	return result
}

func (i Inspector) normalized() Inspector {
	if i.DialContext == nil {
		dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
		i.DialContext = dialer.DialContext
	}
	if i.HandshakeTimeout <= 0 {
		i.HandshakeTimeout = 7 * time.Second
	}
	if i.ExpiringSoon <= 0 {
		i.ExpiringSoon = 30 * 24 * time.Hour
	}
	if i.MinVersion == 0 {
		i.MinVersion = tls.VersionTLS12
	}
	if i.MaxSANs <= 0 {
		i.MaxSANs = 100
	}
	if i.Now == nil {
		i.Now = time.Now
	}
	return i
}

func classifyTLSFailure(parent, handshake context.Context, err error, now time.Time) (*netcheck.Error, string) {
	if errors.Is(parent.Err(), context.Canceled) {
		return netcheck.New(netcheck.RunCancelled, "tls", "handshake", false, context.Canceled), "ERROR"
	}
	if errors.Is(parent.Err(), context.DeadlineExceeded) {
		return netcheck.New(netcheck.RunDeadlineExceeded, "tls", "handshake", false, context.DeadlineExceeded), "ERROR"
	}
	if errors.Is(handshake.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return netcheck.New(netcheck.TLSErrorHandshake, "tls", "handshake", true, context.DeadlineExceeded), "ERROR"
	}
	var hostnameError x509.HostnameError
	if errors.As(err, &hostnameError) {
		return netcheck.New(netcheck.TLSErrorHostname, "tls", "verify hostname", false, err), "HOSTNAME_MISMATCH"
	}
	var invalidError x509.CertificateInvalidError
	if errors.As(err, &invalidError) {
		if invalidError.Reason == x509.Expired && invalidError.Cert != nil && now.After(invalidError.Cert.NotAfter) {
			return netcheck.New(netcheck.TLSErrorExpired, "tls", "verify certificate", false, err), "EXPIRED"
		}
		return netcheck.New(netcheck.TLSErrorHandshake, "tls", "verify certificate", false, err), "INVALID"
	}
	var authorityError x509.UnknownAuthorityError
	if errors.As(err, &authorityError) {
		return netcheck.New(netcheck.TLSErrorUnknownAuthority, "tls", "verify chain", false, err), "INVALID"
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return netcheck.New(netcheck.TLSErrorHandshake, "tls", "handshake", true, err), "ERROR"
	}
	return netcheck.New(netcheck.TLSErrorHandshake, "tls", "handshake", true, err), "ERROR"
}

func certificateSANs(certificate *x509.Certificate, max int) []string {
	values := make([]string, 0, len(certificate.DNSNames)+len(certificate.IPAddresses))
	for _, name := range certificate.DNSNames {
		values = append(values, strings.ToLower(name))
		if len(values) >= max {
			return values
		}
	}
	for _, address := range certificate.IPAddresses {
		values = append(values, address.String())
		if len(values) >= max {
			return values
		}
	}
	return values
}

func (r Result) Error() error {
	if r.ErrorCode == "" {
		return nil
	}
	return netcheck.New(r.ErrorCode, "tls", "inspect", false, fmt.Errorf("%s", r.ErrorMessage))
}
