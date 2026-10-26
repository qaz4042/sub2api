-- Preserve test counters independently of the bounded test-result history.
-- Minute buckets support local-hour boundaries in fractional-offset timezones.
LOCK TABLE scheduled_test_results IN SHARE ROW EXCLUSIVE MODE;

CREATE TABLE IF NOT EXISTS scheduled_test_stats (
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    bucket TIMESTAMPTZ NOT NULL,
    requests BIGINT NOT NULL DEFAULT 0,
    errors BIGINT NOT NULL DEFAULT 0,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, bucket)
);

CREATE TABLE IF NOT EXISTS scheduled_test_stats_state (
    id INT PRIMARY KEY CHECK (id = 1),
    complete_since TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO scheduled_test_stats_state (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

-- Earlier pruned results cannot be reconstructed; expose complete_since in stats.
INSERT INTO scheduled_test_stats (account_id, bucket, requests, errors, latency_ms)
SELECT p.account_id, date_trunc('minute', r.started_at), COUNT(*),
       COUNT(*) FILTER (WHERE r.status <> 'success'), SUM(GREATEST(r.latency_ms, 0))
FROM scheduled_test_results r JOIN scheduled_test_plans p ON p.id = r.plan_id
GROUP BY p.account_id, date_trunc('minute', r.started_at)
ON CONFLICT (account_id, bucket) DO NOTHING;

CREATE OR REPLACE FUNCTION accumulate_scheduled_test_stats() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO scheduled_test_stats (account_id, bucket, requests, errors, latency_ms)
    SELECT account_id, date_trunc('minute', NEW.started_at), 1,
           CASE WHEN NEW.status = 'success' THEN 0 ELSE 1 END, GREATEST(NEW.latency_ms, 0)
    FROM scheduled_test_plans WHERE id = NEW.plan_id
    ON CONFLICT (account_id, bucket) DO UPDATE SET
        requests = scheduled_test_stats.requests + EXCLUDED.requests,
        errors = scheduled_test_stats.errors + EXCLUDED.errors,
        latency_ms = scheduled_test_stats.latency_ms + EXCLUDED.latency_ms;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS scheduled_test_results_accumulate_stats ON scheduled_test_results;
CREATE TRIGGER scheduled_test_results_accumulate_stats
AFTER INSERT ON scheduled_test_results
FOR EACH ROW EXECUTE FUNCTION accumulate_scheduled_test_stats();
