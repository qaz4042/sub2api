package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type accountHourlyUsageReader interface {
	GetAccountHourlyUsageHistory(context.Context, int64, time.Time, time.Time) ([]usagestats.AccountUsageHistory, error)
}

func (s *AccountUsageService) GetAccountUsageStatsWithGranularity(ctx context.Context, accountID int64, start, end time.Time, granularity string) (*usagestats.AccountUsageStatsResponse, error) {
	return s.GetAccountUsageStatsWithOptions(ctx, accountID, start, end, granularity, false)
}

func (s *AccountUsageService) GetAccountUsageStatsWithOptions(ctx context.Context, accountID int64, start, end time.Time, granularity string, includeTests bool) (*usagestats.AccountUsageStatsResponse, error) {
	if granularity == "" {
		granularity = "day"
	}
	if granularity != "day" && granularity != "hour" {
		return nil, fmt.Errorf("invalid account stats granularity: %s", granularity)
	}
	stats, err := s.usageLogRepo.GetAccountUsageStats(ctx, accountID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get account usage stats failed: %w", err)
	}
	if stats == nil {
		return nil, nil
	}
	stats.Granularity = granularity
	dailyHistory := append([]usagestats.AccountUsageHistory(nil), stats.History...)
	now := time.Now()
	if granularity == "hour" {
		reader, ok := s.usageLogRepo.(accountHourlyUsageReader)
		if !ok {
			return nil, fmt.Errorf("hourly account usage statistics unavailable")
		}
		if end.After(now) {
			end = now.In(start.Location())
		}
		stats.History, err = reader.GetAccountHourlyUsageHistory(ctx, accountID, start, end)
		if err != nil {
			return nil, fmt.Errorf("get hourly account usage stats failed: %w", err)
		}
	}
	if s.errorStats != nil {
		s.errorStats.enrich(ctx, stats, accountID, start, end, now)
	}
	if includeTests {
		reader, ok := s.usageLogRepo.(accountScheduledTestStatsReader)
		if !ok {
			return nil, fmt.Errorf("scheduled test statistics unavailable")
		}
		if end.After(now) {
			end = now.In(start.Location())
		}
		tests, err := reader.GetAccountScheduledTestStats(ctx, accountID, start, end, granularity)
		if err != nil {
			return nil, fmt.Errorf("get scheduled test statistics failed: %w", err)
		}
		mergeAccountScheduledTests(stats, tests, dailyHistory, now.In(start.Location()))
	}
	return stats, nil
}
