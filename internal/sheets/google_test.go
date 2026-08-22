package sheets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGoogleClientUsesValuesGetAndPreservesDecimalLexeme(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || !strings.Contains(request.RequestURI, "/v4/spreadsheets/sheet-id/values/Domains%21A:K") {
			t.Errorf("unexpected request: %s %s", request.Method, request.RequestURI)
		}
		if request.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("missing authorization header")
		}
		w.Header().Set("ETag", `"revision-1"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"values":[["domain","renewal_price"],["example.com",1234567890.123456]]}`)
	}))
	defer server.Close()
	client := NewGoogleClient(server.Client(), GoogleClientConfig{APIBase: server.URL, AccessToken: "fixture-token", Timeout: time.Second})
	snapshot, err := client.Fetch(t.Context(), Config{SpreadsheetID: "sheet-id", SheetName: "Domains", Range: "A:K"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision != `"revision-1"` || snapshot.Values[1][1] != "1234567890.123456" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}
