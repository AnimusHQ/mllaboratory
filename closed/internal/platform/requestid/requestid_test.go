package requestid

import (
	"encoding/hex"
	"testing"
)

func TestNew(t *testing.T) {
	id, err := New()
	if err != nil {
		t.Fatalf("New() err=%v", err)
	}
	if len(id) != 32 {
		t.Fatalf("New() len=%d, want 32", len(id))
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Fatalf("New()=%q not hex: %v", id, err)
	}
}
