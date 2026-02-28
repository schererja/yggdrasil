-- name: GetUserByEmail :one
SELECT u.*
FROM users u
WHERE u.tenant_id = $1 AND u.email = $2 AND u.is_active = true
LIMIT 1;

-- name: GetUserByID :one
SELECT u.*
FROM users u
WHERE u.id = $1;

-- name: CreateUser :one
INSERT INTO users (tenant_id, email, first_name, last_name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUserLastLogin :exec
UPDATE users
SET updated_at = NOW()
WHERE id = $1;

-- name: GetUserPermissions :many
SELECT DISTINCT p.key
FROM permissions p
JOIN role_permissions rp ON rp.permission_id = p.id
JOIN user_roles ur ON ur.role_id = rp.role_id
WHERE ur.user_id = $1;
