package testsupport

import (
	"errors"
	"sync"
	"time"
)

// ManualClock provides deterministic time control for tests.
type ManualClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewManualClock(now time.Time) *ManualClock {
	return &ManualClock{now: now}
}

func (c *ManualClock) Now() time.Time {
	if c == nil {
		return time.Now().UTC()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *ManualClock) Advance(d time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func (c *ManualClock) Set(t time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
}

// ErrorInjector returns a deterministic error on configured call counts.
type ErrorInjector struct {
	mu       sync.Mutex
	failures map[string]int
	calls    map[string]int
	err      error
}

func NewErrorInjector(err error) *ErrorInjector {
	if err == nil {
		err = errors.New("injected failure")
	}
	return &ErrorInjector{
		failures: map[string]int{},
		calls:    map[string]int{},
		err:      err,
	}
}

// FailOn configures an error on the Nth call for the given key (1-based).
func (e *ErrorInjector) FailOn(key string, call int) {
	if e == nil || call <= 0 {
		return
	}
	e.mu.Lock()
	e.failures[key] = call
	e.mu.Unlock()
}

// Check increments the call count for key and returns an error on the configured call.
func (e *ErrorInjector) Check(key string) error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls[key]++
	if target, ok := e.failures[key]; ok && target == e.calls[key] {
		return e.err
	}
	return nil
}

func (e *ErrorInjector) Count(key string) int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.calls[key]
}
