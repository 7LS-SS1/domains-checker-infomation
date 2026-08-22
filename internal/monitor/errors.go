package monitor

import "errors"

var (
	ErrRunNotFound           = errors.New("monitoring run not found")
	ErrRunCompleted          = errors.New("monitoring run already completed")
	ErrRunBusy               = errors.New("monitoring run is owned by an active worker")
	ErrRunExpired            = errors.New("monitoring run deadline expired")
	ErrDomainNotFound        = errors.New("domain not found")
	ErrDomainInactive        = errors.New("domain is not active")
	ErrInvalidIdempotencyKey = errors.New("invalid idempotency key")
	ErrInvalidIncidentStatus = errors.New("invalid incident status")
)

const JobEventType = "monitor.run.requested"

type Job struct {
	RunID string `json:"run_id"`
}
