WITH candidates AS (
    SELECT w.id
    FROM workspaces w
             JOIN workspaces_members wm ON wm.workspace_id = w.id
    WHERE w.name = 'Personal workspace'
    GROUP BY w.id
    HAVING COUNT(*) = 1
       AND MAX(wm.role) = 'owner'
       AND MAX(wm.user_id) = MAX(w.created_by)
)
DELETE FROM workspaces
WHERE id IN (SELECT id FROM candidates);
