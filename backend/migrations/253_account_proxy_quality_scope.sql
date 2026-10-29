-- Existing configurations retain their explicitly selected pool.
ALTER TABLE account_proxy_quality_configs
    ADD COLUMN all_eligible boolean NOT NULL DEFAULT false;
