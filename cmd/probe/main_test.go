package main

import (
	"testing"
	"time"

	"domainmonitor/internal/probeprotocol"
	"github.com/google/uuid"
)

func TestValidateJobRejectsArbitraryPorts(t *testing.T) {
	job := validJob()
	job.Target.Ports = []int{443, 8080}
	if err := validateJob(job); err == nil {
		t.Fatal("arbitrary port was accepted")
	}
}

func TestValidateJobAcceptsBoundedDomainContract(t *testing.T) {
	if err := validateJob(validJob()); err != nil {
		t.Fatalf("valid job rejected: %v", err)
	}
}

func validJob() probeprotocol.Job {
	return probeprotocol.Job{
		JobID: uuid.New(), RunID: uuid.New(), Target: probeprotocol.Target{DomainASCII: "example.com", Schemes: []string{"https", "http"}, Ports: []int{443, 80}},
		PolicyVersion: "test", Policy: probeprotocol.Policy{DeadlineMS: 45000, MaxRedirects: 10, MaxBodyBytes: 2 << 20, StoreExcerptBytes: 32 << 10},
		IssuedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(2 * time.Minute), Nonce: "nonce",
	}
}
