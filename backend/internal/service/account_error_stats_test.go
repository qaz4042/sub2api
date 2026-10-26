package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type accountErrorStatsRepoStub struct {
	UsageLogRepository
	err        error
	start, end time.Time
	called     bool
}

func (r *accountErrorStatsRepoStub) GetAccountUsageStats(context.Context, int64, time.Time, time.Time) (*usagestats.AccountUsageStatsResponse, error) {
	return &usagestats.AccountUsageStatsResponse{Summary: usagestats.AccountUsageSummary{TotalCost: 12, TotalRequests: 7}}, nil
}

func (r *accountErrorStatsRepoStub) GetAccountErrorStats(ctx context.Context, _ int64, start, end time.Time, _ ...string) (*usagestats.AccountErrorStats, error) {
	r.called, r.start, r.end = true, start, end
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("missing query deadline")
	}
	return &usagestats.AccountErrorStats{}, r.err
}

func TestAccountErrorStatsFailurePreservesUsage(t *testing.T) {
	repo := &accountErrorStatsRepoStub{err: errors.New("error telemetry unavailable")}
	svc := &AccountUsageService{usageLogRepo: repo, errorStats: &accountErrorStatsService{reader: repo, opsService: &OpsService{}}}
	stats, err := svc.GetAccountUsageStats(context.Background(), 7, time.Now().AddDate(0, 0, -1), time.Now())
	require.NoError(t, err)
	require.True(t, repo.called)
	require.Equal(t, 12.0, stats.Summary.TotalCost)
	require.EqualValues(t, 7, stats.Summary.TotalRequests)
	require.Nil(t, stats.Errors)
	require.Equal(t, "unavailable", stats.ErrorsUnavailableReason)
}

func TestAccountErrorStatsCoverage(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 10, 22, 15, 0, 0, 0, loc)
	start := time.Date(2026, 9, 23, 0, 0, 0, 0, loc)
	end := time.Date(2026, 10, 23, 0, 0, 0, 0, loc)
	for _, tc := range []struct {
		name             string
		enabled, cleanup bool
		days             int
		wantStart        time.Time
		reason           string
		granularity      string
	}{
		{name: "disabled", days: 30, reason: "monitoring_disabled"},
		{name: "retention clipped", enabled: true, cleanup: true, days: 7, wantStart: time.Date(2026, 10, 16, 0, 0, 0, 0, loc)},
		{name: "cleanup disabled", enabled: true, days: 7, wantStart: start},
		{name: "zero retention truncates logs", enabled: true, cleanup: true, reason: "outside_retention"},
		{name: "hourly retention preserves complete hours", enabled: true, cleanup: true, days: 7, granularity: "hour", wantStart: time.Date(2026, 10, 15, 15, 0, 0, 0, loc)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &accountErrorStatsRepoStub{}
			ops := &OpsService{}
			ops.runtimeSettings.Store(&opsRuntimeSettingsSnapshot{monitoringEnabled: tc.enabled, advanced: OpsAdvancedSettings{DataRetention: OpsDataRetentionSettings{CleanupEnabled: tc.cleanup, ErrorLogRetentionDays: tc.days}}})
			svc := &accountErrorStatsService{reader: repo, opsService: ops}
			stats := &usagestats.AccountUsageStatsResponse{Granularity: tc.granularity}
			svc.enrich(context.Background(), stats, 7, start, end, now)
			require.Equal(t, tc.reason, stats.ErrorsUnavailableReason)
			if tc.reason != "" {
				require.False(t, repo.called)
				require.Nil(t, stats.Errors)
				return
			}
			require.Equal(t, tc.wantStart, repo.start)
			require.Equal(t, now, repo.end)
			require.Equal(t, tc.wantStart, stats.Errors.CoverageStart)
			require.Equal(t, tc.wantStart.After(start), stats.Errors.Partial)
		})
	}
}
