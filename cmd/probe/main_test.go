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

func TestValidateJobAcceptsCanonicalPublicDomainsContainingPreviouslyRejectedLetters(t *testing.T) {
	for _, domain := range []string{"123bet.asia", "riches888.games", "riches888.ltd", "example.net"} {
		t.Run(domain, func(t *testing.T) {
			job := validJob()
			job.Target.DomainASCII = domain
			if err := validateJob(job); err != nil {
				t.Fatalf("valid public domain %q rejected: %v", domain, err)
			}
		})
	}
}

func TestValidateJobRejectsURLAndWhitespaceTargets(t *testing.T) {
	for _, domain := range []string{"https://example.com", "example.com/path", "example.com\t", "example.com\r", "example.com\n"} {
		t.Run(domain, func(t *testing.T) {
			job := validJob()
			job.Target.DomainASCII = domain
			if err := validateJob(job); err == nil {
				t.Fatalf("invalid target %q was accepted", domain)
			}
		})
	}
}

func validJob() probeprotocol.Job {
	return probeprotocol.Job{
		JobID: uuid.New(), RunID: uuid.New(), Target: probeprotocol.Target{DomainASCII: "example.com", Schemes: []string{"https", "http"}, Ports: []int{443, 80}},
		PolicyVersion: "test", Policy: probeprotocol.Policy{DeadlineMS: 45000, MaxRedirects: 10, MaxBodyBytes: 2 << 20, StoreExcerptBytes: 32 << 10},
		IssuedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(2 * time.Minute), Nonce: "nonce",
	}
}
