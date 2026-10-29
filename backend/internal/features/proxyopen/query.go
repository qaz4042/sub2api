package proxyopen

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Metrics struct {
	Count     int64    `json:"count"`
	Successes int64    `json:"successes"`
	Failures  int64    `json:"failures"`
	Rate      *float64 `json:"success_rate"`
	Average   *float64 `json:"average_ms"`
	P95       *float64 `json:"p95_ms"`
}
type Bucket struct {
	At time.Time `json:"at"`
	Metrics
}
type Report struct {
	Config        Config     `json:"config"`
	Summary       Metrics    `json:"summary"`
	Latest        *Sample    `json:"latest"`
	Stale         bool       `json:"stale"`
	StaleSeconds  int        `json:"stale_seconds"`
	Eligible      bool       `json:"eligible"`
	From          time.Time  `json:"from"`
	To            time.Time  `json:"to"`
	First         *time.Time `json:"first_sample_at"`
	Last          *time.Time `json:"last_sample_at"`
	BucketSeconds int        `json:"bucket_seconds"`
	Trend         []Bucket   `json:"trend"`
	Records       []Sample   `json:"records"`
	Page          int        `json:"page"`
	PageSize      int        `json:"page_size"`
	Truncated     bool       `json:"truncated"`
}

const metricsSQL = `count(*),count(*) FILTER(WHERE success),count(*) FILTER(WHERE NOT success),
 avg(CASE WHEN success THEN 1.0 ELSE 0 END),avg(latency_ms) FILTER(WHERE success),
 percentile_cont(0.95) WITHIN GROUP(ORDER BY latency_ms) FILTER(WHERE success)`

func scanSample(row interface{ Scan(...any) error }) (*Sample, error) {
	r := &Sample{}
	err := row.Scan(&r.ID, &r.Started, &r.Finished, &r.Success, &r.Latency, &r.Status, &r.Error)
	return r, err
}

const sampleCols = `id,started_at,finished_at,success,latency_ms,http_status,error_class`

func (s *Store) Query(ctx context.Context, id int64, hours, page int) (*Report, error) {
	return s.QueryAt(ctx, id, hours, page, time.Time{})
}

func (s *Store) QueryAt(ctx context.Context, id int64, hours, page int, until time.Time) (*Report, error) {
	if hours != 1 && hours != 24 && hours != 168 {
		return nil, ErrInvalid
	}
	if page < 1 || page > 2000 {
		return nil, ErrInvalid
	}
	// One snapshot keeps config, fingerprint, aggregates and records consistent.
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	r := &Report{Config: Config{Interval: 30, State: "paused"}, Page: page, PageSize: 50, Trend: []Bucket{}, Records: []Sample{}}
	var fingerprint string
	err = tx.QueryRowContext(ctx, `SELECT `+fingerprintSQL+`,(`+eligibleSQL+`),now() FROM proxies p WHERE p.id=$1 AND p.deleted_at IS NULL`, id).Scan(&fingerprint, &r.Eligible, &r.To)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	err = tx.QueryRowContext(ctx, `SELECT enabled,interval_seconds,version,runtime_state FROM proxy_openai_monitor_configs WHERE proxy_id=$1`, id).Scan(&r.Config.Enabled, &r.Config.Interval, &r.Config.Version, &r.Config.State)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if !r.Eligible && r.Config.Enabled {
		r.Config.State = "unavailable"
	}
	r.StaleSeconds = r.Config.Interval*3 + 16
	now := r.To
	if !until.IsZero() {
		if until.After(r.To) || until.Before(r.To.Add(-7*24*time.Hour)) {
			return nil, ErrInvalid
		}
		r.To = until
	}
	r.From = r.To.Add(-time.Duration(hours) * time.Hour)
	r.BucketSeconds = 3600
	if hours == 1 {
		r.BucketSeconds = 60
	}
	filter := ` FROM proxy_openai_monitor_samples WHERE proxy_id=$1 AND fingerprint=$2 AND started_at >= $3 AND started_at < $4 AND finished_at < $4`
	args := []any{id, fingerprint, r.From, r.To}
	err = tx.QueryRowContext(ctx, `SELECT `+metricsSQL+`,min(started_at),max(started_at)`+filter, args...).Scan(&r.Summary.Count, &r.Summary.Successes, &r.Summary.Failures, &r.Summary.Rate, &r.Summary.Average, &r.Summary.P95, &r.First, &r.Last)
	if err != nil {
		return nil, err
	}
	// Latest is current-connection state, independent of the selected history range.
	latest, err := scanSample(tx.QueryRowContext(ctx, `SELECT `+sampleCols+` FROM proxy_openai_monitor_samples WHERE proxy_id=$1 AND fingerprint=$2 AND finished_at>=now()-interval '7 days' ORDER BY started_at DESC,id DESC LIMIT 1`, id, fingerprint))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		r.Latest = latest
	}
	r.Stale = r.Latest == nil || now.Sub(r.Latest.Started) > time.Duration(r.StaleSeconds)*time.Second
	// Buckets start at the exact range boundary; missing buckets remain null.
	rows, err := tx.QueryContext(ctx, `SELECT $3::timestamptz+floor(extract(epoch FROM (started_at-$3::timestamptz))/$5::integer)*$5::integer*interval '1 second',`+metricsSQL+filter+` GROUP BY 1 ORDER BY 1`, id, fingerprint, r.From, r.To, r.BucketSeconds)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b Bucket
		if err = rows.Scan(&b.At, &b.Count, &b.Successes, &b.Failures, &b.Rate, &b.Average, &b.P95); err != nil {
			rows.Close()
			return nil, err
		}
		r.Trend = append(r.Trend, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	filled := make([]Bucket, 0, hours*3600/r.BucketSeconds)
	index := 0
	for at := r.From; at.Before(r.To); at = at.Add(time.Duration(r.BucketSeconds) * time.Second) {
		b := Bucket{At: at}
		if index < len(r.Trend) && r.Trend[index].At.Sub(at).Abs() < time.Millisecond {
			b = r.Trend[index]
			index++
		}
		filled = append(filled, b)
	}
	r.Trend = filled
	rows, err = tx.QueryContext(ctx, `SELECT `+sampleCols+filter+` ORDER BY started_at DESC,id DESC LIMIT 50 OFFSET $5`, id, fingerprint, r.From, r.To, (page-1)*50)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		sample, e := scanSample(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		r.Records = append(r.Records, *sample)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	r.Truncated = int64(page*50) < r.Summary.Count
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}
