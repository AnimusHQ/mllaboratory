package testutil

import (
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

const defaultSeed int64 = 42

func DeterministicSeed() int64 {
	if raw := os.Getenv("ANIMUS_TEST_SEED"); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return value
		}
	}
	return defaultSeed
}

func NewRand(t *testing.T) *rand.Rand {
	t.Helper()
	seed := DeterministicSeed()
	return rand.New(rand.NewSource(seed))
}

type FixedClock struct {
	now time.Time
}

func NewFixedClock(now time.Time) FixedClock {
	return FixedClock{now: now.UTC()}
}

func (c FixedClock) Now() time.Time {
	return c.now
}
