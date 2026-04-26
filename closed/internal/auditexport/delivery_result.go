package auditexport

import "time"

type DeliveryResult struct {
	Outcome    AttemptOutcome
	StatusCode *int
	Error      string
	Latency    time.Duration
}
