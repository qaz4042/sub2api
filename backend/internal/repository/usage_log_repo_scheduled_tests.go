package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

func (r *usageLogRepository) GetAccountScheduledTestStats(ctx context.Context, accountID int64, start, end time.Time, granularity string) (*usagestats.AccountScheduledTestStats, error) {
	stats := &usagestats.AccountScheduledTestStats{}
	if err := scanSingleRow(ctx, r.sql, `SELECT complete_since FROM scheduled_test_stats_state WHERE id = 1`, nil, &stats.CompleteSince); err != nil {
		return nil, err
	}
	stats.Partial = start.Before(stats.CompleteSince)
	rows, err := r.sql.QueryContext(ctx, `
SELECT CASE WHEN $5 = 'hour' THEN DATE_BIN(INTERVAL '1 hour', bucket, $2::timestamptz)
            ELSE date_trunc('day', bucket AT TIME ZONE $4) AT TIME ZONE $4 END AS period,
       SUM(requests), SUM(errors), SUM(latency_ms)
FROM scheduled_test_stats
WHERE account_id = $1 AND bucket >= $2 AND bucket < $3
GROUP BY 1 ORDER BY 1`, accountID, start, end, start.Location().String(), granularity)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var point usagestats.AccountScheduledTestBucket
		if err := rows.Scan(&point.Start, &point.Requests, &point.Errors, &point.LatencyMs); err != nil {
			return nil, err
		}
		point.Start = point.Start.In(start.Location())
		stats.History = append(stats.History, point)
		stats.TotalRequests += point.Requests
		stats.ErrorCount += point.Errors
	}
	return stats, rows.Err()
}
