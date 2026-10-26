package service

import (
	"context"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type accountScheduledTestStatsReader interface {
	GetAccountScheduledTestStats(context.Context, int64, time.Time, time.Time, string) (*usagestats.AccountScheduledTestStats, error)
}

func mergeAccountScheduledTests(stats *usagestats.AccountUsageStatsResponse, tests *usagestats.AccountScheduledTestStats, daily []usagestats.AccountUsageHistory, now time.Time) {
	if tests == nil {
		return
	}
	stats.ScheduledTests = tests
	days := make(map[string]usagestats.AccountUsageHistory, len(daily))
	for _, point := range daily {
		days[point.Date] = point
	}
	history := make(map[string]usagestats.AccountUsageHistory, len(stats.History))
	for _, point := range stats.History {
		history[point.Date] = point
	}
	var latency int64
	for _, test := range tests.History {
		date, label := usagestats.AccountStatsBucket(test.Start, "day")
		day := days[date]
		day.Date, day.Label = date, label
		day.Requests += test.Requests
		days[date] = day
		date, label = usagestats.AccountStatsBucket(test.Start, stats.Granularity)
		point := history[date]
		point.Date, point.Label = date, label
		point.Requests += test.Requests
		history[date] = point
		latency += test.LatencyMs
	}
	stats.History = sortedAccountUsageHistory(history)
	if tests.TotalRequests > 0 {
		summary := &stats.Summary
		oldRequests := summary.TotalRequests
		summary.TotalRequests += tests.TotalRequests
		summary.AvgDurationMs = (summary.AvgDurationMs*float64(oldRequests) + float64(latency)) / float64(summary.TotalRequests)
		updateAccountDailySummary(summary, sortedAccountUsageHistory(days), now)
	}

	// Business error telemetry can cover a shorter retained interval. Add only
	// matching test buckets and preserve unknown rates for uncorrelated failures.
	if errors := stats.Errors; errors != nil {
		byDate := make(map[string]*usagestats.AccountErrorHistory, len(errors.History))
		for i := range errors.History {
			byDate[errors.History[i].Date] = &errors.History[i]
		}
		for _, test := range tests.History {
			if test.Start.Before(errors.CoverageStart) || !test.Start.Before(errors.CoverageEnd) {
				continue
			}
			date, _ := usagestats.AccountStatsBucket(test.Start, stats.Granularity)
			if point := byDate[date]; point != nil {
				point.Requests += test.Requests
				point.ErrorCount += test.Errors
			}
			errors.TotalRequests += test.Requests
			errors.ErrorCount += test.Errors
		}
		errors.Partial = errors.Partial || errors.CoverageStart.Before(tests.CompleteSince)
		if errors.UncorrelatedErrors == 0 && errors.UncorrelatedWSErrors == 0 {
			errors.ErrorRate = scheduledTestCombinedErrorRate(errors.ErrorCount, errors.TotalRequests)
			for i := range errors.History {
				point := &errors.History[i]
				point.ErrorRate = scheduledTestCombinedErrorRate(point.ErrorCount, point.Requests)
			}
		}
	}
}

func scheduledTestCombinedErrorRate(errors, requests int64) *float64 {
	if requests == 0 {
		return nil
	}
	rate := float64(errors) / float64(requests) * 100
	return &rate
}

func sortedAccountUsageHistory(points map[string]usagestats.AccountUsageHistory) []usagestats.AccountUsageHistory {
	history := make([]usagestats.AccountUsageHistory, 0, len(points))
	for _, point := range points {
		history = append(history, point)
	}
	sort.Slice(history, func(i, j int) bool { return history[i].Date < history[j].Date })
	return history
}

// Daily metrics stay independent of the selected chart resolution. Tests add
// calls and active days; monetary and token totals still come from usage logs.
func updateAccountDailySummary(summary *usagestats.AccountUsageSummary, history []usagestats.AccountUsageHistory, now time.Time) {
	summary.ActualDaysUsed = len(history)
	if summary.ActualDaysUsed == 0 {
		summary.ActualDaysUsed = 1
	}
	days := float64(summary.ActualDaysUsed)
	summary.AvgDailyRequests = float64(summary.TotalRequests) / days
	summary.AvgDailyCost = summary.TotalCost / days
	summary.AvgDailyUserCost = summary.TotalUserCost / days
	summary.AvgDailyTokens = float64(summary.TotalTokens) / days
	var peak *usagestats.AccountUsageHistory
	for i := range history {
		point := &history[i]
		if peak == nil || point.Requests > peak.Requests {
			peak = point
		}
		if point.Date == now.Format("2006-01-02") {
			summary.Today = &struct {
				Date     string  `json:"date"`
				Cost     float64 `json:"cost"`
				UserCost float64 `json:"user_cost"`
				Requests int64   `json:"requests"`
				Tokens   int64   `json:"tokens"`
			}{point.Date, point.ActualCost, point.UserCost, point.Requests, point.Tokens}
		}
		if summary.HighestCostDay != nil && point.Date == summary.HighestCostDay.Date {
			summary.HighestCostDay.Requests = point.Requests
		}
	}
	if peak != nil {
		summary.HighestRequestDay = &struct {
			Date     string  `json:"date"`
			Label    string  `json:"label"`
			Requests int64   `json:"requests"`
			Cost     float64 `json:"cost"`
			UserCost float64 `json:"user_cost"`
		}{peak.Date, peak.Label, peak.Requests, peak.ActualCost, peak.UserCost}
	}
}
