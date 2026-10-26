package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type accountHourlyStatsStub struct {
	accountErrorStatsRepoStub
	hourlyCalled bool
	end          time.Time
}

func (r *accountHourlyStatsStub) GetAccountHourlyUsageHistory(_ context.Context, _ int64, _, end time.Time) ([]usagestats.AccountUsageHistory, error) {
	r.hourlyCalled, r.end = true, end
	return []usagestats.AccountUsageHistory{{Label: "10/22 01:00", Requests: 2}}, nil
}

func TestAccountStatsGranularity(t *testing.T) {
	repo := &accountHourlyStatsStub{}
	svc := &AccountUsageService{usageLogRepo: repo}
	start, end := time.Now().Add(-24*time.Hour), time.Now().Add(24*time.Hour)
	daily, err := svc.GetAccountUsageStats(context.Background(), 7, start, end)
	require.NoError(t, err)
	require.Equal(t, "day", daily.Granularity)
	require.False(t, repo.hourlyCalled)

	hourly, err := svc.GetAccountUsageStatsWithGranularity(context.Background(), 7, start, end, "hour")
	require.NoError(t, err)
	require.True(t, repo.hourlyCalled)
	require.Equal(t, daily.Summary, hourly.Summary, "resolution must not change daily summary metrics")
	require.Equal(t, "hour", hourly.Granularity)
	require.Len(t, hourly.History, 1)
	require.True(t, repo.end.Before(end), "do not fill future hours")
	_, err = svc.GetAccountUsageStatsWithGranularity(context.Background(), 7, start, end, "minute")
	require.ErrorContains(t, err, "invalid account stats granularity")
}
