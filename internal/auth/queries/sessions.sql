-- name: CreateSession :one
INSERT INTO user_sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT s.id, s.user_id, s.token_hash, s.expires_at, s.created_at, s.revoked_at, u.tenant_id
FROM user_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > NOW()
LIMIT 1;

-- name: RevokeSession :exec
UPDATE user_sessions
SET revoked_at = NOW()
WHERE id = $1;

-- name: RevokeAllUserSessions :exec
UPDATE user_sessions
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;
