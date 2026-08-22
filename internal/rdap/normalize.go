package rdap

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type Normalized struct {
	RegistrarName  *string    `json:"registrar_name,omitempty"`
	RegistrarIANA  *int64     `json:"registrar_iana_id,omitempty"`
	RegistrationAt *time.Time `json:"registration_at,omitempty"`
	ExpirationAt   *time.Time `json:"expiration_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	Nameservers    []string   `json:"nameservers"`
	DNSSEC         *bool      `json:"dnssec,omitempty"`
	Statuses       []string   `json:"statuses"`
}

type rdapDocument struct {
	Status []string `json:"status"`
	Events []struct {
		Action string `json:"eventAction"`
		Date   string `json:"eventDate"`
	} `json:"events"`
	Nameservers []struct {
		LDHName     string `json:"ldhName"`
		UnicodeName string `json:"unicodeName"`
	} `json:"nameservers"`
	SecureDNS *struct {
		DelegationSigned *bool `json:"delegationSigned"`
	} `json:"secureDNS"`
	Entities []struct {
		Roles     []string          `json:"roles"`
		VCard     []json.RawMessage `json:"vcardArray"`
		PublicIDs []struct {
			Type       string `json:"type"`
			Identifier string `json:"identifier"`
		} `json:"publicIds"`
	} `json:"entities"`
}

func Normalize(payload []byte) (Normalized, error) {
	var document rdapDocument
	if err := json.Unmarshal(payload, &document); err != nil {
		return Normalized{}, err
	}
	result := Normalized{Nameservers: []string{}, Statuses: uniqueSorted(document.Status)}
	for _, event := range document.Events {
		parsed, err := time.Parse(time.RFC3339, event.Date)
		if err != nil {
			continue
		}
		parsed = parsed.UTC()
		switch strings.ToLower(event.Action) {
		case "registration":
			result.RegistrationAt = &parsed
		case "expiration":
			result.ExpirationAt = &parsed
		case "last changed", "last update of rdap database":
			if result.UpdatedAt == nil || parsed.After(*result.UpdatedAt) {
				result.UpdatedAt = &parsed
			}
		}
	}
	for _, nameserver := range document.Nameservers {
		name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(nameserver.LDHName), "."))
		if name == "" {
			name = strings.TrimSuffix(strings.TrimSpace(nameserver.UnicodeName), ".")
		}
		if name != "" {
			result.Nameservers = append(result.Nameservers, name)
		}
	}
	result.Nameservers = uniqueSorted(result.Nameservers)
	if document.SecureDNS != nil {
		result.DNSSEC = document.SecureDNS.DelegationSigned
	}
	for _, entity := range document.Entities {
		if !containsFold(entity.Roles, "registrar") {
			continue
		}
		if name := vcardName(entity.VCard); name != "" {
			result.RegistrarName = &name
		}
		for _, publicID := range entity.PublicIDs {
			if strings.Contains(strings.ToLower(publicID.Type), "iana registrar") {
				if identifier, err := strconv.ParseInt(strings.TrimSpace(publicID.Identifier), 10, 64); err == nil && identifier > 0 {
					result.RegistrarIANA = &identifier
				}
			}
		}
		break
	}
	return result, nil
}

func vcardName(raw []json.RawMessage) string {
	if len(raw) != 2 {
		return ""
	}
	var properties []json.RawMessage
	if json.Unmarshal(raw[1], &properties) != nil {
		return ""
	}
	for _, propertyRaw := range properties {
		var property []json.RawMessage
		if json.Unmarshal(propertyRaw, &property) != nil || len(property) < 4 {
			continue
		}
		var key, value string
		if json.Unmarshal(property[0], &key) == nil && strings.EqualFold(key, "fn") && json.Unmarshal(property[3], &value) == nil {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(value, wanted) {
			return true
		}
	}
	return false
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	// RDAP arrays are small; stable insertion sorting keeps this helper dependency-free.
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j] < result[j-1]; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}
