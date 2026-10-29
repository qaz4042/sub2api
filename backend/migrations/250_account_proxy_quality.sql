-- Opt-in account proxy quality routing; no changes to core account/proxy schema.
CREATE TABLE account_proxy_quality_configs (
    account_id bigint PRIMARY KEY,
    enabled boolean NOT NULL DEFAULT false,
    proxy_ids bigint[] NOT NULL DEFAULT '{}',
    version bigint NOT NULL DEFAULT 0,
    expected_proxy_id bigint,
    next_at timestamptz NOT NULL DEFAULT now(),
    last_evaluated_at timestamptz,
    last_switch_at timestamptz,
    reason text NOT NULL DEFAULT 'disabled',
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (cardinality(proxy_ids) <= 10)
);
CREATE INDEX account_proxy_quality_due ON account_proxy_quality_configs(next_at) WHERE enabled;
CREATE TABLE account_proxy_quality_events (
    id bigserial PRIMARY KEY,
    account_id bigint NOT NULL,
    from_proxy_id bigint NOT NULL,
    to_proxy_id bigint NOT NULL,
    reason text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX account_proxy_quality_history ON account_proxy_quality_events(account_id, id DESC);
