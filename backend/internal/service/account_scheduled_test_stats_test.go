package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type scheduledStatsRepoStub struct {
	UsageLogRepository
	start  time.Time
	called bool
	err    error
}

func (r *scheduledStatsRepoStub) GetAccountUsageStats(context.Context, int64, time.Time, time.Time) (*usagestats.AccountUsageStatsResponse, error) {
	date, label := usagestats.AccountStatsBucket(r.start.AddDate(0, 0, 1), "day")
	return &usagestats.AccountUsageStatsResponse{
		History: []usagestats.AccountUsageHistory{{Date: date, Label: label, Requests: 5, Tokens: 50, Cost: 10, ActualCost: 10, UserCost: 8}},
		Summary: usagestats.AccountUsageSummary{Days: 2, ActualDaysUsed: 1, TotalRequests: 5, TotalTokens: 50, TotalCost: 10, TotalUserCost: 8, TotalStandardCost: 10, AvgDurationMs: 100, DurationSampleCount: 5},
	}, nil
}

func (r *scheduledStatsRepoStub) GetAccountHourlyUsageHistory(context.Context, int64, time.Time, time.Time) ([]usagestats.AccountUsageHistory, error) {
	date, label := usagestats.AccountStatsBucket(r.start.AddDate(0, 0, 1), "hour")
	return []usagestats.AccountUsageHistory{{Date: date, Label: label, Requests: 5, Tokens: 50, Cost: 10, ActualCost: 10, UserCost: 8}}, nil
}

func (r *scheduledStatsRepoStub) GetAccountScheduledTestStats(_ context.Context, _ int64, _, _ time.Time, _ string) (*usagestats.AccountScheduledTestStats, error) {
	r.called = true
	return &usagestats.AccountScheduledTestStats{
		TotalRequests: 5, ErrorCount: 2, CompleteSince: r.start, Partial: false,
		History: []usagestats.AccountScheduledTestBucket{
			{Start: r.start, Requests: 3, Errors: 1, LatencyMs: 600},
			{Start: r.start.AddDate(0, 0, 1), Requests: 2, Errors: 1, LatencyMs: 400},
		},
	}, r.err
}

type scheduledStatsErrorEnricher struct{ uncorrelated bool }

func (s scheduledStatsErrorEnricher) enrich(_ context.Context, stats *usagestats.AccountUsageStatsResponse, _ int64, start, end, _ time.Time) {
	start = start.AddDate(0, 0, 1) // Business error logs retain only the second day.
	date, label := usagestats.AccountStatsBucket(start, stats.Granularity)
	stats.Errors = &usagestats.AccountErrorStats{
		CoverageStart: start, CoverageEnd: end, Partial: true,
		TotalRequests: 7, ErrorCount: 2,
		History: []usagestats.AccountErrorHistory{{Date: date, Label: label, Requests: 7, ErrorCount: 2}},
	}
	if s.uncorrelated {
		stats.Errors.UncorrelatedErrors = 1
	}
}

func TestAccountStatsIncludeScheduledTests(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2026, 10, 20, 0, 0, 0, 0, loc)
	for _, granularity := range []string{"day", "hour"} {
		t.Run(granularity, func(t *testing.T) {
			repo := &scheduledStatsRepoStub{start: start}
			svc := &AccountUsageService{usageLogRepo: repo, errorStats: scheduledStatsErrorEnricher{}}
			business, err := svc.GetAccountUsageStatsWithOptions(context.Background(), 7, start, start.AddDate(0, 0, 2), granularity, false)
			require.NoError(t, err)
			require.False(t, repo.called)
			require.Nil(t, business.ScheduledTests)
			require.EqualValues(t, 5, business.Summary.TotalRequests)
			combined, err := svc.GetAccountUsageStatsWithOptions(context.Background(), 7, start, start.AddDate(0, 0, 2), granularity, true)
			require.NoError(t, err)
			require.True(t, repo.called)
			require.EqualValues(t, 10, combined.Summary.TotalRequests)
			require.Equal(t, 2, combined.Summary.ActualDaysUsed)
			require.Equal(t, 5.0, combined.Summary.AvgDailyRequests)
			require.Equal(t, 150.0, combined.Summary.AvgDurationMs)
			require.EqualValues(t, 10, combined.Summary.DurationSampleCount)
			require.EqualValues(t, 7, combined.Summary.HighestRequestDay.Requests)
			require.Equal(t, 10.0, combined.Summary.TotalCost)
			require.EqualValues(t, 50, combined.Summary.TotalTokens)
			require.Len(t, combined.History, 2)
			require.EqualValues(t, 3, combined.History[0].Requests)
			require.EqualValues(t, 7, combined.History[1].Requests)
			require.EqualValues(t, 5, combined.ScheduledTests.TotalRequests)
			require.EqualValues(t, 9, combined.Errors.TotalRequests, "respect business error retention")
			require.EqualValues(t, 3, combined.Errors.ErrorCount)
			require.InDelta(t, 100.0/3, *combined.Errors.ErrorRate, 1e-9)
			require.EqualValues(t, 9, combined.Errors.History[0].Requests)
			require.InDelta(t, 100.0/3, *combined.Errors.History[0].ErrorRate, 1e-9)
		})
	}
}

