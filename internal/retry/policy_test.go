package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPolicyUsesExponentialCeilingAndInjectedJitter(t *testing.T) {
	policy := Policy{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    250 * time.Millisecond,
		Jitter:      func(max time.Duration) time.Duration { return max },
	}
	if got := policy.Delay(1); got != 100*time.Millisecond {
		t.Fatalf("Delay(1) = %s", got)
	}
	if got := policy.Delay(2); got != 200*time.Millisecond {
		t.Fatalf("Delay(2) = %s", got)
	}
	if got := policy.Delay(3); got != 250*time.Millisecond {
		t.Fatalf("Delay(3) = %s", got)
	}
}

func TestWaitPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	policy := Policy{Jitter: func(time.Duration) time.Duration { return time.Second }}
	if err := policy.Wait(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() error = %v", err)
	}
}
