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
 (7, 1, 'blocked-local', 'blocked', '2026-10-20 16:03Z', 403, '', FALSE),
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
	require.EqualValues(t, 2, stats.UncorrelatedErrors)
	require.Nil(t, stats.ErrorRate)
	require.Nil(t, stats.History[0].ErrorRate)

	// Cyber logs use request_type=4 even for WS turns. HTTP cyber usage can
	// also use a raw ID while the error carries a client ID. Neither case has
	// a reliable union denominator; repeated turns must remain visible.
	_, err = tx.ExecContext(ctx, `
INSERT INTO usage_logs (account_id, api_key_id, request_id, request_type, created_at) VALUES
 (7, 1, 'cyber-http-raw', 4, '2026-10-20 17:06Z');
INSERT INTO ops_error_logs (account_id, api_key_id, request_id, client_request_id, request_type, created_at, status_code, error_type) VALUES
 (7, 1, 'cyber-http-raw', 'cyber-http-client', 4, '2026-10-20 17:06Z', 400, 'cyber_policy'),
 (7, 1, 'cyber-ws', 'cyber-connection', 4, '2026-10-20 17:07Z', 200, 'cyber_policy'),
 (7, 1, 'cyber-ws', 'cyber-connection', 4, '2026-10-20 17:08Z', 200, 'cyber_policy')`)
	require.NoError(t, err)
	stats, err = repo.GetAccountErrorStats(ctx, 7, start, start.AddDate(0, 0, 2))
	require.NoError(t, err)
	require.EqualValues(t, 8, stats.ErrorCount)
	require.EqualValues(t, 2, stats.UncorrelatedWSErrors)
	require.EqualValues(t, 5, stats.UncorrelatedErrors)
	require.Nil(t, stats.ErrorRate)
	for _, point := range stats.History {
		require.Nil(t, point.ErrorRate)
	}
}

func TestAccountHourlyStatsSQL(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `
CREATE TEMP TABLE usage_logs (id BIGSERIAL, account_id BIGINT, api_key_id BIGINT, request_id TEXT, created_at TIMESTAMPTZ,
 input_tokens INT DEFAULT 0, output_tokens INT DEFAULT 0, cache_creation_tokens INT DEFAULT 0, cache_read_tokens INT DEFAULT 0,
 total_cost NUMERIC DEFAULT 0, account_stats_cost NUMERIC, account_rate_multiplier NUMERIC DEFAULT 1, actual_cost NUMERIC DEFAULT 0, duration_ms INT) ON COMMIT DROP;
CREATE TEMP TABLE ops_error_logs (id BIGSERIAL, account_id BIGINT, api_key_id BIGINT, request_id TEXT, client_request_id TEXT, created_at TIMESTAMPTZ, status_code INT, error_type TEXT DEFAULT '', request_type SMALLINT DEFAULT 1, is_count_tokens BOOLEAN DEFAULT FALSE) ON COMMIT DROP;
INSERT INTO usage_logs (account_id, api_key_id, request_id, created_at, input_tokens, total_cost, account_stats_cost, account_rate_multiplier, actual_cost, duration_ms) VALUES
 (7, 1, 'local:a', '2026-11-01 05:15Z', 10, 2, 3, 2, 1, 1200),
 (7, 1, 'local:b', '2026-11-01 06:15Z', 20, 5, NULL, 2, 4, NULL),
 (8, 1, 'other', '2026-11-01 06:15Z', 30, 9, NULL, 1, 9, 9000);
INSERT INTO ops_error_logs (account_id, api_key_id, request_id, created_at, status_code) VALUES
 (7, 1, 'a', '2026-11-01 05:16Z', 502),
 (7, 1, 'c', '2026-11-01 06:30Z', 500)`)
	require.NoError(t, err)
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1) // Fall DST day has two distinct 01:00 hours.
	repo := &usageLogRepository{sql: tx}
	history, err := repo.GetAccountHourlyUsageHistory(ctx, 7, start, end)
	require.NoError(t, err)
	require.Len(t, history, 25)
	require.Equal(t, "2026-11-01T05:00:00Z", history[1].Date)
	require.Equal(t, "2026-11-01T06:00:00Z", history[2].Date)
	require.EqualValues(t, 1, history[1].Requests)
	require.Equal(t, 6.0, history[1].ActualCost)
	require.Equal(t, 10.0, history[2].ActualCost)
	require.EqualValues(t, 20, history[2].Tokens)
	require.Equal(t, 4.0, history[2].UserCost)
	require.Zero(t, history[0].Requests)
	require.EqualValues(t, 1, history[1].DurationSampleCount)
	require.Equal(t, 1200.0, history[1].AvgDurationMs)
	require.Zero(t, history[2].DurationSampleCount)
	require.Zero(t, history[0].DurationSampleCount)

	errors, err := repo.GetAccountErrorStats(ctx, 7, start, end, "hour")
	require.NoError(t, err)
	require.Len(t, errors.History, 25)
	require.Equal(t, history[1].Date, errors.History[1].Date)
	require.Equal(t, 100.0, *errors.History[1].ErrorRate)
	require.Equal(t, 50.0, *errors.History[2].ErrorRate)
	require.Nil(t, errors.History[0].ErrorRate)
	require.EqualValues(t, 3, errors.TotalRequests)
	require.InDelta(t, 200.0/3, *errors.ErrorRate, 1e-9)
}
