package proxyopen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func defaultOverview() OverviewOptions {
	return OverviewOptions{Hours: 24, Page: 1, Mode: "all", Sort: "latency"}
}
func TestOverviewCurrentConnectionAndMetrics(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for _, id := range []int64{1, 2} {
		enable(t, s, id)
		j := claimOne(t, s)
		for n, latency := range []int64{100, 300, -1} {
			r := successSample()
			r.Started = time.Now().Add(-time.Duration(n*20+1) * time.Second)
			r.Finished = r.Started
			if latency < 0 {
				r.Success = false
				r.Latency = nil
				r.Status = 503
				r.Error = "http_5xx"
			} else {
				value := latency + id
				r.Latency = &value
			}
			_, err := s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status,error_class)
   VALUES($1,$2,1,$3,$4,$4,$5,$6,$7,$8,$9)`, id, j.Fingerprint, j.Execution+string(rune('a'+n)), r.Started, r.Finished, r.Success, r.Latency, r.Status, r.Error)
			require.NoError(t, err)
		}
	}
	_, err := s.DB.Exec(`UPDATE proxies SET name='proxy-'||id,username='private-user',password='private-pass' WHERE id=2`)
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO proxies(id,name,status) VALUES(3,'inactive','inactive'),(4,'expired','active'),(5,'deleted','active'),(6,'literal%_','active');UPDATE proxies SET expires_at=now()-interval '1 second' WHERE id=4;UPDATE proxies SET deleted_at=now() WHERE id=5`)
	require.NoError(t, err)
	overview, err := s.Overview(ctx, defaultOverview())
	require.NoError(t, err)
	require.EqualValues(t, 5, overview.Total)
	require.Len(t, overview.Rows, 5)
	var first, changed, paused OverviewRow
	for _, row := range overview.Rows {
		switch row.ID {
		case 1:
			first = row
		case 2:
			changed = row
		case 6:
			paused = row
		}
	}
	report, err := s.QueryAt(ctx, 1, 24, 1, overview.To)
	require.NoError(t, err)
	require.Equal(t, report.Summary, first.Summary)
	require.Equal(t, report.Latest.ID, first.Latest.ID)
	require.True(t, report.Latest.Started.Equal(first.Latest.Started))
	require.Equal(t, report.Latest.Success, first.Latest.Success)
	require.Equal(t, report.Latest.Latency, first.Latest.Latency)
	require.False(t, first.Stale)
	require.EqualValues(t, 3, first.Summary.Count)
	require.EqualValues(t, 1, first.Summary.Failures)
	require.Zero(t, changed.Summary.Count)
	require.Nil(t, changed.Latest)
	require.True(t, changed.Stale)
	require.False(t, paused.Config.Enabled)
	require.Equal(t, 30, paused.Config.Interval)
	require.True(t, paused.Eligible)
	for _, row := range overview.Rows {
		if row.ID == 3 || row.ID == 4 {
			require.False(t, row.Eligible)
		}
	}
	bytes, err := json.Marshal(overview)
	require.NoError(t, err)
	for _, secret := range []string{"private-user", "private-pass", "fingerprint", "password", "username"} {
		require.NotContains(t, string(bytes), secret)
	}
	o := defaultOverview()
	o.Search = "%_"
	filtered, err := s.Overview(ctx, o)
	require.NoError(t, err)
	require.EqualValues(t, 1, filtered.Total)
	require.EqualValues(t, 6, filtered.Rows[0].ID)
	o.Search = ""
	o.Mode = "enabled"
	filtered, err = s.Overview(ctx, o)
	require.NoError(t, err)
	require.EqualValues(t, 2, filtered.Total)
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: false, Interval: 60}))
	o.Mode = "paused"
	filtered, err = s.Overview(ctx, o)
	require.NoError(t, err)
	require.EqualValues(t, 4, filtered.Total)
	// Changing only the interval/enable flag keeps same-connection history.
	for _, row := range filtered.Rows {
		if row.ID == 1 {
			require.EqualValues(t, 3, row.Summary.Count)
		}
	}
}

