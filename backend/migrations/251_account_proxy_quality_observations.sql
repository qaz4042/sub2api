-- Independent, bounded-retention observations. Historical usage logs cannot
-- establish which proxy was used and are deliberately not backfilled.
CREATE TABLE account_proxy_quality_observations (
 id bigserial PRIMARY KEY,
 account_id bigint NOT NULL,
 proxy_id bigint NOT NULL,
 fingerprint text NOT NULL,
 model text NOT NULL,
 transport text NOT NULL DEFAULT '',
 success boolean NOT NULL,
 persistent_failure boolean NOT NULL DEFAULT false,
 first_token_ms integer,
 observed_at timestamptz NOT NULL,
 CHECK (first_token_ms IS NULL OR first_token_ms >= 0)
);
CREATE INDEX account_proxy_quality_observation_lookup ON account_proxy_quality_observations(account_id,proxy_id,observed_at DESC);
CREATE INDEX account_proxy_quality_observation_retention ON account_proxy_quality_observations(observed_at);
CREATE TABLE account_proxy_quality_collection_health (
 id integer PRIMARY KEY CHECK (id=1),
 last_gap_at timestamptz NOT NULL
);
