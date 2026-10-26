package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// GetAccountHourlyUsageHistory changes only trend resolution. Daily summaries
// continue to come from GetAccountUsageStats, including averages and peak days.
func (r *usageLogRepository) GetAccountHourlyUsageHistory(ctx context.Context, accountID int64, start, end time.Time) ([]usagestats.AccountUsageHistory, error) {
	const query = `SELECT DATE_BIN(INTERVAL '1 hour', created_at, $2::timestamptz) AS bucket,
 COUNT(*), COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0),
 COALESCE(SUM(total_cost), 0),
 COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0),
 COALESCE(SUM(actual_cost), 0)
 FROM usage_logs WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
 GROUP BY bucket ORDER BY bucket`
	rows, err := r.sql.QueryContext(ctx, query, accountID, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	points := make(map[string]usagestats.AccountUsageHistory)
	for rows.Next() {
		var at time.Time
		var point usagestats.AccountUsageHistory
		if err := rows.Scan(&at, &point.Requests, &point.Tokens, &point.Cost, &point.ActualCost, &point.UserCost); err != nil {
			return nil, err
		}
		points[at.UTC().Format(time.RFC3339)] = point
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	history := make([]usagestats.AccountUsageHistory, 0)
	for at := start; at.Before(end); at = at.Add(time.Hour) {
		key, label := usagestats.AccountStatsBucket(at, "hour")
		point := points[key]
		point.Date, point.Label = key, label
		history = append(history, point)
	}
	return history, nil
}
