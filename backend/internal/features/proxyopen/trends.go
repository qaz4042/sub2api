package proxyopen

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
)

const maxTrendProxies = 10

type TrendSeries struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Points []Bucket `json:"points"`
}
type Trends struct {
	From          time.Time     `json:"from"`
	To            time.Time     `json:"to"`
	BucketSeconds int           `json:"bucket_seconds"`
	Series        []TrendSeries `json:"series"`
}

func trendBucketSeconds(hours int) int {
	if hours == 1 {
		return 60
	}
	if hours == 24 {
		return 300
	}
	return 3600
}
func validTrendIDs(ids []int64) bool {
	if len(ids) == 0 || len(ids) > maxTrendProxies {
		return false
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

// Trends reads at most ten current connections in one grouped query. Missing
// buckets remain null so service downtime cannot appear as a stable zero line.
func (s *Store) Trends(ctx context.Context, ids []int64, hours int, until time.Time) (*Trends, error) {
	if !validTrendIDs(ids) || (hours != 1 && hours != 24 && hours != 168) {
		return nil, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	r := &Trends{Series: []TrendSeries{}, BucketSeconds: trendBucketSeconds(hours)}
	if err = tx.QueryRowContext(ctx, `SELECT now()`).Scan(&r.To); err != nil {
		return nil, err
	}
	if !until.IsZero() {
		if until.After(r.To) || until.Before(r.To.Add(-7*24*time.Hour)) {
			return nil, ErrInvalid
		}
		r.To = until
	}
	r.From = r.To.Add(-time.Duration(hours) * time.Hour)
	names, err := tx.QueryContext(ctx, `SELECT id,name FROM proxies WHERE id=ANY($1::bigint[]) AND deleted_at IS NULL`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	byID := map[int64]*TrendSeries{}
	for names.Next() {
		series := &TrendSeries{}
		if err = names.Scan(&series.ID, &series.Name); err != nil {
			names.Close()
			return nil, err
		}
		byID[series.ID] = series
	}
	err = names.Err()
	names.Close()
	if err != nil {
		return nil, err
	}
	// A deletion during comparison simply removes the series; never expose old
	// connection samples or deleted proxies via a stored selection.
	rows, err := tx.QueryContext(ctx, `SELECT s.proxy_id,
 $2::timestamptz+floor(extract(epoch FROM (s.started_at-$2::timestamptz))/$4::integer)*$4::integer*interval '1 second',`+metricsSQL+`
 FROM proxy_openai_monitor_samples s JOIN proxies p ON p.id=s.proxy_id
 WHERE p.id=ANY($1::bigint[]) AND p.deleted_at IS NULL AND s.fingerprint=`+fingerprintSQL+`
 AND s.started_at >= $2 AND s.started_at < $3 AND s.finished_at < $3
 GROUP BY 1,2 ORDER BY 1,2`, pq.Array(ids), r.From, r.To, r.BucketSeconds)
	if err != nil {
		return nil, err
	}
	buckets := map[int64]map[int64]Bucket{}
	for rows.Next() {
		var id int64
		var b Bucket
		if err = rows.Scan(&id, &b.At, &b.Count, &b.Successes, &b.Failures, &b.Rate, &b.Average, &b.P95); err != nil {
			rows.Close()
			return nil, err
		}
		if buckets[id] == nil {
			buckets[id] = map[int64]Bucket{}
		}
		index := int64(b.At.Sub(r.From)/time.Second) / int64(r.BucketSeconds)
		buckets[id][index] = b
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		series := byID[id]
		if series == nil {
			continue
		}
		series.Points = make([]Bucket, 0, hours*3600/r.BucketSeconds)
		for index, at := int64(0), r.From; at.Before(r.To); index, at = index+1, at.Add(time.Duration(r.BucketSeconds)*time.Second) {
			b, exists := buckets[id][index]
			if !exists {
				b = Bucket{At: at}
			}
			series.Points = append(series.Points, b)
		}
		r.Series = append(r.Series, *series)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *Runner) trends(c *gin.Context) {
	parts := strings.Split(c.Query("ids"), ",")
	ids := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid trend proxy IDs")
			return
		}
		ids = append(ids, id)
	}
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	var until time.Time
	if value := c.Query("until"); value != "" {
		var err error
		until, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			response.BadRequest(c, "Invalid trend cutoff")
			return
		}
	}
	if !validTrendIDs(ids) || (hours != 1 && hours != 24 && hours != 168) {
		response.BadRequest(c, "Select up to 10 proxies and a supported range")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := r.store.Trends(ctx, ids, hours, until)
	if err == ErrInvalid {
		response.BadRequest(c, "Invalid trend range")
		return
	}
	if err != nil {
		response.Error(c, 500, "Monitor trends unavailable")
		return
	}
	response.Success(c, result)
}
