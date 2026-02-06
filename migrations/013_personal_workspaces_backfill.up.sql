WITH new_ws AS (
INSERT INTO workspaces (id, name, default_currency, created_by, created_at)
SELECT
    gen_random_uuid(),
    'Personal workspace',
    'UAH',
    u.id,
    now()
FROM users u
WHERE NOT EXISTS (
    SELECT 1
    FROM workspaces_members wm
    WHERE wm.user_id = u.id
)
    RETURNING id, created_by
)
INSERT INTO workspaces_members (workspace_id, user_id, role, created_at)
SELECT id, created_by, 'owner', now()
FROM new_ws
    ON CONFLICT DO NOTHING;
