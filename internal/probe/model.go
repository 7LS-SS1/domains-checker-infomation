package probe

import (
	"crypto/ed25519"
	"encoding/json"
	"time"

	"domainmonitor/internal/probeprotocol"
	"github.com/google/uuid"
)

type Principal struct {
	TokenID      uuid.UUID
	ProbeID      uuid.UUID
	Name         string
	RegionCode   string
	CountryCode  string
	NetworkName  string
	AgentVersion string
	Status       string
	PublicKey    ed25519.PublicKey
}

type Node struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	RegionCode    string          `json:"region_code"`
	CountryCode   string          `json:"country_code"`
	NetworkName   *string         `json:"network_name,omitempty"`
	Status        string          `json:"status"`
	Version       string          `json:"version"`
	Capabilities  json.RawMessage `json:"capabilities"`
	LastSeenAt    *time.Time      `json:"last_seen_at,omitempty"`
	ClockOffsetMS *int            `json:"clock_offset_ms,omitempty"`
	RegisteredAt  time.Time       `json:"registered_at"`
	RevokedAt     *time.Time      `json:"revoked_at,omitempty"`
}

type RegistrationToken struct {
	ID        uuid.UUID `json:"id"`
	Token     string    `json:"token"`
	Name      string    `json:"name"`
	Region    string    `json:"region_code"`
	Country   string    `json:"country_code"`
	Network   string    `json:"network_name,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

type RegistrationSpec struct {
	Name, Region, Country, Network string
	TTL                            time.Duration
	CreatedBy                      uuid.UUID
	RequestID                      string
}

type ClaimedJob struct {
	Job probeprotocol.Job
}
