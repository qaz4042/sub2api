-- Per-account observation gaps; existing observations remain unchanged.
CREATE TABLE account_proxy_quality_account_health (
 account_id bigint PRIMARY KEY,
 version bigint NOT NULL DEFAULT 1,
 last_gap_at timestamptz NOT NULL
);

ALTER TABLE account_proxy_quality_observations
 ADD COLUMN http_5xx boolean NOT NULL DEFAULT false;
