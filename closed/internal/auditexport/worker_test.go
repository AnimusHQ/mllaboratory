package auditexport

import (
	"testing"
	"time"
)

func TestBackoffDelay(t *testing.T) {
	base := 5 * time.Second
	max := 1 * time.Minute

	if got := backoffDelay(1, base, max); got != 5*time.Second {
		t.Fatalf("attempt 1 backoff = %v", got)
	}
	if got := backoffDelay(2, base, max); got != 10*time.Second {
		t.Fatalf("attempt 2 backoff = %v", got)
	}
	if got := backoffDelay(3, base, max); got != 20*time.Second {
		t.Fatalf("attempt 3 backoff = %v", got)
	}
	if got := backoffDelay(5, base, max); got != max {
		t.Fatalf("attempt 5 backoff = %v", got)
	}

	if got := backoffDelay(1, 90*time.Second, max); got != max {
		t.Fatalf("expected base capped to max, got %v", got)
	}
}
