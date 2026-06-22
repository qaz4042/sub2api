-- Platform access registry. This table is the source of truth for platform
-- visibility and gateway access switches. Existing switch values are imported
-- once during migration; runtime code no longer reads the old settings keys.

CREATE TABLE IF NOT EXISTS platform_configs (
    key         VARCHAR(32) PRIMARY KEY,
    label       VARCHAR(80) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled     BOOLEAN NOT NULL DEFAULT FALSE,
    core        BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (key ~ '^[a-z][a-z0-9_]*$'),
    CHECK (NOT core OR enabled)
);

INSERT INTO platform_configs (key, label, description, enabled, core, sort_order)
VALUES
    ('openai', 'OpenAI', 'Core OpenAI-compatible interface. Always enabled.', TRUE, TRUE, 10),
    (
        'anthropic',
        'Anthropic / Claude',
        'Claude-compatible interface. Disabled platforms are hidden from users and rejected by the gateway.',
        COALESCE((SELECT value = 'true' FROM settings WHERE key = 'platform_anthropic_enabled'), FALSE),
        FALSE,
        20
    ),
    (
        'gemini',
        'Gemini',
        'Google Gemini interface. Disabled platforms are hidden from users and rejected by the gateway.',
        COALESCE((SELECT value = 'true' FROM settings WHERE key = 'platform_gemini_enabled'), FALSE),
        FALSE,
        30
    ),
    (
        'antigravity',
        'Antigravity',
        'Antigravity interface. Disabled platforms are hidden from users and rejected by the gateway.',
        COALESCE((SELECT value = 'true' FROM settings WHERE key = 'platform_antigravity_enabled'), FALSE),
        FALSE,
        40
    )
ON CONFLICT (key) DO NOTHING;

CREATE INDEX IF NOT EXISTS platform_configs_sort_order_key_idx
    ON platform_configs (sort_order, key);
