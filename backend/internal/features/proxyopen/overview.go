package proxyopen

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type OverviewOptions struct {
	Hours, Page        int
	Search, Mode, Sort string
}
type OverviewRow struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	ProxyStatus string  `json:"proxy_status"`
	Eligible    bool    `json:"eligible"`
	Config      Config  `json:"config"`
	Latest      *Sample `json:"latest"`
	Stale       bool    `json:"stale"`
	Summary     Metrics `json:"summary"`
}
type Overview struct {
	Rows     []OverviewRow `json:"rows"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	From     time.Time     `json:"from"`
	To       time.Time     `json:"to"`
}

var overviewOrders = map[string]string{
	"latency":      "latest.latency_ms DESC NULLS LAST",
	"failures":     "stats.failures DESC",
	"p95":          "stats.p95_ms DESC NULLS LAST",
	"success_rate": "stats.success_rate ASC NULLS LAST",
	"name":         "b.name ASC",
}

func (o OverviewOptions) valid() bool {
	_, sortOK := overviewOrders[o.Sort]
	return (o.Hours == 1 || o.Hours == 24 || o.Hours == 168) && o.Page >= 1 && o.Page <= 2000 && utf8.RuneCountInString(o.Search) <= 100 &&
		(o.Mode == "all" || o.Mode == "enabled" || o.Mode == "paused") && sortOK
}

const overviewBase = ` FROM proxies p LEFT JOIN proxy_openai_monitor_configs c ON c.proxy_id=p.id
 WHERE p.deleted_at IS NULL AND ($1='' OR p.name ILIKE '%'||$1||'%' ESCAPE '\')
 AND ($2='all' OR ($2='enabled' AND coalesce(c.enabled,false)) OR ($2='paused' AND NOT coalesce(c.enabled,false)))`

// Overview aggregates current-connection samples in one bounded request; it does
// not fetch proxy credentials or run a full history/trend request per proxy.
func (s *Store) Overview(ctx context.Context, o OverviewOptions) (*Overview, error) {
	if !o.valid() {
		return nil, ErrInvalid
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	r := &Overview{Rows: []OverviewRow{}, Page: o.Page, PageSize: 50}
	if err = tx.QueryRowContext(ctx, `SELECT now()`).Scan(&r.To); err != nil {
		return nil, err
	}
	r.From = r.To.Add(-time.Duration(o.Hours) * time.Hour)
	search := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(o.Search)
	if err = tx.QueryRowContext(ctx, `SELECT count(*)`+overviewBase, search, o.Mode).Scan(&r.Total); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `WITH base AS (
 SELECT p.id,p.name,p.status,(`+eligibleSQL+`) AS eligible,`+fingerprintSQL+` AS fingerprint,
 coalesce(c.enabled,false) AS enabled,coalesce(c.interval_seconds,30) AS interval_seconds,
 coalesce(c.version,0) AS version,coalesce(c.runtime_state,'paused') AS runtime_state`+overviewBase+`)
 SELECT b.id,b.name,b.status,b.eligible,b.enabled,b.interval_seconds,b.version,b.runtime_state,
 CASE WHEN latest.id IS NOT NULL THEN to_jsonb(latest) END,
 stats.count,stats.successes,stats.failures,stats.success_rate,stats.average_ms,stats.p95_ms
 FROM base b
 CROSS JOIN LATERAL (SELECT `+metricsSQL+` FROM proxy_openai_monitor_samples
 WHERE proxy_id=b.id AND fingerprint=b.fingerprint AND started_at >= $3 AND started_at < $4 AND finished_at < $4)
 AS stats(count,successes,failures,success_rate,average_ms,p95_ms)
 LEFT JOIN LATERAL (SELECT `+sampleCols+` FROM proxy_openai_monitor_samples
 WHERE proxy_id=b.id AND fingerprint=b.fingerprint AND finished_at >= $4::timestamptz-interval '7 days' AND finished_at < $4
 ORDER BY started_at DESC,id DESC LIMIT 1) latest ON true
 ORDER BY `+overviewOrders[o.Sort]+`,b.id ASC LIMIT 50 OFFSET $5`, search, o.Mode, r.From, r.To, (o.Page-1)*50)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row OverviewRow
		var latestJSON []byte
		err = rows.Scan(&row.ID, &row.Name, &row.ProxyStatus, &row.Eligible, &row.Config.Enabled, &row.Config.Interval, &row.Config.Version, &row.Config.State, &latestJSON,
			&row.Summary.Count, &row.Summary.Successes, &row.Summary.Failures, &row.Summary.Rate, &row.Summary.Average, &row.Summary.P95)
		if err != nil {
			return nil, err
		}
		if len(latestJSON) > 0 {
			if err = json.Unmarshal(latestJSON, &row.Latest); err != nil {
				return nil, err
			}
		}
		row.Stale = row.Latest == nil || r.To.Sub(row.Latest.Started) > time.Duration(row.Config.Interval*3+16)*time.Second
		if row.Config.Enabled && !row.Eligible {
			row.Config.State = "unavailable"
		}
		r.Rows = append(r.Rows, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *Runner) overview(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	options := OverviewOptions{Hours: hours, Page: page, Search: strings.TrimSpace(c.Query("search")), Mode: c.DefaultQuery("mode", "all"), Sort: c.DefaultQuery("sort", "latency")}
	if !options.valid() {
		response.BadRequest(c, "Invalid monitor overview query")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	result, err := r.store.Overview(ctx, options)
	if err != nil {
		response.Error(c, 500, "Monitor overview unavailable")
		return
	}
	response.Success(c, result)
}
