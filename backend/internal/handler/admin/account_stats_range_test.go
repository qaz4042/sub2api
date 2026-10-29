package admin

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountStatsTimeRange(t *testing.T) {
	for _, tc := range []struct {
		name, start, end, tz string
		hours                time.Duration
	}{
		{"single day", "2026-10-01", "2026-10-01", "Asia/Shanghai", 24},
		{"inclusive end", "2026-10-01", "2026-10-03", "Asia/Shanghai", 72},
		{"90 days", "2026-01-01", "2026-03-31", "Asia/Shanghai", 90 * 24},
		{"DST spring", "2026-03-08", "2026-03-08", "America/New_York", 23},
		{"DST fall", "2026-11-01", "2026-11-01", "America/New_York", 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := accountStatsTimeRange(30, tc.start, tc.end, tc.tz)
			require.NoError(t, err)
			require.Equal(t, tc.start, start.Format(time.DateOnly))
			require.Equal(t, tc.end, end.AddDate(0, 0, -1).Format(time.DateOnly))
			require.Equal(t, tc.hours*time.Hour, end.Sub(start))
			require.Zero(t, start.Hour())
			require.Zero(t, end.Hour())
		})
	}
	start, end, err := accountStatsTimeRange(7, "", "", "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, start.AddDate(0, 0, 7), end)
}

func TestAccountStatsRejectInvalidDateRange(t *testing.T) {
	for _, query := range []string{
		"start_date=2026-10-01", "end_date=2026-10-01",
		"start_date=2026-02-30&end_date=2026-03-01",
		"start_date=2026-10-02&end_date=2026-10-01",
		"start_date=2026-01-01&end_date=2026-04-01",
	} {
		t.Run(query, func(t *testing.T) {
			response := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(response)
			ctx.Params = gin.Params{{Key: "id", Value: "7"}}
			ctx.Request = httptest.NewRequest("GET", "/?"+query, nil)
			(&AccountHandler{}).GetStats(ctx)
			require.Equal(t, 400, response.Code)
		})
	}
}
