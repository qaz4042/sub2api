package proxyopen

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Quality is the credential-free, current-connection contract consumed by
// optional routing features. Monitoring itself never changes account bindings.
type Quality struct {
	ID                              int64
	Fingerprint                     string
	Eligible, Enabled               bool
	Interval                        int
	Count, Successes                int
	P95                             *float64
	First, Last                     *time.Time
	RecentSuccesses, RecentFailures int
}

// ReadQuality uses the caller's transaction and clock. The caller locks proxies
// and monitor configs before reading, so pause/connection changes cannot race a
// routing commit. No overview pagination or historical UI range affects routing.
func ReadQuality(ctx context.Context, tx *sql.Tx, ids []int64, now time.Time) ([]Quality, error) {
	// Use the decision clock rather than transaction start after lock waits.
	eligibleAt := strings.ReplaceAll(eligibleSQL, "now()", "$2::timestamptz")
	rows, err := tx.QueryContext(ctx, `SELECT p.id,`+fingerprintSQL+`,(`+eligibleAt+`),coalesce(c.enabled,false),coalesce(c.interval_seconds,30),
 stats.n,stats.ok,stats.p95,stats.first_at,stats.last_at,stats.recent_ok,stats.recent_fail
 FROM proxies p LEFT JOIN proxy_openai_monitor_configs c ON c.proxy_id=p.id
 CROSS JOIN LATERAL (
 SELECT count(*) AS n,count(*) FILTER(WHERE success) AS ok,
 percentile_cont(0.95) WITHIN GROUP(ORDER BY latency_ms) FILTER(WHERE success) AS p95,
 min(started_at) AS first_at,max(started_at) AS last_at,
 count(*) FILTER(WHERE rn<=3 AND success) AS recent_ok,
 count(*) FILTER(WHERE rn<=3 AND NOT success) AS recent_fail
 FROM (SELECT started_at,success,latency_ms,row_number() OVER(ORDER BY started_at DESC,id DESC) AS rn
 FROM proxy_openai_monitor_samples WHERE proxy_id=p.id AND fingerprint=`+fingerprintSQL+`
 AND started_at >= $2::timestamptz-interval '5 minutes' AND started_at < $2 AND finished_at < $2) samples
 ) stats WHERE p.id=ANY($1)`, pq.Array(ids), now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Quality{}
	for rows.Next() {
		var q Quality
		if err = rows.Scan(&q.ID, &q.Fingerprint, &q.Eligible, &q.Enabled, &q.Interval, &q.Count, &q.Successes, &q.P95, &q.First, &q.Last, &q.RecentSuccesses, &q.RecentFailures); err != nil {
			return nil, err
		}
		result = append(result, q)
	}
	return result, rows.Err()
}
