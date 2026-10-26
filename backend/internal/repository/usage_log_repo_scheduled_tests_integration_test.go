//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestScheduledTestStatsMigrationAndRetention(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	// Exercise the actual migration against legacy results in an isolated schema.
	_, err = tx.ExecContext(ctx, `
CREATE SCHEMA scheduled_test_stats_test;
SET LOCAL search_path TO scheduled_test_stats_test;
CREATE TABLE accounts (id BIGINT PRIMARY KEY);
CREATE TABLE scheduled_test_plans (id BIGINT PRIMARY KEY, account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE);
CREATE TABLE scheduled_test_results (id BIGSERIAL PRIMARY KEY, plan_id BIGINT REFERENCES scheduled_test_plans(id) ON DELETE CASCADE,
 status TEXT NOT NULL, latency_ms BIGINT NOT NULL, started_at TIMESTAMPTZ NOT NULL);
INSERT INTO accounts VALUES (7), (8);
INSERT INTO scheduled_test_plans VALUES (10, 7), (20, 8);
INSERT INTO scheduled_test_results (plan_id, status, latency_ms, started_at) VALUES
 (10, 'success', 100, '2026-10-20 18:20Z'),
 (20, 'failed', 900, '2026-10-20 18:20Z');`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("243_scheduled_test_stats.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO scheduled_test_results (plan_id, status, latency_ms, started_at) VALUES
 (10, 'failed', 300, '2026-10-20 18:20:30Z'),
 (10, 'success', 200, '2026-10-20 19:20Z'),
 (10, 'failed', 500, '2026-10-21 18:15Z');`)
	require.NoError(t, err)
	// A replay must neither reset nor double-count pre-existing buckets.
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	loc, err := time.LoadLocation("Asia/Kathmandu")
	require.NoError(t, err)
	start := time.Date(2026, 10, 21, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	repo := &usageLogRepository{sql: tx}
	stats, err := repo.GetAccountScheduledTestStats(ctx, 7, start, end, "hour")
	require.NoError(t, err)
	require.EqualValues(t, 3, stats.TotalRequests, "filter by account and exclusive local-day end")
	require.EqualValues(t, 1, stats.ErrorCount)
	require.Len(t, stats.History, 2)
	require.True(t, stats.History[0].Start.Equal(start))
	require.EqualValues(t, 2, stats.History[0].Requests)
	require.EqualValues(t, 400, stats.History[0].LatencyMs)
	require.True(t, stats.History[1].Start.Equal(start.Add(time.Hour)))

	_, err = tx.ExecContext(ctx, `DELETE FROM scheduled_test_results; DELETE FROM scheduled_test_plans WHERE id = 10;`)
	require.NoError(t, err)
	stats, err = repo.GetAccountScheduledTestStats(ctx, 7, start, end, "day")
	require.NoError(t, err)
	require.EqualValues(t, 3, stats.TotalRequests, "pruning details and deleting plans must preserve counters")
	require.Len(t, stats.History, 1)
	require.True(t, stats.History[0].Start.Equal(start))
	_, err = tx.ExecContext(ctx, `DELETE FROM accounts WHERE id = 7`)
	require.NoError(t, err)
	stats, err = repo.GetAccountScheduledTestStats(ctx, 7, start, end, "day")
	require.NoError(t, err)
	require.Zero(t, stats.TotalRequests, "account deletion removes its counters")
}