func TestAccountScheduledTestsDurationSamples(t *testing.T) {
	start := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                                      string
		businessSamples                           int64
		businessAverage, testLatency, wantAverage float64
	}{
		{"missing business duration", 1, 100, 200, 150},
		{"all business durations missing", 0, 0, 200, 200},
		{"measured zero", 1, 0, 200, 100},
		{"zero test latency", 1, 100, 0, 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stats := &usagestats.AccountUsageStatsResponse{
				Granularity: "day",
				Summary:     usagestats.AccountUsageSummary{TotalRequests: 2, DurationSampleCount: tc.businessSamples, AvgDurationMs: tc.businessAverage},
			}
			tests := &usagestats.AccountScheduledTestStats{
				TotalRequests: 1,
				History:       []usagestats.AccountScheduledTestBucket{{Start: start, Requests: 1, LatencyMs: int64(tc.testLatency)}},
			}
			mergeAccountScheduledTests(stats, tests, nil, start)
			require.EqualValues(t, 3, stats.Summary.TotalRequests)
			require.Equal(t, tc.businessSamples+1, stats.Summary.DurationSampleCount)
			require.Equal(t, tc.wantAverage, stats.Summary.AvgDurationMs)
		})
	}
}

func TestAccountStatsScheduledTestsPreserveUnknownErrorsAndQueryFailures(t *testing.T) {
	start := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	repo := &scheduledStatsRepoStub{start: start}
	svc := &AccountUsageService{usageLogRepo: repo, errorStats: scheduledStatsErrorEnricher{uncorrelated: true}}
	stats, err := svc.GetAccountUsageStatsWithOptions(context.Background(), 7, start, start.AddDate(0, 0, 2), "day", true)
	require.NoError(t, err)
	require.Nil(t, stats.Errors.ErrorRate)
	require.Nil(t, stats.Errors.History[0].ErrorRate)
	repo.err = errors.New("rollup query failed")
	_, err = svc.GetAccountUsageStatsWithOptions(context.Background(), 7, start, start.AddDate(0, 0, 2), "day", true)
	require.ErrorContains(t, err, "get scheduled test statistics failed")
}

func TestAccountScheduledTestsDSTAndToday(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	tests := &usagestats.AccountScheduledTestStats{TotalRequests: 2, ErrorCount: 1, CompleteSince: start, History: []usagestats.AccountScheduledTestBucket{
		{Start: start.Add(time.Hour), Requests: 1, LatencyMs: 100},
		{Start: start.Add(2 * time.Hour), Requests: 1, Errors: 1, LatencyMs: 200},
	}}
	stats := &usagestats.AccountUsageStatsResponse{Granularity: "hour"}
	mergeAccountScheduledTests(stats, tests, nil, start.Add(3*time.Hour))
	require.Len(t, stats.History, 2)
	require.Equal(t, "2026-11-01T05:00:00Z", stats.History[0].Date)
	require.Equal(t, "2026-11-01T06:00:00Z", stats.History[1].Date)
	require.Equal(t, stats.History[0].Label, stats.History[1].Label)
	require.EqualValues(t, 2, stats.Summary.Today.Requests)
	require.Equal(t, 150.0, stats.Summary.AvgDurationMs)
	require.Nil(t, stats.Errors, "tests must not imply business monitoring is available")
}
