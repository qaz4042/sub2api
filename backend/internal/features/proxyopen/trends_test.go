package proxyopen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTrendsMixedFailuresGapsAndFingerprint(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	enable(t, s, 1)
	j := claimOne(t, s)
	var until time.Time
	require.NoError(t, s.DB.QueryRow(`SELECT now()`).Scan(&until))
	from := until.Add(-time.Hour)
	// One bucket with successes 100/300ms and a failure, one all-failed bucket,
	// plus missing buckets. Include an exact left boundary and exclude right.
	for n, entry := range []struct {
		at      time.Time
		latency int64
	}{{from, 100}, {from.Add(time.Second), 300}, {from.Add(2 * time.Second), -1}, {from.Add(time.Minute), -1}, {until, 500}} {
		var latency any
		success := entry.latency >= 0
		if success {
			latency = entry.latency
		}
		_, err := s.DB.Exec(`INSERT INTO proxy_openai_monitor_samples(proxy_id,fingerprint,version,execution_id,planned_at,started_at,finished_at,success,latency_ms,http_status)
  VALUES(1,$1,1,$2,$3,$3,$3,$4,$5,401)`, j.Fingerprint, j.Execution+string(rune('a'+n)), entry.at, success, latency)
		require.NoError(t, err)
	}
	trends, err := s.Trends(ctx, []int64{1, 2}, 1, until)
	require.NoError(t, err)
	require.Len(t, trends.Series, 2)
	require.Len(t, trends.Series[0].Points, 60)
	point := trends.Series[0].Points[0]
	require.EqualValues(t, 3, point.Count)
	require.EqualValues(t, 1, point.Failures)
	require.InDelta(t, 200, *point.Average, 0.001)
	require.InDelta(t, 290, *point.P95, 0.001)
	require.InDelta(t, 2.0/3, *point.Rate, 0.00001)
	failed := trends.Series[0].Points[1]
	require.EqualValues(t, 1, failed.Count)
	require.Equal(t, 0.0, *failed.Rate)
	require.Nil(t, failed.Average)
	require.Nil(t, failed.P95)
	gap := trends.Series[0].Points[2]
	require.Zero(t, gap.Count)
	require.Nil(t, gap.Rate)
	require.Nil(t, gap.Average)
	for _, point := range trends.Series[1].Points {
		require.Zero(t, point.Count)
		require.Nil(t, point.Rate)
	}
	require.NoError(t, s.Configure(ctx, 1, Config{Enabled: true, Interval: 60}))
	same, err := s.Trends(ctx, []int64{1}, 1, until)
	require.NoError(t, err)
	require.EqualValues(t, 3, same.Series[0].Points[0].Count)
	_, err = s.DB.Exec(`UPDATE proxies SET password='new-private-password' WHERE id=1`)
	require.NoError(t, err)
	changed, err := s.Trends(ctx, []int64{1}, 1, until)
	require.NoError(t, err)
	for _, point := range changed.Series[0].Points {
		require.Zero(t, point.Count)
		require.Nil(t, point.Average)
	}
	_, err = s.DB.Exec(`UPDATE proxies SET deleted_at=now() WHERE id=1`)
	require.NoError(t, err)
	deleted, err := s.Trends(ctx, []int64{1, 2}, 1, until)
	require.NoError(t, err)
	require.Len(t, deleted.Series, 1)
	require.EqualValues(t, 2, deleted.Series[0].ID)
}
func TestTrendLimitsGranularityAndHTTP(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for _, tc := range []struct{ hours, seconds, buckets int }{{1, 60, 60}, {24, 300, 288}, {168, 3600, 168}} {
		r, err := s.Trends(ctx, []int64{2, 1}, tc.hours, time.Time{})
		require.NoError(t, err)
		require.Equal(t, tc.seconds, r.BucketSeconds)
		require.EqualValues(t, 2, r.Series[0].ID)
		require.Len(t, r.Series[0].Points, tc.buckets)
	}
	for _, ids := range [][]int64{{}, {0}, {1, 1}, {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}} {
		_, err := s.Trends(ctx, ids, 24, time.Time{})
		require.ErrorIs(t, err, ErrInvalid)
	}
	_, err := s.Trends(ctx, []int64{1}, 2, time.Time{})
	require.ErrorIs(t, err, ErrInvalid)
	_, err = s.Trends(ctx, []int64{1}, 24, time.Now().Add(time.Hour))
	require.ErrorIs(t, err, ErrInvalid)
	gin.SetMode(gin.TestMode)
	g := gin.New()
	group := g.Group("/proxies", func(c *gin.Context) {
		if c.GetHeader("role") == "" {
			c.AbortWithStatus(401)
		} else if c.GetHeader("role") != "admin" {
			c.AbortWithStatus(403)
		}
	})
	New(s, nil).Register(group)
	for _, role := range []string{"", "user", "admin"} {
		out := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/proxies/openai-monitor/trends?ids=1,2", nil)
		req.Header.Set("role", role)
		g.ServeHTTP(out, req)
		want := 200
		if role == "" {
			want = 401
		} else if role == "user" {
			want = 403
		}
		require.Equal(t, want, out.Code)
	}
	for _, query := range []string{"ids=1,1", "ids=bad", "ids=1&hours=2", "ids=1&until=bad", "ids=1,2,3,4,5,6,7,8,9,10,11"} {
		out := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/proxies/openai-monitor/trends?"+query, nil)
		req.Header.Set("role", "admin")
		g.ServeHTTP(out, req)
		require.Equal(t, 400, out.Code)
	}
}
