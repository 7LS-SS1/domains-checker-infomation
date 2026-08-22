package probeprotocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"domainmonitor/internal/dnscheck"
	"domainmonitor/internal/httpcheck"
	"github.com/google/uuid"
)

const Version = "domain-monitor-probe.v1"

type RegisterRequest struct {
	RegistrationToken string         `json:"registration_token"`
	Name              string         `json:"name"`
	RegionCode        string         `json:"region_code"`
	CountryCode       string         `json:"country_code"`
	NetworkName       string         `json:"network_name,omitempty"`
	PublicKey         string         `json:"public_key"`
	ProtocolVersion   string         `json:"protocol_version"`
	AgentVersion      string         `json:"agent_version"`
	Capabilities      map[string]any `json:"capabilities,omitempty"`
}

type RegisterResponse struct {
	ProbeID         uuid.UUID `json:"probe_id"`
	ProtocolVersion string    `json:"protocol_version"`
	RegisteredAt    time.Time `json:"registered_at"`
}

type TokenRequest struct {
	ProbeID     uuid.UUID `json:"probe_id"`
	ChallengeID uuid.UUID `json:"challenge_id,omitempty"`
	Signature   string    `json:"signature,omitempty"`
}

type TokenChallenge struct {
	ChallengeID    uuid.UUID `json:"challenge_id"`
	Challenge      string    `json:"challenge"`
	SigningMessage string    `json:"signing_message"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	Scope       []string  `json:"scope"`
}

type HeartbeatRequest struct {
	ProtocolVersion string         `json:"protocol_version"`
	AgentVersion    string         `json:"agent_version"`
	ClockOffsetMS   int            `json:"clock_offset_ms"`
	QueueCapacity   int            `json:"queue_capacity"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
}

type HeartbeatResponse struct {
	Status      string    `json:"status"`
	ServerTime  time.Time `json:"server_time"`
	NextAfterMS int64     `json:"next_after_ms"`
}

type ClaimRequest struct {
	MaxWaitMS int64 `json:"max_wait_ms"`
}

type Target struct {
	DomainASCII string   `json:"domain_ascii"`
	Schemes     []string `json:"schemes"`
	Ports       []int    `json:"ports"`
}

type Policy struct {
	DeadlineMS        int64 `json:"deadline_ms"`
	MaxRedirects      int   `json:"max_redirects"`
	MaxBodyBytes      int64 `json:"max_body_bytes"`
	StoreExcerptBytes int   `json:"store_excerpt_bytes"`
}

type Job struct {
	JobID         uuid.UUID `json:"job_id"`
	RunID         uuid.UUID `json:"run_id"`
	Target        Target    `json:"target"`
	PolicyVersion string    `json:"policy_version"`
	Policy        Policy    `json:"policy"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Nonce         string    `json:"nonce"`
}

type ResultPayload struct {
	ProtocolVersion string                 `json:"protocol_version"`
	ProbeID         uuid.UUID              `json:"probe_id"`
	AgentVersion    string                 `json:"agent_version"`
	RegionCode      string                 `json:"region_code"`
	CountryCode     string                 `json:"country_code"`
	NetworkName     string                 `json:"network_name,omitempty"`
	JobID           uuid.UUID              `json:"job_id"`
	RunID           uuid.UUID              `json:"run_id"`
	Nonce           string                 `json:"nonce"`
	StartedAt       time.Time              `json:"started_at"`
	FinishedAt      time.Time              `json:"finished_at"`
	ClockOffsetMS   int                    `json:"clock_offset_ms"`
	DNS             []dnscheck.Result      `json:"dns"`
	HTTP            httpcheck.OriginResult `json:"http"`
}

type ResultEnvelope struct {
	Payload       json.RawMessage `json:"payload"`
	PayloadSHA256 string          `json:"payload_sha256"`
	Signature     string          `json:"signature"`
}

func NewSecret() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate random secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), sha256.Sum256(raw[:]), nil
}

func HashSecret(encoded string) ([32]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(raw) != 32 {
		return [32]byte{}, errors.New("secret must be 32 bytes of base64url data")
	}
	return sha256.Sum256(raw), nil
}

func EqualHash(left, right []byte) bool {
	return len(left) == sha256.Size && len(right) == sha256.Size && subtle.ConstantTimeCompare(left, right) == 1
}

func DecodePublicKey(encoded string) (ed25519.PublicKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("public_key must be an Ed25519 public key encoded as base64url")
	}
	return ed25519.PublicKey(raw), nil
}

func EncodePublicKey(key ed25519.PublicKey) string {
	return base64.RawURLEncoding.EncodeToString(key)
}

func EncodeSignature(signature []byte) string {
	return base64.RawURLEncoding.EncodeToString(signature)
}

func DecodeSignature(encoded string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(raw) != ed25519.SignatureSize {
		return nil, errors.New("signature must be an Ed25519 signature encoded as base64url")
	}
	return raw, nil
}

func TokenSigningMessage(challengeID uuid.UUID, challenge string, expiresAt time.Time) string {
	return fmt.Sprintf("%s\ntoken\n%s\n%s\n%s", Version, challengeID, challenge, expiresAt.UTC().Format(time.RFC3339Nano))
}

func ResultSigningMessage(payloadHash [32]byte, nonce string) string {
	return fmt.Sprintf("%s\nresult\n%s\n%s", Version, hex.EncodeToString(payloadHash[:]), nonce)
}

func SignResult(privateKey ed25519.PrivateKey, payload ResultPayload) (ResultEnvelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return ResultEnvelope{}, fmt.Errorf("marshal result payload: %w", err)
	}
	hash := sha256.Sum256(raw)
	signature := ed25519.Sign(privateKey, []byte(ResultSigningMessage(hash, payload.Nonce)))
	return ResultEnvelope{Payload: raw, PayloadSHA256: hex.EncodeToString(hash[:]), Signature: EncodeSignature(signature)}, nil
}

func VerifyResult(publicKey ed25519.PublicKey, envelope ResultEnvelope) (ResultPayload, [32]byte, error) {
	if len(envelope.Payload) == 0 {
		return ResultPayload{}, [32]byte{}, errors.New("result payload is required")
	}
	computed := sha256.Sum256(envelope.Payload)
	claimed, err := hex.DecodeString(strings.TrimSpace(envelope.PayloadSHA256))
	if err != nil || len(claimed) != sha256.Size || !EqualHash(computed[:], claimed) {
		return ResultPayload{}, [32]byte{}, errors.New("payload hash does not match")
	}
	var payload ResultPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return ResultPayload{}, [32]byte{}, fmt.Errorf("decode result payload: %w", err)
	}
	signature, err := DecodeSignature(envelope.Signature)
	if err != nil {
		return ResultPayload{}, [32]byte{}, err
	}
	if !ed25519.Verify(publicKey, []byte(ResultSigningMessage(computed, payload.Nonce)), signature) {
		return ResultPayload{}, [32]byte{}, errors.New("result signature is invalid")
	}
	return payload, computed, nil
}
