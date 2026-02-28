-- name: ListRoles :many
SELECT * FROM roles
WHERE tenant_id = $1
ORDER BY name;

-- name: GetRoleByID :one
SELECT * FROM roles WHERE id = $1;

-- name: CreateRole :one
INSERT INTO roles (tenant_id, name, slug)
VALUES ($1, $2, $3)
RETURNING *;

-- name: AssignPermissionsToRole :exec
INSERT INTO role_permissions (role_id, permission_id)
SELECT $1, unnest($2::uuid[])
ON CONFLICT DO NOTHING;

-- name: ReplaceRolePermissions :exec
DELETE FROM role_permissions WHERE role_id = $1;

-- name: AssignRoleToUser :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;
