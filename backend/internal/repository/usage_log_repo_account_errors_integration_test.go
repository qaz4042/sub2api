//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountErrorStatsSQL(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	// Temporary tables isolate query semantics from unrelated test data and allow
	// legacy missing IDs, recovered rows and placeholders to be exercised together.
	_, err = tx.ExecContext(ctx, `
CREATE TEMP TABLE usage_logs (id BIGSERIAL, account_id BIGINT, api_key_id BIGINT, request_id TEXT, request_type SMALLINT DEFAULT 1, openai_ws_mode BOOLEAN DEFAULT FALSE, created_at TIMESTAMPTZ) ON COMMIT DROP;
CREATE TEMP TABLE ops_error_logs (id BIGSERIAL, account_id BIGINT, api_key_id BIGINT, request_id TEXT, client_request_id TEXT, created_at TIMESTAMPTZ, status_code INT, error_type TEXT DEFAULT '', request_type SMALLINT DEFAULT 1, is_count_tokens BOOLEAN DEFAULT FALSE) ON COMMIT DROP;
INSERT INTO usage_logs (account_id, api_key_id, request_id, created_at) VALUES
 (7, 1, 'local:ok', '2026-10-20 16:01Z'),
 (7, 1, 'local:failed', '2026-10-20 16:02Z'),
 (7, 1, 'client:blocked', '2026-10-20 16:03Z'),
 (7, 1, 'local:recovered', '2026-10-20 16:04Z'),
 (7, 1, '', '2026-10-20 16:05Z'),
 (7, 2, 'local:ok', '2026-10-20 16:06Z'),
 (8, 1, 'local:other', '2026-10-20 16:07Z');
INSERT INTO ops_error_logs (account_id, api_key_id, request_id, client_request_id, created_at, status_code, error_type, is_count_tokens) VALUES
 (7, 1, 'failed', NULL, '2026-10-20 16:02Z', 502, '', FALSE),
 (7, 1, 'failed', NULL, '2026-10-20 16:02Z', 502, '', FALSE),
 (7, 1, 'blocked-local', 'blocked', '2026-10-20 16:03Z', 200, 'cyber_policy', FALSE),
 (7, 1, 'recovered', NULL, '2026-10-20 16:04Z', 200, '', FALSE),
 (7, 1, NULL, NULL, '2026-10-20 16:05Z', 500, '', FALSE),
 (7, 1, 'count', NULL, '2026-10-20 16:06Z', 500, '', TRUE),
 (8, 1, 'other', NULL, '2026-10-20 16:07Z', 500, '', FALSE),
 (7, 1, 'before', NULL, '2026-10-20 15:59Z', 500, '', FALSE),
 (7, 1, 'end', NULL, '2026-10-22 16:00Z', 500, '', FALSE)`)
	require.NoError(t, err)
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2026, 10, 21, 0, 0, 0, 0, loc)
	repo := &usageLogRepository{sql: tx}
	stats, err := repo.GetAccountErrorStats(ctx, 7, start, start.AddDate(0, 0, 2))
	require.NoError(t, err)
	require.EqualValues(t, 7, stats.TotalRequests)
	require.EqualValues(t, 3, stats.ErrorCount)
	require.InDelta(t, 300.0/7, *stats.ErrorRate, 1e-9)
	require.Len(t, stats.History, 2)
	require.Equal(t, "2026-10-21", stats.History[0].Date)
	require.EqualValues(t, 3, stats.History[0].ErrorCount)
	require.Zero(t, stats.History[1].Requests)

	_, err = tx.ExecContext(ctx, `
INSERT INTO ops_error_logs (account_id, api_key_id, request_id, client_request_id, request_type, created_at, status_code) VALUES
 (7, 1, 'legacy-ws', 'legacy-connection', 3, '2026-10-20 17:04Z', 502),
 (7, 1, 'legacy-ws', 'legacy-connection', 3, '2026-10-20 17:05Z', 502)`)
	require.NoError(t, err)
	stats, err = repo.GetAccountErrorStats(ctx, 7, start, start.AddDate(0, 0, 2))
	require.NoError(t, err)
	require.EqualValues(t, 5, stats.ErrorCount)
	require.EqualValues(t, 2, stats.UncorrelatedWSErrors)
	require.Nil(t, stats.ErrorRate)
	require.Nil(t, stats.History[0].ErrorRate)
}
