package sheets

import (
	"testing"

	"domainmonitor/internal/domain"
)

func TestParseRowsMappingValidationAndDuplicates(t *testing.T) {
	snapshot := Snapshot{Values: [][]string{
		{"Domain Name", "Renew", "Currency", "Active"},
		{"Example.COM", "100.25", "thb", "yes"},
		{"example.com", "100.25", "THB", "true"},
		{"bad domain", "x", "", "maybe"},
	}}
	rows, mapping, err := ParseRows(snapshot, map[string]string{"domain": "Domain Name", "renewal_price": "Renew"}, domain.Normalizer{AllowUnknownTLD: true})
	if err != nil {
		t.Fatal(err)
	}
	if mapping["domain"] != "Domain Name" || len(rows) != 3 {
		t.Fatalf("mapping=%#v rows=%#v", mapping, rows)
	}
	if rows[0].Valid || rows[1].Valid || rows[0].ValidationErrors[0] != "DUPLICATE_DOMAIN:example.com" {
		t.Fatalf("duplicates not rejected: %#v %#v", rows[0], rows[1])
	}
	if rows[2].Valid || len(rows[2].ValidationErrors) < 2 {
		t.Fatalf("invalid row accepted: %#v", rows[2])
	}
}

func TestParseRowsRejectsAmbiguousHeaders(t *testing.T) {
	_, _, err := ParseRows(Snapshot{Values: [][]string{{"domain", "Domain"}}}, nil, domain.Normalizer{AllowUnknownTLD: true})
	if err == nil {
		t.Fatal("expected ambiguous mapping error")
	}
}
