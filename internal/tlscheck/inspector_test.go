package tlscheck

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"domainmonitor/internal/netcheck"
)

func TestInspectorValidExpiredAndHostnameMismatch(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		serverName string
		notBefore  time.Time
		notAfter   time.Time
		wantStatus string
		wantCode   netcheck.Code
		diagnostic bool
	}{
		{name: "valid", serverName: "valid.test", notBefore: now.Add(-time.Hour), notAfter: now.Add(90 * 24 * time.Hour), wantStatus: "VALID"},
		{name: "expiring", serverName: "expiring.test", notBefore: now.Add(-time.Hour), notAfter: now.Add(7 * 24 * time.Hour), wantStatus: "EXPIRING"},
		{name: "expired", serverName: "expired.test", notBefore: now.Add(-48 * time.Hour), notAfter: now.Add(-time.Hour), wantStatus: "EXPIRED", wantCode: netcheck.TLSErrorExpired, diagnostic: true},
		{name: "hostname", serverName: "wrong.test", notBefore: now.Add(-time.Hour), notAfter: now.Add(90 * 24 * time.Hour), wantStatus: "HOSTNAME_MISMATCH", wantCode: netcheck.TLSErrorHostname, diagnostic: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			certificateName := test.serverName
			if test.name == "hostname" {
				certificateName = "different.test"
			}
			server, roots := newTLSServer(t, certificateName, test.notBefore, test.notAfter)
			parsed, _ := url.Parse(server.URL)
			result := (Inspector{RootCAs: roots, Now: func() time.Time { return now }}).Inspect(context.Background(), test.serverName, parsed.Host)
			if result.Status != test.wantStatus || result.ErrorCode != test.wantCode || result.DiagnosticOnly != test.diagnostic {
				t.Fatalf("result = %#v", result)
			}
			if result.CertificateSubject == "" || result.ValidUntil == nil || len(result.SerialHash) != sha256.Size {
				t.Fatalf("certificate evidence incomplete: %#v", result)
			}
		})
	}
}

func TestInspectorClassifiesHandshakeFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			_ = connection.Close()
		}
	}()
	result := (Inspector{HandshakeTimeout: time.Second}).Inspect(context.Background(), "failure.test", listener.Addr().String())
	_ = listener.Close()
	<-done
	if result.ErrorCode != netcheck.TLSErrorHandshake || result.Status != "ERROR" {
		t.Fatalf("result = %#v", result)
	}
}

func TestInspectorUnknownAuthority(t *testing.T) {
	now := time.Now().UTC()
	server, _ := newTLSServer(t, "unknown.test", now.Add(-time.Hour), now.Add(24*time.Hour))
	parsed, _ := url.Parse(server.URL)
	result := (Inspector{RootCAs: x509.NewCertPool(), Now: func() time.Time { return now }}).Inspect(context.Background(), "unknown.test", parsed.Host)
	if result.ErrorCode != netcheck.TLSErrorUnknownAuthority || !result.DiagnosticOnly || result.CertificateIssuer == "" {
		t.Fatalf("result = %#v", result)
	}
}

func newTLSServer(t *testing.T, dnsName string, notBefore, notAfter time.Time) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Phase 2 Test CA"},
		NotBefore: notBefore.Add(-time.Hour), NotAfter: notAfter.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: dnsName}, DNSNames: []string{dnsName},
		NotBefore: notBefore, NotAfter: notAfter, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := append(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})...,
	)
	certificate, err := tls.X509KeyPair(
		certificatePEM,
		pem.EncodeToMemory(marshalECPrivateKey(t, leafKey)),
	)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return server, roots
}

func marshalECPrivateKey(t *testing.T, key *ecdsa.PrivateKey) *pem.Block {
	t.Helper()
	encoded, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return &pem.Block{Type: "EC PRIVATE KEY", Bytes: encoded}
}
