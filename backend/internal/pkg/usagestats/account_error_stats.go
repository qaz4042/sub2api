package usagestats

import "time"

// AccountErrorStats describes observations in retained logs, not a guarantee of
// continuous monitoring. Rates are percentages; nil means no reliable denominator.
type AccountErrorStats struct {
	CoverageStart        time.Time             `json:"coverage_start"`
	CoverageEnd          time.Time             `json:"coverage_end"`
	Partial              bool                  `json:"partial"`
	UncorrelatedWSErrors int64                 `json:"uncorrelated_ws_errors"`
	TotalRequests        int64                 `json:"total_requests"`
	ErrorCount           int64                 `json:"error_count"`
	ErrorRate            *float64              `json:"error_rate"`
	History              []AccountErrorHistory `json:"history"`
}

type AccountErrorHistory struct {
	Date       string   `json:"date"`
	Label      string   `json:"label"`
	Requests   int64    `json:"requests"`
	ErrorCount int64    `json:"error_count"`
	ErrorRate  *float64 `json:"error_rate"`
}
