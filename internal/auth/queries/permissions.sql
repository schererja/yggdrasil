-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY key;

-- name: UpsertPermission :one
INSERT INTO permissions (key, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE
    SET name = EXCLUDED.name,
        description = EXCLUDED.description
RETURNING *;
