package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type accountErrorStatsReader interface {
	GetAccountErrorStats(context.Context, int64, time.Time, time.Time) (*usagestats.AccountErrorStats, error)
}

type accountErrorStatsEnricher interface {
	enrich(context.Context, *usagestats.AccountUsageStatsResponse, int64, time.Time, time.Time, time.Time)
}

type accountErrorStatsService struct {
	reader     accountErrorStatsReader
	opsService *OpsService
}

// Error telemetry is optional: it must not make usage/cost statistics unavailable.
func (s *accountErrorStatsService) enrich(ctx context.Context, stats *usagestats.AccountUsageStatsResponse, accountID int64, start, end, now time.Time) {
	stats.Errors = nil
	stats.ErrorsUnavailableReason = "unavailable"
	if s.opsService == nil {
		return
	}
	if !s.opsService.IsMonitoringEnabled(ctx) {
		stats.ErrorsUnavailableReason = "monitoring_disabled"
		return
	}
	if s.reader == nil {
		return
	}
	originalStart := start
	retention := s.opsService.OpsAdvancedSettingsSnapshot().DataRetention
	if retention.CleanupEnabled && retention.ErrorLogRetentionDays == 0 {
		stats.ErrorsUnavailableReason = "outside_retention"
		return
	}
	if retention.CleanupEnabled && retention.ErrorLogRetentionDays > 0 {
		// Cleanup uses a rolling cutoff. Exclude its partially retained calendar
		// day, so the numerator and denominator cover the same whole days.
		cutoff := now.AddDate(0, 0, -retention.ErrorLogRetentionDays).In(start.Location())
		day := time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, start.Location())
		if day.Before(cutoff) {
			day = day.AddDate(0, 0, 1)
		}
		if day.After(start) {
			start = day
		}
	}
	if end.After(now) {
		end = now.In(start.Location())
	}
	if !start.Before(end) {
		stats.ErrorsUnavailableReason = "outside_retention"
		return
	}
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	errors, err := s.reader.GetAccountErrorStats(queryCtx, accountID, start, end)
	if err != nil {
		slog.Warn("account_error_stats.query_failed", "account_id", accountID, "error", err)
		return
	}
	if errors == nil {
		return
	}
	errors.CoverageStart = start
	errors.CoverageEnd = end
	errors.Partial = start.After(originalStart) || errors.UncorrelatedWSErrors > 0
	stats.Errors = errors
	stats.ErrorsUnavailableReason = ""
}
