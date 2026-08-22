package i18n

import "testing"

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		input string
		want  Locale
	}{
		{"th-TH,th;q=0.9,en;q=0.8", Thai},
		{"en-US,en;q=0.9", English},
		{"ja,en;q=0.5", English},
		{"ja", Thai},
	}
	for _, test := range tests {
		if got := Parse(test.input, Thai); got != test.want {
			t.Errorf("Parse(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestCatalogHasBothLanguages(t *testing.T) {
	for code, translation := range catalog {
		if translation.TH == "" || translation.EN == "" {
			t.Errorf("catalog entry %s is incomplete", code)
		}
	}
}
