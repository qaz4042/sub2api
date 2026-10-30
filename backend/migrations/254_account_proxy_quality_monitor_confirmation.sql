-- Persist confirmation across worker instances and restarts.
ALTER TABLE account_proxy_quality_configs
    ADD COLUMN monitor_candidate_id bigint,
    ADD COLUMN monitor_confirmations integer NOT NULL DEFAULT 0,
    ADD COLUMN monitor_current_sample_at timestamptz,
    ADD COLUMN monitor_candidate_sample_at timestamptz,
    ADD COLUMN monitor_current_fingerprint text,
    ADD COLUMN monitor_candidate_fingerprint text;
