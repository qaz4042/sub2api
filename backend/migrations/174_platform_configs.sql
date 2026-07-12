CREATE TABLE IF NOT EXISTS platform_configs (
    key         VARCHAR(32) PRIMARY KEY,
    label       VARCHAR(80) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    core        BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (key ~ '^[a-z][a-z0-9_]*$'),
    CHECK (NOT core OR enabled)
);

INSERT INTO platform_configs (key, label, description, enabled, core, sort_order)
VALUES
    ('openai', 'OpenAI', 'OpenAI-compatible interface.', TRUE, TRUE, 10),
    ('anthropic', 'Anthropic / Claude', 'Claude-compatible interface.', TRUE, FALSE, 20),
    ('gemini', 'Gemini', 'Google Gemini interface.', TRUE, FALSE, 30),
    ('antigravity', 'Antigravity', 'Antigravity interface.', TRUE, FALSE, 40),
    ('grok', 'Grok', 'Grok-compatible interface.', TRUE, FALSE, 50)
ON CONFLICT (key) DO NOTHING;

CREATE INDEX IF NOT EXISTS platform_configs_sort_order_key_idx
    ON platform_configs (sort_order, key);
