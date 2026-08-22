package retry

import (
	"context"
	rand "math/rand/v2"
	"time"
)

type Jitter func(max time.Duration) time.Duration

type Policy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      Jitter
}

func (p Policy) Normalized() Policy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 3
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 100 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 2 * time.Second
	}
	if p.MaxDelay < p.BaseDelay {
		p.MaxDelay = p.BaseDelay
	}
	if p.Jitter == nil {
		p.Jitter = fullJitter
	}
	return p
}

func (p Policy) Delay(afterAttempt int) time.Duration {
	p = p.Normalized()
	if afterAttempt < 1 {
		afterAttempt = 1
	}
	exponent := afterAttempt - 1
	if exponent > 30 {
		exponent = 30
	}
	factor := time.Duration(uint64(1) << exponent)
	ceiling := p.MaxDelay
	if p.BaseDelay <= p.MaxDelay/factor {
		ceiling = p.BaseDelay * factor
	}
	if ceiling > p.MaxDelay {
		ceiling = p.MaxDelay
	}
	return p.Jitter(ceiling)
}

func (p Policy) Wait(ctx context.Context, afterAttempt int) error {
	delay := p.Delay(afterAttempt)
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func fullJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(max) + 1))
}
