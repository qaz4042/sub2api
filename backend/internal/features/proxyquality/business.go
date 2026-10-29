package proxyquality

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

type cohortKey struct{ model, transport string }

func readBusiness(ctx context.Context, tx *sql.Tx, accountID, current int64, ids []int64, now time.Time, lastSwitch *time.Time) (businessSummary, error) {
	var result businessSummary
	var gap time.Time
	err := tx.QueryRowContext(ctx, `SELECT version,last_gap_at FROM account_proxy_quality_account_health WHERE account_id=$1`, accountID).Scan(&result.GapVersion, &gap)
	if errors.Is(err, sql.ErrNoRows) {
		result.Gap = true
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if now.Sub(gap) < 5*time.Minute {
		result.Gap = true
		return result, nil
	}
	filter := ` FROM account_proxy_quality_observations s JOIN proxies p ON p.id=s.proxy_id
 WHERE s.account_id=$1 AND s.proxy_id=ANY($2) AND s.observed_at >= $3::timestamptz-interval '1 hour' AND s.observed_at < $3
 AND s.fingerprint=encode(sha256(convert_to(jsonb_build_array(p.protocol,p.host,p.port,coalesce(p.username,''),coalesce(p.password,''))::text,'UTF8')),'hex')`
	recent := `s.observed_at >= $3::timestamptz-interval '5 minutes' AND (s.proxy_id<>$5 OR $4::timestamptz IS NULL OR s.observed_at >= $4)`
	rows, err := tx.QueryContext(ctx, `SELECT s.proxy_id,s.model,s.transport,
 count(*) FILTER(WHERE NOT s.http_5xx),count(*) FILTER(WHERE s.success),count(s.first_token_ms) FILTER(WHERE s.success),
 percentile_cont(0.5) WITHIN GROUP(ORDER BY s.first_token_ms) FILTER(WHERE s.success AND s.first_token_ms IS NOT NULL),max(s.observed_at),
 count(*) FILTER(WHERE `+recent+` AND NOT s.http_5xx),count(*) FILTER(WHERE `+recent+` AND s.success),
 count(s.first_token_ms) FILTER(WHERE `+recent+` AND s.success),
 percentile_cont(0.5) WITHIN GROUP(ORDER BY s.first_token_ms) FILTER(WHERE `+recent+` AND s.success AND s.first_token_ms IS NOT NULL),
 max(s.observed_at) FILTER(WHERE `+recent+` AND s.success)`+filter+` GROUP BY s.proxy_id,s.model,s.transport`, accountID, pq.Array(ids), now, lastSwitch, current)
	if err != nil {
		return result, err
	}
	indices := map[cohortKey]int{}
	for rows.Next() {
		var m businessMetric
		if err = rows.Scan(&m.ProxyID, &m.Model, &m.Transport, &m.Total, &m.Successes, &m.TTFTCount, &m.Median, &m.Last, &m.RecentTotal, &m.RecentSuccesses, &m.RecentTTFTCount, &m.RecentMedian, &m.LastRecentSuccess); err != nil {
			rows.Close()
			return result, err
		}
		if m.ProxyID == current {
			indices[cohortKey{m.Model, m.Transport}] = len(result.Metrics)
		}
		result.Metrics = append(result.Metrics, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT model,transport,success,persistent_failure,http_5xx,observed_at FROM (
 SELECT s.model,s.transport,s.success,s.persistent_failure,s.http_5xx,s.observed_at,
 row_number() OVER(PARTITION BY s.model,s.transport ORDER BY s.observed_at DESC,s.id DESC) AS rn`+filter+`
 AND s.proxy_id=$5 AND `+recent+`) ranked WHERE rn<=3 ORDER BY model,transport,rn`, accountID, pq.Array(ids), now, lastSwitch, current)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	type streak struct {
		network, server bool
		index           int
	}
	streaks := map[cohortKey]streak{}
	for rows.Next() {
		var model, transport string
		var success, persistent, http5xx bool
		var at time.Time
		if err = rows.Scan(&model, &transport, &success, &persistent, &http5xx, &at); err != nil {
			return result, err
		}
		key := cohortKey{model, transport}
		index, ok := indices[key]
		if !ok {
			continue
		}
		state, seen := streaks[key]
		if !seen {
			state.network = true
			state.server = true
		}
		m := &result.Metrics[index]
		if success || http5xx {
			state.network = false
		} else if state.network {
			m.RecentFailures++
			if state.index == 0 && persistent {
				m.RecentPersistent = true
			}
		}
		if success || !http5xx {
			state.server = false
		} else if state.server {
			m.RecentHTTP5xx++
			earliest := at
			m.FirstHTTP5xx = &earliest
		}
		state.index++
		streaks[key] = state
	}
	return result, rows.Err()
}
