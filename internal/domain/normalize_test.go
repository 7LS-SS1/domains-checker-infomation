package domain

import "testing"

func TestNormalize(t *testing.T) {
	normalizer := Normalizer{}
	tests := []struct {
		name            string
		input           string
		wantASCII       string
		wantRegistrable string
		wantErr         bool
	}{
		{name: "plain", input: "Example.COM", wantASCII: "example.com", wantRegistrable: "example.com"},
		{name: "url", input: "https://www.example.com/path?q=1", wantASCII: "www.example.com", wantRegistrable: "example.com"},
		{name: "trailing slash", input: "www.example.com/", wantASCII: "www.example.com", wantRegistrable: "example.com"},
		{name: "thai idn", input: "https://ภาษาไทย.com/", wantASCII: "xn--o3crh0a8bb0k.com", wantRegistrable: "xn--o3crh0a8bb0k.com"},
		{name: "ip", input: "127.0.0.1", wantErr: true},
		{name: "public suffix", input: "com", wantErr: true},
		{name: "credentials", input: "https://user:pass@example.com", wantErr: true},
		{name: "port", input: "https://example.com:8443", wantErr: true},
		{name: "unknown suffix", input: "example.invalidtld", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizer.Normalize(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("Normalize(%q) expected an error", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Normalize(%q) error = %v", test.input, err)
			}
			if got.ASCII != test.wantASCII || got.RegistrableDomain != test.wantRegistrable {
				t.Fatalf("Normalize(%q) = %#v", test.input, got)
			}
		})
	}
}

func TestNormalizePreservesWWW(t *testing.T) {
	got, err := (Normalizer{}).Normalize("www.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.ASCII != "www.example.com" {
		t.Fatalf("ASCII = %q", got.ASCII)
	}
}
