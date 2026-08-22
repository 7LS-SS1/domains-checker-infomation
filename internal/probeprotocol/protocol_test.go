package probeprotocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSignedResultDetectsTampering(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := ResultPayload{
		ProtocolVersion: Version, ProbeID: uuid.New(), JobID: uuid.New(), RunID: uuid.New(),
		Nonce: "test-nonce", StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(),
	}
	envelope, err := SignResult(privateKey, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyResult(publicKey, envelope); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	envelope.Payload[0] ^= 1
	if _, _, err := VerifyResult(publicKey, envelope); err == nil {
		t.Fatal("tampered payload was accepted")
	}
}

func TestSecretHashRoundTrip(t *testing.T) {
	secret, expected, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	actual, err := HashSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !EqualHash(expected[:], actual[:]) {
		t.Fatal("secret hashes differ")
	}
}
