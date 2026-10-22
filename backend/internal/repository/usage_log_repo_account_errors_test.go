package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAccountErrorStats(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	start := time.Date(2026, 3, 7, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 3) // Includes the spring DST transition.
	for _, tc := range []struct {
		name  string
		empty bool
	}{
		{name: "weighted rates and zero-filled DST days"},
		{name: "no requests", empty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := &usageLogRepository{sql: db}
			rows := sqlmock.NewRows([]string{"date", "requests", "error_count", "uncorrelated_ws"})
			if !tc.empty {
				rows.AddRow("2026-03-07", 10, 2, 0).AddRow("2026-03-09", 2, 2, 0)
			}
			mock.ExpectQuery(`(?s)WITH request_records.*is_count_tokens = FALSE.*GROUP BY request_key, api_key_id.*AT TIME ZONE \$4`).
				WithArgs(int64(7), start, end, loc.String()).WillReturnRows(rows).RowsWillBeClosed()
			stats, err := repo.GetAccountErrorStats(context.Background(), 7, start, end)
			require.NoError(t, err)
			require.Len(t, stats.History, 3)
			require.Equal(t, "2026-03-08", stats.History[1].Date)
			require.Zero(t, stats.History[1].Requests)
			require.Zero(t, stats.History[1].ErrorRate)
			if tc.empty {
				require.Zero(t, stats.TotalRequests)
				require.Zero(t, stats.ErrorRate)
			} else {
				require.EqualValues(t, 12, stats.TotalRequests)
				require.EqualValues(t, 4, stats.ErrorCount)
				require.InDelta(t, 100.0/3, *stats.ErrorRate, 1e-9)
				require.Equal(t, 20.0, *stats.History[0].ErrorRate)
				require.Equal(t, 100.0, *stats.History[2].ErrorRate)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAccountErrorStatsQueryFailure(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	mock.ExpectQuery(`WITH request_records`).WillReturnError(errors.New("query failed"))
	stats, err := repo.GetAccountErrorStats(context.Background(), 7, time.Now(), time.Now())
	require.ErrorContains(t, err, "query failed")
	require.Nil(t, stats)
	require.NoError(t, mock.ExpectationsWereMet())
}
