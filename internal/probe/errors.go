package probe

import "errors"

var (
	ErrUnauthorized    = errors.New("probe authentication failed")
	ErrForbidden       = errors.New("probe is revoked or incompatible")
	ErrInvalidRequest  = errors.New("probe request is invalid")
	ErrExpired         = errors.New("probe credential or job expired")
	ErrReplay          = errors.New("probe nonce or result was already used")
	ErrNotFound        = errors.New("probe resource was not found")
	ErrConflict        = errors.New("probe resource conflicts with current state")
	ErrNoJob           = errors.New("no remote probe job is available")
	ErrPayloadTooLarge = errors.New("probe result payload is too large")
	ErrClockSkew       = errors.New("probe clock skew exceeds policy")
	ErrSignature       = errors.New("probe signature is invalid")
)
