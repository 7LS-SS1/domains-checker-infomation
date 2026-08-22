package rdap

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResolveBaseAndNormalizeMissingFields(t *testing.T) {
	base, err := ResolveBase([]byte(`{"services":[[["com","net"],["https://rdap.example/"]]]}`), "com")
	if err != nil || base != "https://rdap.example" {
		t.Fatalf("base=%q err=%v", base, err)
	}
	result, err := Normalize([]byte(`{"objectClassName":"domain","status":["active"],"nameservers":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.RegistrarName != nil || result.ExpirationAt != nil || len(result.Nameservers) != 0 || len(result.Statuses) != 1 {
		t.Fatalf("missing fields were invented: %#v", result)
	}
}

func TestNormalizeRegistrarEventsAndDNSSEC(t *testing.T) {
	payload := `{"events":[{"eventAction":"registration","eventDate":"2020-01-02T00:00:00Z"},{"eventAction":"expiration","eventDate":"2027-01-02T00:00:00Z"}],"nameservers":[{"ldhName":"NS2.EXAMPLE."},{"ldhName":"ns1.example."}],"secureDNS":{"delegationSigned":true},"entities":[{"roles":["registrar"],"vcardArray":["vcard",[["fn",{},"text","Example Registrar"]]],"publicIds":[{"type":"IANA Registrar ID","identifier":"123"}]}]}`
	result, err := Normalize([]byte(payload))
	if err != nil || result.RegistrarName == nil || *result.RegistrarName != "Example Registrar" || result.RegistrarIANA == nil || *result.RegistrarIANA != 123 || result.DNSSEC == nil || !*result.DNSSEC || result.Nameservers[0] != "ns1.example" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestClientHandles429AndUnavailable(t *testing.T) {
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := NewClient(server.Client(), Config{BootstrapURL: server.URL, MinInterval: time.Nanosecond, RetryMaxDelay: time.Millisecond})
	_, err := client.Bootstrap(t.Context(), "", "")
	if !errors.Is(err, ErrRateLimited) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}

	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	_, err = client.Bootstrap(t.Context(), "", "")
	if !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err=%v", err)
	}
}
