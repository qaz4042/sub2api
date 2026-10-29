INSERT INTO platform_configs (key, label, enabled, sort_order)
VALUES
    ('kimi', 'Kimi', TRUE, 60),
    ('zhipu', 'Zhipu', TRUE, 70),
    ('deepseek', 'DeepSeek', TRUE, 80),
    ('minimax', 'MiniMax', TRUE, 90),
    ('opencode_go', 'OpenCode Go', TRUE, 100),
    ('typesafe', 'TypeSafe', TRUE, 110),
    ('command_code', 'Command Code', TRUE, 120),
    ('cline', 'Cline', TRUE, 130),
    ('composite', 'Composite', TRUE, 140)
ON CONFLICT (key) DO NOTHING;
