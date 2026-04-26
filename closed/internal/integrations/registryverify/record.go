package registryverify

import (
	"encoding/json"
	"time"
)

type Record struct {
	ID             int64
	ProjectID      string
	ImageDigestRef string
	PolicyMode     string
	Provider       string
	Status         Status
	Signed         bool
	Verified       bool
	FailureReason  string
	Details        json.RawMessage
	CreatedAt      time.Time
	VerifiedAt     *time.Time
}
