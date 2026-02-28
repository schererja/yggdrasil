-- name: GetIdentityByProvider :one
SELECT i.*
FROM identities i
JOIN users u ON u.id = i.user_id
WHERE i.provider = $1
  AND i.provider_id = $2
  AND u.tenant_id = $3
LIMIT 1;

-- name: CreateIdentity :one
INSERT INTO identities (user_id, provider, provider_id, credential_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;
