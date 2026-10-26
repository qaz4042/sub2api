package usagestats

import "time"

// AccountScheduledTestStats contains counters only: test results have no usage or cost data.
type AccountScheduledTestStats struct {
	TotalRequests int64                        `json:"total_requests"`
	ErrorCount    int64                        `json:"error_count"`
	CompleteSince time.Time                    `json:"complete_since"`
	Partial       bool                         `json:"partial"`
	History       []AccountScheduledTestBucket `json:"-"`
}

type AccountScheduledTestBucket struct {
	Start     time.Time
	Requests  int64
	Errors    int64
	LatencyMs int64
}