func TestOverviewSortPaginationAndEmpty(t *testing.T) {
	s := testStoreWithData(t, false)
	ctx := context.Background()
	result, err := s.Overview(ctx, defaultOverview())
	require.NoError(t, err)
	require.Zero(t, result.Total)
	require.Empty(t, result.Rows)
	_, err = s.DB.Exec(`INSERT INTO proxies(id,name) SELECT n,'proxy-'||lpad(n::text,3,'0') FROM generate_series(1,53)n`)
	require.NoError(t, err)
	for _, id := range []int64{1, 2, 3} {
		enable(t, s, id)
		j := claimOne(t, s)
		sample := successSample()
		value := id * 100
		sample.Latency = &value
		if id == 3 {
			sample.Success = false
			sample.Latency = nil
			sample.Status = 429
			sample.Error = "http_429"
		}
		ok, err := s.finish(ctx, j, sample)
		require.NoError(t, err)
		require.True(t, ok)
	}
	o := defaultOverview()
	result, err = s.Overview(ctx, o)
	require.NoError(t, err)
	require.Len(t, result.Rows, 50)
	require.EqualValues(t, 2, result.Rows[0].ID)
	require.EqualValues(t, 1, result.Rows[1].ID)
	for _, sort := range []string{"failures", "success_rate"} {
		o.Sort = sort
		result, err = s.Overview(ctx, o)
		require.NoError(t, err)
		require.EqualValues(t, 3, result.Rows[0].ID)
		require.Nil(t, result.Rows[0].Summary.P95)
		require.Nil(t, result.Rows[0].Latest.Latency)
	}
	o.Sort = "p95"
	result, err = s.Overview(ctx, o)
	require.NoError(t, err)
	require.EqualValues(t, 2, result.Rows[0].ID)
	_, err = s.DB.Exec(`UPDATE proxy_openai_monitor_samples SET started_at=now()-interval '10 minutes',finished_at=now()-interval '10 minutes' WHERE proxy_id=1`)
	require.NoError(t, err)
	result, err = s.Overview(ctx, o)
	require.NoError(t, err)
	for _, row := range result.Rows {
		if row.ID == 1 {
			require.True(t, row.Stale)
		}
	}
	o.Sort = "name"
	o.Page = 2
	result, err = s.Overview(ctx, o)
	require.NoError(t, err)
	require.EqualValues(t, 53, result.Total)
	require.Len(t, result.Rows, 3)
	require.EqualValues(t, 51, result.Rows[0].ID)
	o.Page = 3
	result, err = s.Overview(ctx, o)
	require.NoError(t, err)
	require.EqualValues(t, 53, result.Total)
	require.Empty(t, result.Rows)
	for _, bad := range []OverviewOptions{{Hours: 24, Page: 0, Mode: "all", Sort: "latency"}, {Hours: 2, Page: 1, Mode: "all", Sort: "name"}, {Hours: 24, Page: 1, Mode: "bad", Sort: "name"}, {Hours: 24, Page: 1, Mode: "all", Sort: "name;DROP TABLE proxies"}} {
		_, err = s.Overview(ctx, bad)
		require.ErrorIs(t, err, ErrInvalid)
	}
}

func TestOverviewHTTPValidationAndAdminBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := gin.New()
	group := g.Group("/proxies", func(c *gin.Context) {
		if c.GetHeader("role") == "" {
			c.AbortWithStatus(401)
		} else if c.GetHeader("role") != "admin" {
			c.AbortWithStatus(403)
		}
	})
	(&Runner{}).Register(group)
	for _, role := range []string{"", "user", "admin"} {
		out := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/proxies/openai-monitor/overview?sort=invalid", nil)
		req.Header.Set("role", role)
		g.ServeHTTP(out, req)
		expected := 400
		if role == "" {
			expected = 401
		} else if role == "user" {
			expected = 403
		}
		require.Equal(t, expected, out.Code)
	}
	s := testStore(t)
	g = gin.New()
	New(s, nil).Register(g.Group("/proxies"))
	out := httptest.NewRecorder()
	g.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/proxies/openai-monitor/overview", nil))
	require.Equal(t, 200, out.Code)
	require.True(t, strings.Contains(out.Body.String(), `"total":2`))
}

func TestOverviewSearchCharacterLimit(t *testing.T) {
	o := defaultOverview()
	o.Search = strings.Repeat("日", 100)
	require.True(t, o.valid())
	o.Search += "本"
	require.False(t, o.valid())
}
