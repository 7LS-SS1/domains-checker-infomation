package domain

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

type Normalized struct {
	OriginalInput     string
	ASCII             string
	Unicode           string
	RegistrableDomain string
}

type Normalizer struct {
	AllowUnknownTLD bool
}

func (n Normalizer) Normalize(input string) (Normalized, error) {
	original := strings.TrimSpace(input)
	if original == "" {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "value is required"}
	}
	for _, character := range original {
		if unicode.IsControl(character) {
			return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "control characters are not allowed"}
		}
	}

	host, err := extractHost(original)
	if err != nil {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: err.Error()}
	}
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if host == "" {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "hostname is empty"}
	}
	if net.ParseIP(host) != nil {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "IP literals are not domains"}
	}

	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "IDNA conversion failed"}
	}
	ascii = strings.ToLower(strings.TrimSuffix(ascii, "."))
	if err := validateASCIIName(ascii); err != nil {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: err.Error()}
	}

	suffix, icann := publicsuffix.PublicSuffix(ascii)
	if suffix == "" || ascii == suffix {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "a registrable domain is required"}
	}
	if !icann && !n.AllowUnknownTLD {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "unknown or private public suffix"}
	}
	registrable, err := publicsuffix.EffectiveTLDPlusOne(ascii)
	if err != nil {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "cannot determine registrable domain"}
	}
	display, err := idna.Lookup.ToUnicode(ascii)
	if err != nil {
		return Normalized{}, &ValidationError{Field: "domain", ReasonCode: "DOMAIN_INVALID", Reason: "cannot produce Unicode display form"}
	}

	return Normalized{
		OriginalInput:     original,
		ASCII:             ascii,
		Unicode:           display,
		RegistrableDomain: registrable,
	}, nil
}

func extractHost(input string) (string, error) {
	raw := input
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("cannot parse input")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("only http and https URLs are accepted")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("URL credentials are not allowed")
	}
	if parsed.Port() != "" {
		return "", fmt.Errorf("custom ports are not allowed")
	}
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("hostname is missing")
	}
	return parsed.Hostname(), nil
}

func validateASCIIName(name string) error {
	if len(name) > 253 {
		return fmt.Errorf("domain exceeds 253 bytes")
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return fmt.Errorf("domain must contain a public suffix")
	}
	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("domain contains an empty label")
		}
		if len(label) > 63 {
			return fmt.Errorf("domain label exceeds 63 bytes")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("domain label cannot start or end with a hyphen")
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return fmt.Errorf("domain contains an invalid character")
			}
		}
	}
	return nil
}
