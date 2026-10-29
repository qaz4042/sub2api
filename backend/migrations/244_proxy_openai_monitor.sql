-- Independent, opt-in connectivity history. No credentials or billing data.
CREATE TABLE proxy_openai_monitor_configs (
    proxy_id bigint PRIMARY KEY,
    enabled boolean NOT NULL DEFAULT false,
    interval_seconds integer NOT NULL DEFAULT 30 CHECK (interval_seconds IN (10,30,60)),
    version bigint NOT NULL DEFAULT 1,
    next_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    execution_id text,
    runtime_state text NOT NULL DEFAULT 'paused',
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX proxy_openai_monitor_due ON proxy_openai_monitor_configs(next_at) WHERE enabled;
CREATE TABLE proxy_openai_monitor_samples (
    id bigserial PRIMARY KEY,
    proxy_id bigint NOT NULL,
    fingerprint text NOT NULL,
    version bigint NOT NULL,
    execution_id text NOT NULL UNIQUE,
    planned_at timestamptz NOT NULL,
    started_at timestamptz NOT NULL,
    finished_at timestamptz NOT NULL,
    success boolean NOT NULL,
    latency_ms bigint,
    http_status integer NOT NULL,
    error_class text NOT NULL DEFAULT '',
    CHECK ((success AND latency_ms IS NOT NULL) OR (NOT success AND latency_ms IS NULL))
);
CREATE INDEX proxy_openai_monitor_history ON proxy_openai_monitor_samples(proxy_id, fingerprint, started_at DESC, id DESC);
CREATE INDEX proxy_openai_monitor_retention ON proxy_openai_monitor_samples(finished_at);
