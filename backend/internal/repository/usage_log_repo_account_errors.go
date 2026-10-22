package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

// GetAccountErrorStats correlates final errors with usage, including zero-cost
// successes. A failed request can also have a usage placeholder; group it once.
// Missing request IDs use row identities so unrelated legacy rows stay separate.
func (r *usageLogRepository) GetAccountErrorStats(ctx context.Context, accountID int64, startTime, endTime time.Time) (*usagestats.AccountErrorStats, error) {
	const query = `
WITH request_records AS (
  SELECT CASE
           WHEN request_id LIKE 'client:%' THEN request_id
           WHEN request_id LIKE 'local:%' THEN 'request:' || SUBSTRING(request_id FROM 7)
           ELSE COALESCE('request:' || NULLIF(request_id, ''), 'usage:' || id::text)
         END AS request_key,
         COALESCE(api_key_id, 0) AS api_key_id, created_at, 0 AS is_error, 0 AS uncorrelated_ws
  FROM usage_logs
  WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
  UNION ALL
  SELECT CASE
           WHEN request_type = 3 THEN 'error:' || id::text
           ELSE COALESCE('client:' || NULLIF(client_request_id, ''),
                  'request:' || NULLIF(request_id, ''), 'error:' || id::text)
         END,
         COALESCE(api_key_id, 0), created_at, 1,
         CASE WHEN request_type = 3 THEN 1 ELSE 0 END
  FROM ops_error_logs
  WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
    AND is_count_tokens = FALSE
    AND (COALESCE(status_code, 0) >= 400 OR error_type = 'cyber_policy')
), requests AS (
  SELECT request_key, MIN(created_at) AS created_at, MAX(is_error) AS is_error, MAX(uncorrelated_ws) AS uncorrelated_ws
  FROM request_records
  GROUP BY request_key, api_key_id
)
SELECT TO_CHAR(created_at AT TIME ZONE $4, 'YYYY-MM-DD') AS date,
       COUNT(*) AS requests, SUM(is_error) AS error_count, SUM(uncorrelated_ws)
FROM requests
GROUP BY 1
ORDER BY 1`
	rows, err := r.sql.QueryContext(ctx, query, accountID, startTime, endTime, startTime.Location().String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	byDate := make(map[string]usagestats.AccountErrorHistory)
	var uncorrelatedWSErrors int64
	for rows.Next() {
		var point usagestats.AccountErrorHistory
		var uncorrelated int64
		if err := rows.Scan(&point.Date, &point.Requests, &point.ErrorCount, &uncorrelated); err != nil {
			return nil, err
		}
		byDate[point.Date] = point
		uncorrelatedWSErrors += uncorrelated
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	stats := &usagestats.AccountErrorStats{History: make([]usagestats.AccountErrorHistory, 0), UncorrelatedWSErrors: uncorrelatedWSErrors}
	// Calendar arithmetic keeps DST days and empty days aligned with the UI timezone.
	for day := startTime; day.Before(endTime); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		point := byDate[date]
		point.Date = date
		point.Label = day.Format("01/02")
		point.ErrorRate = accountErrorRate(point.ErrorCount, point.Requests)
		stats.History = append(stats.History, point)
		stats.TotalRequests += point.Requests
		stats.ErrorCount += point.ErrorCount
	}
	stats.ErrorRate = accountErrorRate(stats.ErrorCount, stats.TotalRequests)
	if uncorrelatedWSErrors > 0 {
		// Legacy WS failures cannot be matched to partial usage rows reliably.
		stats.ErrorRate = nil
		for i := range stats.History {
			stats.History[i].ErrorRate = nil
		}
	}
	return stats, nil
}

func accountErrorRate(errors, requests int64) *float64 {
	if requests == 0 {
		return nil
	}
	rate := float64(errors) / float64(requests) * 100
	return &rate
}
