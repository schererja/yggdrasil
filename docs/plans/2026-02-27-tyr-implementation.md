# Týr Auth Service Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the Týr authentication and authorization service — JWT-based login, tenant-configurable roles, plugin permission registry with in-memory cache, and stored refresh token sessions.

**Architecture:** Email+password auth via an `identities` table (OAuth-ready). Access tokens are short-lived JWTs (15min); refresh tokens are 32-byte random values stored as SHA-256 hashes in `user_sessions` (30-day, revocable). Permissions are registered by domain packages via `init()`, resolved per-user via DB with a 5-minute in-memory cache.

**Tech Stack:** Go 1.23, Chi v5, golang-jwt/jwt v5, pgx/v5, sqlc, golang-migrate, argon2id (x/crypto), testify, testcontainers-go

**Design doc:** `docs/plans/2026-02-27-tyr-design.md`

---

## Task 1: Initialize Go module and project skeleton

**Files:**
- Create: `go.mod`
- Create: `go.sum` (auto-generated)
- Create: `cmd/api/main.go`
- Create: `internal/shared/config.go`
- Create: `internal/shared/database.go`

**Step 1: Initialize Go module**

```bash
go mod init github.com/schererja/yggdrasil
```

Expected: `go.mod` created with module path `github.com/schererja/yggdrasil`

**Step 2: Install core dependencies**

```bash
go get github.com/go-chi/chi/v5
go get github.com/golang-jwt/jwt/v5
go get github.com/google/uuid
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/stdlib
go get github.com/joho/godotenv
go get golang.org/x/crypto
go get github.com/sqlc-dev/sqlc@latest
```

**Step 3: Install dev/test dependencies**

```bash
go get github.com/stretchr/testify/assert
go get github.com/stretchr/testify/require
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
```

**Step 4: Create config**

Create `internal/shared/config.go`:

```go
package shared

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL          string
	NatsURL              string
	JWTSecret            string
	JWTExpiry            time.Duration
	RefreshTokenExpiry   time.Duration
	APIHost              string
	Env                  string
	LogLevel             string
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	jwtExpiry, _ := time.ParseDuration(getEnv("JWT_EXPIRY", "15m"))
	refreshExpiry, _ := time.ParseDuration(getEnv("REFRESH_TOKEN_EXPIRY", "720h")) // 30 days

	return &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://yggdrasil:yggdrasil@localhost:5432/yggdrasil?sslmode=disable"),
		NatsURL:            getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTExpiry:          jwtExpiry,
		RefreshTokenExpiry: refreshExpiry,
		APIHost:            getEnv("API_HOST", "0.0.0.0:8080"),
		Env:                getEnv("ENV", "development"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

**Step 5: Create database connection helper**

Create `internal/shared/database.go`:

```go
package shared

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func ConnectDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}

// SetTenantContext sets the PostgreSQL session variable used by RLS policies.
func SetTenantContext(ctx context.Context, db *sql.DB, tenantID string) error {
	_, err := db.ExecContext(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID)
	return err
}
```

**Step 6: Create minimal main.go**

Create `cmd/api/main.go`:

```go
package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/schererja/yggdrasil/internal/shared"
)

func main() {
	cfg := shared.LoadConfig()

	db, err := shared.ConnectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("starting on %s", cfg.APIHost)
	if err := http.ListenAndServe(cfg.APIHost, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

**Step 7: Update .env.example**

Modify `.env.example` — change `JWT_EXPIRY` and add `REFRESH_TOKEN_EXPIRY`:

```
JWT_EXPIRY=15m
REFRESH_TOKEN_EXPIRY=720h
```

**Step 8: Verify it compiles**

```bash
go build ./...
```

Expected: no errors

**Step 9: Commit**

```bash
git add go.mod go.sum cmd/ internal/shared/ .env.example
git commit -m "chore: initialize Go module and project skeleton"
```

---

## Task 2: Database migrations for Týr

**Files:**
- Create: `migrations/001_tenants.up.sql`
- Create: `migrations/001_tenants.down.sql`
- Create: `migrations/002_users.up.sql`
- Create: `migrations/002_users.down.sql`
- Create: `migrations/003_identities.up.sql`
- Create: `migrations/003_identities.down.sql`
- Create: `migrations/004_roles.up.sql`
- Create: `migrations/004_roles.down.sql`
- Create: `migrations/005_permissions.up.sql`
- Create: `migrations/005_permissions.down.sql`
- Create: `migrations/006_user_sessions.up.sql`
- Create: `migrations/006_user_sessions.down.sql`

**Step 1: Create migrations directory**

```bash
mkdir -p migrations
```

**Step 2: Create `001_tenants.up.sql`**

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE tenants (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    slug       VARCHAR(100) UNIQUE NOT NULL,
    settings   JSONB       NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Step 3: Create `001_tenants.down.sql`**

```sql
DROP TABLE IF EXISTS tenants;
```

**Step 4: Create `002_users.up.sql`**

```sql
CREATE TABLE users (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email       VARCHAR(255) NOT NULL,
    first_name  VARCHAR(100),
    last_name   VARCHAR(100),
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, email)
);

CREATE INDEX idx_users_tenant_email ON users (tenant_id, email);

ALTER TABLE users ENABLE ROW LEVEL SECURITY;

CREATE POLICY users_tenant_isolation ON users
    USING (tenant_id = current_setting('app.tenant_id', true)::UUID);
```

**Step 5: Create `002_users.down.sql`**

```sql
DROP TABLE IF EXISTS users;
```

**Step 6: Create `003_identities.up.sql`**

```sql
CREATE TABLE identities (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        VARCHAR(50) NOT NULL,
    provider_id     VARCHAR(255) NOT NULL,
    credential_hash VARCHAR(255),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_id)
);

CREATE INDEX idx_identities_user_id ON identities (user_id);
```

Note: `identities` is not RLS-protected directly — access is always through the `users` join.

**Step 7: Create `003_identities.down.sql`**

```sql
DROP TABLE IF EXISTS identities;
```

**Step 8: Create `004_roles.up.sql`**

```sql
CREATE TABLE roles (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, slug)
);

CREATE INDEX idx_roles_tenant ON roles (tenant_id);

ALTER TABLE roles ENABLE ROW LEVEL SECURITY;

CREATE POLICY roles_tenant_isolation ON roles
    USING (tenant_id = current_setting('app.tenant_id', true)::UUID);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id)  ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);
```

**Step 9: Create `004_roles.down.sql`**

```sql
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
```

**Step 10: Create `005_permissions.up.sql`**

```sql
CREATE TABLE permissions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    key         VARCHAR(100) UNIQUE NOT NULL,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
```

**Step 11: Create `005_permissions.down.sql`**

```sql
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
```

**Step 12: Create `006_user_sessions.up.sql`**

```sql
CREATE TABLE user_sessions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(64) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at  TIMESTAMPTZ
);

CREATE INDEX idx_user_sessions_user_id ON user_sessions (user_id);
CREATE INDEX idx_user_sessions_token_hash ON user_sessions (token_hash);

ALTER TABLE user_sessions ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_sessions_tenant_isolation ON user_sessions
    USING (
        user_id IN (
            SELECT id FROM users
            WHERE tenant_id = current_setting('app.tenant_id', true)::UUID
        )
    );
```

**Step 13: Create `006_user_sessions.down.sql`**

```sql
DROP TABLE IF EXISTS user_sessions;
```

**Step 14: Run migrations**

```bash
migrate -path migrations -database "postgres://yggdrasil:yggdrasil@localhost:5432/yggdrasil?sslmode=disable" up
```

Expected: `6/u users` (or similar — all 6 versions applied, no errors)

**Step 15: Commit**

```bash
git add migrations/
git commit -m "feat(tyr): add database migrations for auth tables"
```

---

## Task 3: sqlc configuration and SQL queries

**Files:**
- Create: `sqlc.yaml`
- Create: `internal/auth/queries/users.sql`
- Create: `internal/auth/queries/identities.sql`
- Create: `internal/auth/queries/roles.sql`
- Create: `internal/auth/queries/permissions.sql`
- Create: `internal/auth/queries/sessions.sql`
- Create (generated): `internal/auth/db/` (auto-generated by sqlc)

**Step 1: Create `sqlc.yaml`**

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/auth/queries/"
    schema: "migrations/"
    gen:
      go:
        package: "authdb"
        out: "internal/auth/db"
        emit_json_tags: true
        emit_interface: true
        emit_exact_table_names: false
        emit_empty_slices: true
```

**Step 2: Create `internal/auth/queries/users.sql`**

```sql
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
```

**Step 3: Create `internal/auth/queries/identities.sql`**

```sql
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
```

**Step 4: Create `internal/auth/queries/roles.sql`**

```sql
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
WITH deleted AS (
    DELETE FROM role_permissions WHERE role_id = $1
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT $1, unnest($2::uuid[]);

-- name: AssignRoleToUser :exec
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;
```

**Step 5: Create `internal/auth/queries/permissions.sql`**

```sql
-- name: ListPermissions :many
SELECT * FROM permissions ORDER BY key;

-- name: UpsertPermission :one
INSERT INTO permissions (key, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE
    SET name = EXCLUDED.name,
        description = EXCLUDED.description
RETURNING *;
```

**Step 6: Create `internal/auth/queries/sessions.sql`**

```sql
-- name: CreateSession :one
INSERT INTO user_sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT s.*, u.tenant_id
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
```

**Step 7: Generate sqlc code**

```bash
sqlc generate
```

Expected: `internal/auth/db/` directory created with `models.go`, `querier.go`, `db.go`, and query implementation files. No errors.

**Step 8: Verify compilation**

```bash
go build ./...
```

Expected: no errors

**Step 9: Commit**

```bash
git add sqlc.yaml internal/auth/queries/ internal/auth/db/
git commit -m "feat(tyr): add sqlc config and auth SQL queries"
```

---

## Task 4: Domain types

**Files:**
- Create: `internal/auth/types.go`

**Step 1: Write failing test**

Create `internal/auth/types_test.go`:

```go
package auth_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestUserFullName(t *testing.T) {
	u := auth.User{
		FirstName: strPtr("Jane"),
		LastName:  strPtr("Doe"),
	}
	assert.Equal(t, "Jane Doe", u.FullName())
}

func TestUserFullName_NilFields(t *testing.T) {
	u := auth.User{}
	assert.Equal(t, "", u.FullName())
}

func TestPermissionKey_Valid(t *testing.T) {
	assert.True(t, auth.IsValidPermissionKey("ticket:create"))
	assert.True(t, auth.IsValidPermissionKey("admin:*"))
	assert.False(t, auth.IsValidPermissionKey("invalid"))
	assert.False(t, auth.IsValidPermissionKey(""))
}

func strPtr(s string) *string { return &s }

// Ensure types compile with expected fields
var _ = auth.User{ID: uuid.New(), TenantID: uuid.New(), Email: "a@b.com"}
var _ = auth.Role{ID: uuid.New(), TenantID: uuid.New(), Name: "admin", Slug: "admin"}
var _ = auth.Permission{Key: "ticket:create", Name: "Create Tickets"}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestUser -v
```

Expected: FAIL — `auth` package does not exist yet

**Step 3: Create `internal/auth/types.go`**

```go
package auth

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Email     string
	FirstName *string
	LastName  *string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (u User) FullName() string {
	parts := []string{}
	if u.FirstName != nil {
		parts = append(parts, *u.FirstName)
	}
	if u.LastName != nil {
		parts = append(parts, *u.LastName)
	}
	return strings.Join(parts, " ")
}

type Identity struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Provider       string
	ProviderID     string
	CredentialHash *string
	CreatedAt      time.Time
}

type Role struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
}

type Permission struct {
	Key         string
	Name        string
	Description string
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
}

type TokenClaims struct {
	UserID   uuid.UUID `json:"sub"`
	TenantID uuid.UUID `json:"tid"`
}

// IsValidPermissionKey checks that a key follows "resource:action" format.
func IsValidPermissionKey(key string) bool {
	if key == "" {
		return false
	}
	parts := strings.SplitN(key, ":", 2)
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/auth/... -run TestUser -run TestPermissionKey -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/types.go internal/auth/types_test.go
git commit -m "feat(tyr): add domain types"
```

---

## Task 5: Permission registry

**Files:**
- Create: `internal/auth/permission.go`
- Create: `internal/auth/permission_test.go`

**Step 1: Write failing test**

Create `internal/auth/permission_test.go`:

```go
package auth_test

import (
	"testing"

	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndList(t *testing.T) {
	r := auth.NewRegistry()
	r.Register(auth.Permission{Key: "ticket:create", Name: "Create Tickets"})
	r.Register(auth.Permission{Key: "ticket:read", Name: "Read Tickets"})

	perms := r.List()
	assert.Len(t, perms, 2)
}

func TestRegistry_Register_DuplicateKey(t *testing.T) {
	r := auth.NewRegistry()
	r.Register(auth.Permission{Key: "ticket:create", Name: "Create Tickets"})
	r.Register(auth.Permission{Key: "ticket:create", Name: "Updated Name"}) // overwrite

	perms := r.List()
	require.Len(t, perms, 1)
	assert.Equal(t, "Updated Name", perms[0].Name)
}

func TestRegistry_Get(t *testing.T) {
	r := auth.NewRegistry()
	r.Register(auth.Permission{Key: "ticket:create", Name: "Create Tickets"})

	p, ok := r.Get("ticket:create")
	assert.True(t, ok)
	assert.Equal(t, "ticket:create", p.Key)

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestRegistry -v
```

Expected: FAIL — `NewRegistry` not defined

**Step 3: Create `internal/auth/permission.go`**

```go
package auth

import "sync"

// Registry holds all permissions registered by domain packages.
type Registry struct {
	mu          sync.RWMutex
	permissions map[string]Permission
}

func NewRegistry() *Registry {
	return &Registry{permissions: make(map[string]Permission)}
}

// Register adds or overwrites a permission in the registry.
func (r *Registry) Register(p Permission) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.permissions[p.Key] = p
}

// Get returns a permission by key.
func (r *Registry) Get(key string) (Permission, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.permissions[key]
	return p, ok
}

// List returns all registered permissions.
func (r *Registry) List() []Permission {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Permission, 0, len(r.permissions))
	for _, p := range r.permissions {
		out = append(out, p)
	}
	return out
}

// Global registry — populated by domain init() functions.
var GlobalRegistry = NewRegistry()

// RegisterPermission registers a permission in the global registry.
// Call this from domain init() functions.
func RegisterPermission(p Permission) {
	GlobalRegistry.Register(p)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/... -run TestRegistry -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/permission.go internal/auth/permission_test.go
git commit -m "feat(tyr): add permission registry"
```

---

## Task 6: In-memory permission cache

**Files:**
- Create: `internal/auth/cache.go`
- Create: `internal/auth/cache_test.go`

**Step 1: Write failing test**

Create `internal/auth/cache_test.go`:

```go
package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestCache_SetAndGet(t *testing.T) {
	c := auth.NewPermissionCache(5 * time.Minute)
	uid := uuid.New()

	c.Set(uid, []string{"ticket:create", "ticket:read"})

	perms, ok := c.Get(uid)
	assert.True(t, ok)
	assert.ElementsMatch(t, []string{"ticket:create", "ticket:read"}, perms)
}

func TestCache_Miss(t *testing.T) {
	c := auth.NewPermissionCache(5 * time.Minute)

	_, ok := c.Get(uuid.New())
	assert.False(t, ok)
}

func TestCache_Expiry(t *testing.T) {
	c := auth.NewPermissionCache(10 * time.Millisecond)
	uid := uuid.New()

	c.Set(uid, []string{"ticket:create"})
	time.Sleep(20 * time.Millisecond)

	_, ok := c.Get(uid)
	assert.False(t, ok, "should be expired")
}

func TestCache_Invalidate(t *testing.T) {
	c := auth.NewPermissionCache(5 * time.Minute)
	uid := uuid.New()

	c.Set(uid, []string{"ticket:create"})
	c.Invalidate(uid)

	_, ok := c.Get(uid)
	assert.False(t, ok)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestCache -v
```

Expected: FAIL — `NewPermissionCache` not defined

**Step 3: Create `internal/auth/cache.go`**

```go
package auth

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type cacheEntry struct {
	permissions []string
	expiresAt   time.Time
}

// PermissionCache is an in-memory, TTL-based cache of user permissions.
type PermissionCache struct {
	mu      sync.RWMutex
	entries map[uuid.UUID]cacheEntry
	ttl     time.Duration
}

func NewPermissionCache(ttl time.Duration) *PermissionCache {
	return &PermissionCache{
		entries: make(map[uuid.UUID]cacheEntry),
		ttl:     ttl,
	}
}

// Get returns cached permissions for a user. Returns false if not cached or expired.
func (c *PermissionCache) Get(userID uuid.UUID) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[userID]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.permissions, true
}

// Set stores permissions for a user with the configured TTL.
func (c *PermissionCache) Set(userID uuid.UUID, permissions []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[userID] = cacheEntry{
		permissions: permissions,
		expiresAt:   time.Now().Add(c.ttl),
	}
}

// Invalidate removes a user's cached permissions immediately.
func (c *PermissionCache) Invalidate(userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, userID)
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/... -run TestCache -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/cache.go internal/auth/cache_test.go
git commit -m "feat(tyr): add in-memory permission cache"
```

---

## Task 7: JWT management

**Files:**
- Create: `internal/auth/jwt.go`
- Create: `internal/auth/jwt_test.go`

**Step 1: Write failing test**

Create `internal/auth/jwt_test.go`:

```go
package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWT_IssueAndVerify(t *testing.T) {
	mgr := auth.NewJWTManager("supersecretkey-at-least-32-chars!!", 15*time.Minute)

	userID := uuid.New()
	tenantID := uuid.New()

	token, err := mgr.Issue(userID, tenantID)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	claims, err := mgr.Verify(token)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, tenantID, claims.TenantID)
}

func TestJWT_Expired(t *testing.T) {
	mgr := auth.NewJWTManager("supersecretkey-at-least-32-chars!!", -1*time.Second) // already expired

	token, err := mgr.Issue(uuid.New(), uuid.New())
	require.NoError(t, err)

	_, err = mgr.Verify(token)
	assert.Error(t, err)
}

func TestJWT_InvalidSignature(t *testing.T) {
	mgr1 := auth.NewJWTManager("supersecretkey-at-least-32-chars!!", 15*time.Minute)
	mgr2 := auth.NewJWTManager("a-completely-different-secret-key!", 15*time.Minute)

	token, err := mgr1.Issue(uuid.New(), uuid.New())
	require.NoError(t, err)

	_, err = mgr2.Verify(token)
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestJWT -v
```

Expected: FAIL — `NewJWTManager` not defined

**Step 3: Create `internal/auth/jwt.go`**

```go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type jwtClaims struct {
	TenantID uuid.UUID `json:"tid"`
	jwt.RegisteredClaims
}

// JWTManager issues and verifies access tokens.
type JWTManager struct {
	secret []byte
	expiry time.Duration
}

func NewJWTManager(secret string, expiry time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), expiry: expiry}
}

// Issue creates a signed JWT for the given user and tenant.
func (m *JWTManager) Issue(userID, tenantID uuid.UUID) (string, error) {
	claims := jwtClaims{
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.expiry)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Verify parses and validates a JWT, returning the embedded claims.
func (m *JWTManager) Verify(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid subject: %w", err)
	}

	return &TokenClaims{
		UserID:   userID,
		TenantID: claims.TenantID,
	}, nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/... -run TestJWT -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/jwt.go internal/auth/jwt_test.go
git commit -m "feat(tyr): add JWT manager"
```

---

## Task 8: Repository (database access layer)

**Files:**
- Create: `internal/auth/repository.go`
- Create: `internal/auth/repository_test.go` (integration test)

**Step 1: Write failing integration test**

Create `internal/auth/repository_test.go`:

```go
package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests require a real database.
// Run with: go test ./internal/auth/... -tags integration -v

func TestRepository_CreateAndGetUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t) // helper defined in testhelpers_test.go
	repo := auth.NewRepository(db)
	ctx := context.Background()

	tenantID := seedTenant(t, db)

	user, err := repo.CreateUser(ctx, tenantID, "jane@example.com", strPtr("Jane"), strPtr("Doe"))
	require.NoError(t, err)
	assert.Equal(t, "jane@example.com", user.Email)
	assert.Equal(t, tenantID, user.TenantID)

	fetched, err := repo.GetUserByEmail(ctx, tenantID, "jane@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, fetched.ID)
}

func TestRepository_CreateIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t)
	repo := auth.NewRepository(db)
	ctx := context.Background()

	tenantID := seedTenant(t, db)
	user, _ := repo.CreateUser(ctx, tenantID, "test@example.com", nil, nil)

	hash := "argon2idhashvalue"
	identity, err := repo.CreateIdentity(ctx, user.ID, "local", "test@example.com", &hash)
	require.NoError(t, err)
	assert.Equal(t, "local", identity.Provider)
}

func TestRepository_GetUserPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t)
	repo := auth.NewRepository(db)
	ctx := context.Background()

	tenantID := seedTenant(t, db)
	user, _ := repo.CreateUser(ctx, tenantID, "staff@example.com", nil, nil)
	role, _ := repo.CreateRole(ctx, tenantID, "Support", "support")
	perm, _ := repo.UpsertPermission(ctx, "ticket:create", "Create Tickets", "")
	_ = repo.AssignPermissionsToRole(ctx, role.ID, []uuid.UUID{perm.ID})
	_ = repo.AssignRoleToUser(ctx, user.ID, role.ID)

	perms, err := repo.GetUserPermissions(ctx, user.ID)
	require.NoError(t, err)
	assert.Contains(t, perms, "ticket:create")
}
```

**Step 2: Create test helper**

Create `internal/auth/testhelpers_test.go`:

```go
package auth_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", "postgres://yggdrasil:yggdrasil@localhost:5432/yggdrasil?sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func seedTenant(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(),
		"INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)",
		id, "Test Tenant", id.String(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM tenants WHERE id = $1", id)
	})
	return id
}
```

**Step 3: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestRepository -v -short
```

Expected: tests skipped (short mode). When run without -short against real DB, fails because `NewRepository` doesn't exist yet.

**Step 4: Create `internal/auth/repository.go`**

```go
package auth

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	authdb "github.com/schererja/yggdrasil/internal/auth/db"
)

// Repository provides data access for the auth domain.
type Repository struct {
	db      *sql.DB
	queries *authdb.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, queries: authdb.New(db)}
}

func (r *Repository) GetUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	row, err := r.queries.GetUserByEmail(ctx, authdb.GetUserByEmailParams{
		TenantID: tenantID,
		Email:    email,
	})
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return userFromDB(row), nil
}

func (r *Repository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return userFromDB(row), nil
}

func (r *Repository) CreateUser(ctx context.Context, tenantID uuid.UUID, email string, firstName, lastName *string) (*User, error) {
	var fn, ln sql.NullString
	if firstName != nil {
		fn = sql.NullString{String: *firstName, Valid: true}
	}
	if lastName != nil {
		ln = sql.NullString{String: *lastName, Valid: true}
	}
	row, err := r.queries.CreateUser(ctx, authdb.CreateUserParams{
		TenantID:  tenantID,
		Email:     email,
		FirstName: fn,
		LastName:  ln,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return userFromDB(row), nil
}

func (r *Repository) GetIdentityByProvider(ctx context.Context, provider, providerID string, tenantID uuid.UUID) (*Identity, error) {
	row, err := r.queries.GetIdentityByProvider(ctx, authdb.GetIdentityByProviderParams{
		Provider:   provider,
		ProviderID: providerID,
		TenantID:   tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("get identity: %w", err)
	}
	return identityFromDB(row), nil
}

func (r *Repository) CreateIdentity(ctx context.Context, userID uuid.UUID, provider, providerID string, credentialHash *string) (*Identity, error) {
	var hash sql.NullString
	if credentialHash != nil {
		hash = sql.NullString{String: *credentialHash, Valid: true}
	}
	row, err := r.queries.CreateIdentity(ctx, authdb.CreateIdentityParams{
		UserID:         userID,
		Provider:       provider,
		ProviderID:     providerID,
		CredentialHash: hash,
	})
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	return identityFromDB(row), nil
}

func (r *Repository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.queries.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user permissions: %w", err)
	}
	return rows, nil
}

func (r *Repository) CreateRole(ctx context.Context, tenantID uuid.UUID, name, slug string) (*Role, error) {
	row, err := r.queries.CreateRole(ctx, authdb.CreateRoleParams{
		TenantID: tenantID,
		Name:     name,
		Slug:     slug,
	})
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	return roleFromDB(row), nil
}

func (r *Repository) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]Role, error) {
	rows, err := r.queries.ListRoles(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	out := make([]Role, len(rows))
	for i, row := range rows {
		out[i] = *roleFromDB(row)
	}
	return out, nil
}

func (r *Repository) AssignPermissionsToRole(ctx context.Context, roleID uuid.UUID, permIDs []uuid.UUID) error {
	return r.queries.AssignPermissionsToRole(ctx, authdb.AssignPermissionsToRoleParams{
		RoleID:  roleID,
		Column2: permIDs,
	})
}

func (r *Repository) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return r.queries.AssignRoleToUser(ctx, authdb.AssignRoleToUserParams{
		UserID: userID,
		RoleID: roleID,
	})
}

func (r *Repository) UpsertPermission(ctx context.Context, key, name, description string) (*Permission, error) {
	row, err := r.queries.UpsertPermission(ctx, authdb.UpsertPermissionParams{
		Key:         key,
		Name:        name,
		Description: sql.NullString{String: description, Valid: description != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("upsert permission: %w", err)
	}
	desc := ""
	if row.Description.Valid {
		desc = row.Description.String
	}
	return &Permission{Key: row.Key, Name: row.Name, Description: desc}, nil
}

func (r *Repository) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt interface{}) (*Session, error) {
	row, err := r.queries.CreateSession(ctx, authdb.CreateSessionParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sessionFromDB(row), nil
}

func (r *Repository) GetSessionByTokenHash(ctx context.Context, hash string) (*Session, uuid.UUID, error) {
	row, err := r.queries.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("get session: %w", err)
	}
	return sessionFromDB(row.UserSession), row.TenantID, nil
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID uuid.UUID) error {
	return r.queries.RevokeSession(ctx, sessionID)
}

func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error {
	return r.queries.RevokeAllUserSessions(ctx, userID)
}

// --- mapping helpers ---

func userFromDB(row authdb.User) *User {
	u := &User{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Email:     row.Email,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
	if row.FirstName.Valid {
		u.FirstName = &row.FirstName.String
	}
	if row.LastName.Valid {
		u.LastName = &row.LastName.String
	}
	return u
}

func identityFromDB(row authdb.Identity) *Identity {
	i := &Identity{
		ID:         row.ID,
		UserID:     row.UserID,
		Provider:   row.Provider,
		ProviderID: row.ProviderID,
		CreatedAt:  row.CreatedAt.Time,
	}
	if row.CredentialHash.Valid {
		i.CredentialHash = &row.CredentialHash.String
	}
	return i
}

func roleFromDB(row authdb.Role) *Role {
	return &Role{
		ID:        row.ID,
		TenantID:  row.TenantID,
		Name:      row.Name,
		Slug:      row.Slug,
		CreatedAt: row.CreatedAt.Time,
	}
}

func sessionFromDB(row authdb.UserSession) *Session {
	s := &Session{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt.Time,
		CreatedAt: row.CreatedAt.Time,
	}
	if row.RevokedAt.Valid {
		t := row.RevokedAt.Time
		s.RevokedAt = &t
	}
	return s
}
```

> **Note:** The exact sqlc-generated parameter struct field names (e.g., `Column2`) may differ — check `internal/auth/db/` after `sqlc generate` and adjust accordingly.

**Step 5: Verify compilation**

```bash
go build ./...
```

Expected: no errors

**Step 6: Run integration tests against real DB**

```bash
docker compose up -d postgres
migrate -path migrations -database "postgres://yggdrasil:yggdrasil@localhost:5432/yggdrasil?sslmode=disable" up
go test ./internal/auth/... -run TestRepository -v
```

Expected: PASS

**Step 7: Commit**

```bash
git add internal/auth/repository.go internal/auth/repository_test.go internal/auth/testhelpers_test.go
git commit -m "feat(tyr): add auth repository"
```

---

## Task 9: Service (business logic)

**Files:**
- Create: `internal/auth/service.go`
- Create: `internal/auth/service_test.go`

**Step 1: Write failing test**

Create `internal/auth/service_test.go`:

```go
package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Login_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t)
	tenantID := seedTenant(t, db)
	svc := newTestService(t, db)
	ctx := context.Background()

	// Seed a user with a known password
	err := svc.Register(ctx, tenantID, "login@example.com", "CorrectHorse#Battery9", nil, nil)
	require.NoError(t, err)

	resp, refreshToken, err := svc.Login(ctx, tenantID, "login@example.com", "CorrectHorse#Battery9")
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, refreshToken)
}

func TestService_Login_WrongPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t)
	tenantID := seedTenant(t, db)
	svc := newTestService(t, db)
	ctx := context.Background()

	_ = svc.Register(ctx, tenantID, "wrong@example.com", "CorrectHorse#Battery9", nil, nil)

	_, _, err := svc.Login(ctx, tenantID, "wrong@example.com", "wrongpassword")
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestService_HasPermission(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t)
	tenantID := seedTenant(t, db)
	svc := newTestService(t, db)
	ctx := context.Background()

	_ = svc.Register(ctx, tenantID, "perm@example.com", "CorrectHorse#Battery9", nil, nil)
	user, _ := svc.GetUserByEmail(ctx, tenantID, "perm@example.com")

	role, _ := svc.CreateRole(ctx, tenantID, "Support", "support")
	perm, _ := svc.UpsertPermission(ctx, "ticket:read", "Read Tickets", "")
	_ = svc.AssignPermissionsToRole(ctx, role.ID, []uuid.UUID{perm.ID})
	_ = svc.AssignRoleToUser(ctx, user.ID, role.ID)

	assert.True(t, svc.HasPermission(ctx, user.ID, "ticket:read"))
	assert.False(t, svc.HasPermission(ctx, user.ID, "ticket:delete"))
}

func TestService_Refresh(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t)
	tenantID := seedTenant(t, db)
	svc := newTestService(t, db)
	ctx := context.Background()

	_ = svc.Register(ctx, tenantID, "refresh@example.com", "CorrectHorse#Battery9", nil, nil)
	_, refreshToken, _ := svc.Login(ctx, tenantID, "refresh@example.com", "CorrectHorse#Battery9")

	newAccess, newRefresh, err := svc.Refresh(ctx, refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newAccess)
	assert.NotEmpty(t, newRefresh)
	assert.NotEqual(t, refreshToken, newRefresh)

	// Old refresh token must be revoked
	_, _, err = svc.Refresh(ctx, refreshToken)
	assert.Error(t, err)
}

func newTestService(t *testing.T, db *sql.DB) *auth.Service {
	t.Helper()
	return auth.NewService(db, "testsecret-at-least-32-chars-long!", 15*time.Minute, 720*time.Hour)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestService -v -short
```

Expected: skipped. Without -short, fails because `NewService` doesn't exist.

**Step 3: Create `internal/auth/service.go`**

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrSessionExpired     = errors.New("session expired or revoked")
	ErrUserNotFound       = errors.New("user not found")
)

// Service provides authentication and authorization operations.
type Service struct {
	repo       *Repository
	jwtManager *JWTManager
	cache      *PermissionCache
	refreshTTL time.Duration
}

func NewService(db *sql.DB, jwtSecret string, jwtExpiry, refreshTTL time.Duration) *Service {
	return &Service{
		repo:       NewRepository(db),
		jwtManager: NewJWTManager(jwtSecret, jwtExpiry),
		cache:      NewPermissionCache(5 * time.Minute),
		refreshTTL: refreshTTL,
	}
}

// Register creates a new user with a local identity (email + password).
func (s *Service) Register(ctx context.Context, tenantID uuid.UUID, email, password string, firstName, lastName *string) error {
	user, err := s.repo.CreateUser(ctx, tenantID, email, firstName, lastName)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	hash, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = s.repo.CreateIdentity(ctx, user.ID, "local", email, &hash)
	return err
}

// Login verifies credentials and returns an access token + raw refresh token.
func (s *Service) Login(ctx context.Context, tenantID uuid.UUID, email, password string) (*LoginResponse, string, error) {
	identity, err := s.repo.GetIdentityByProvider(ctx, "local", email, tenantID)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}

	if identity.CredentialHash == nil || !verifyPassword(password, *identity.CredentialHash) {
		return nil, "", ErrInvalidCredentials
	}

	user, err := s.repo.GetUserByID(ctx, identity.UserID)
	if err != nil {
		return nil, "", ErrUserNotFound
	}

	accessToken, err := s.jwtManager.Issue(user.ID, tenantID)
	if err != nil {
		return nil, "", fmt.Errorf("issue token: %w", err)
	}

	rawRefresh, err := generateRefreshToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate refresh token: %w", err)
	}

	tokenHash := hashRefreshToken(rawRefresh)
	expiresAt := time.Now().Add(s.refreshTTL)
	_, err = s.repo.CreateSession(ctx, user.ID, tokenHash, expiresAt)
	if err != nil {
		return nil, "", fmt.Errorf("create session: %w", err)
	}

	return &LoginResponse{
		AccessToken: accessToken,
		ExpiresIn:   int(s.jwtManager.expiry.Seconds()),
	}, rawRefresh, nil
}

// Refresh rotates a refresh token and returns a new access token + new refresh token.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (string, string, error) {
	hash := hashRefreshToken(rawRefreshToken)
	session, tenantID, err := s.repo.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return "", "", ErrSessionExpired
	}

	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		return "", "", fmt.Errorf("revoke old session: %w", err)
	}

	accessToken, err := s.jwtManager.Issue(session.UserID, tenantID)
	if err != nil {
		return "", "", fmt.Errorf("issue token: %w", err)
	}

	newRawRefresh, err := generateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	newHash := hashRefreshToken(newRawRefresh)
	expiresAt := time.Now().Add(s.refreshTTL)
	if _, err = s.repo.CreateSession(ctx, session.UserID, newHash, expiresAt); err != nil {
		return "", "", fmt.Errorf("create new session: %w", err)
	}

	return accessToken, newRawRefresh, nil
}

// Logout revokes the session identified by the raw refresh token.
func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	hash := hashRefreshToken(rawRefreshToken)
	session, _, err := s.repo.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		return nil // already gone, not an error
	}
	return s.repo.RevokeSession(ctx, session.ID)
}

// HasPermission checks if a user has the given permission key, using the cache.
func (s *Service) HasPermission(ctx context.Context, userID uuid.UUID, permKey string) bool {
	perms, ok := s.cache.Get(userID)
	if !ok {
		var err error
		perms, err = s.repo.GetUserPermissions(ctx, userID)
		if err != nil {
			return false
		}
		s.cache.Set(userID, perms)
	}
	for _, p := range perms {
		if p == permKey {
			return true
		}
	}
	return false
}

// VerifyToken validates a JWT and returns its claims.
func (s *Service) VerifyToken(token string) (*TokenClaims, error) {
	return s.jwtManager.Verify(token)
}

// SyncPermissions writes all registry permissions to the DB.
// Call once at startup after all init() functions have run.
func (s *Service) SyncPermissions(ctx context.Context) error {
	for _, p := range GlobalRegistry.List() {
		if _, err := s.repo.UpsertPermission(ctx, p.Key, p.Name, p.Description); err != nil {
			return fmt.Errorf("upsert permission %q: %w", p.Key, err)
		}
	}
	return nil
}

func (s *Service) GetUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error) {
	return s.repo.GetUserByEmail(ctx, tenantID, email)
}

func (s *Service) CreateRole(ctx context.Context, tenantID uuid.UUID, name, slug string) (*Role, error) {
	return s.repo.CreateRole(ctx, tenantID, name, slug)
}

func (s *Service) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]Role, error) {
	return s.repo.ListRoles(ctx, tenantID)
}

func (s *Service) AssignPermissionsToRole(ctx context.Context, roleID uuid.UUID, permIDs []uuid.UUID) error {
	return s.repo.AssignPermissionsToRole(ctx, roleID, permIDs)
}

func (s *Service) AssignRoleToUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return s.repo.AssignRoleToUser(ctx, userID, roleID)
}

func (s *Service) UpsertPermission(ctx context.Context, key, name, description string) (*Permission, error) {
	return s.repo.UpsertPermission(ctx, key, name, description)
}

func (s *Service) ListPermissions(ctx context.Context) ([]string, error) {
	perms := GlobalRegistry.List()
	keys := make([]string, len(perms))
	for i, p := range perms {
		keys[i] = p.Key
	}
	return keys, nil
}

// --- crypto helpers ---

const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

func hashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := base64.RawStdEncoding.EncodeToString(salt) + "." + base64.RawStdEncoding.EncodeToString(hash)
	return encoded, nil
}

func verifyPassword(password, encoded string) bool {
	var saltB64, hashB64 string
	if n, _ := fmt.Sscanf(encoded, "%s.%s", &saltB64, &hashB64); n != 2 {
		// fallback: split on "."
		parts := splitDot(encoded)
		if len(parts) != 2 {
			return false
		}
		saltB64, hashB64 = parts[0], parts[1]
	}
	salt, err := base64.RawStdEncoding.DecodeString(saltB64)
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(hashB64)
	if err != nil {
		return false
	}
	actualHash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return constEq(actualHash, expectedHash)
}

func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", h)
}

func splitDot(s string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func constEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
```

**Step 4: Run tests**

```bash
go test ./internal/auth/... -run TestService -v
```

Expected: PASS (integration tests require running DB; unit tests pass without it)

**Step 5: Commit**

```bash
git add internal/auth/service.go internal/auth/service_test.go
git commit -m "feat(tyr): add auth service with login, refresh, and permission check"
```

---

## Task 10: HTTP middleware

**Files:**
- Create: `internal/auth/middleware.go`
- Create: `internal/auth/middleware_test.go`

**Step 1: Write failing test**

Create `internal/auth/middleware_test.go`:

```go
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestRequireAuth_ValidToken(t *testing.T) {
	mgr := auth.NewJWTManager("testsecret-at-least-32-chars-long!", 15*time.Minute)
	token, _ := mgr.Issue(uuid.New(), uuid.New())

	middleware := auth.RequireAuthMiddleware(mgr)
	called := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAuth_MissingToken(t *testing.T) {
	mgr := auth.NewJWTManager("testsecret-at-least-32-chars-long!", 15*time.Minute)
	middleware := auth.RequireAuthMiddleware(mgr)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	mgr := auth.NewJWTManager("testsecret-at-least-32-chars-long!", 15*time.Minute)
	middleware := auth.RequireAuthMiddleware(mgr)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestRequireAuth -v
```

Expected: FAIL — `RequireAuthMiddleware` not defined

**Step 3: Create `internal/auth/middleware.go`**

```go
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const (
	contextKeyUserID   contextKey = "userID"
	contextKeyTenantID contextKey = "tenantID"
)

// RequireAuthMiddleware validates the Bearer JWT and injects userID + tenantID into context.
func RequireAuthMiddleware(jwtManager *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				writeUnauthorized(w, "missing token")
				return
			}

			claims, err := jwtManager.Verify(token)
			if err != nil {
				writeUnauthorized(w, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, contextKeyTenantID, claims.TenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission returns middleware that checks a specific permission key.
// Must be chained after RequireAuthMiddleware.
func RequirePermission(svc *Service, permKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				writeUnauthorized(w, "not authenticated")
				return
			}
			if !svc.HasPermission(r.Context(), userID, permKey) {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserIDFromContext retrieves the authenticated user ID from context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(contextKeyUserID).(uuid.UUID)
	return v, ok
}

// TenantIDFromContext retrieves the tenant ID from context.
func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(contextKeyTenantID).(uuid.UUID)
	return v, ok
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{"error": "forbidden"})
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/... -run TestRequireAuth -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/middleware.go internal/auth/middleware_test.go
git commit -m "feat(tyr): add auth middleware"
```

---

## Task 11: HTTP handlers

**Files:**
- Create: `internal/auth/handler.go`
- Create: `internal/auth/handler_test.go`

**Step 1: Write failing test**

Create `internal/auth/handler_test.go`:

```go
package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Login_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t)
	tenantID := seedTenant(t, db)
	svc := newTestService(t, db)
	h := auth.NewHandler(svc)

	_ = svc.Register(context.Background(), tenantID, "handler@example.com", "CorrectHorse#Battery9", nil, nil)

	body, _ := json.Marshal(auth.LoginRequest{Email: "handler@example.com", Password: "CorrectHorse#Battery9"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String()) // tenant resolved from header in tests
	w := httptest.NewRecorder()

	h.Login(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp auth.LoginResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.NotEmpty(t, resp.AccessToken)

	// Check httpOnly cookie was set
	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	require.NotNil(t, refreshCookie)
	assert.True(t, refreshCookie.HttpOnly)
}

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	db := testDB(t)
	tenantID := seedTenant(t, db)
	svc := newTestService(t, db)
	h := auth.NewHandler(svc)

	body, _ := json.Marshal(auth.LoginRequest{Email: "nobody@example.com", Password: "wrongpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", tenantID.String())
	w := httptest.NewRecorder()

	h.Login(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestHandler -v -short
```

Expected: skipped

**Step 3: Create `internal/auth/handler.go`**

```go
package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const refreshCookieName = "refresh_token"

// Handler provides HTTP endpoints for the auth domain.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Login handles POST /api/v1/auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tenantID, err := tenantIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing or invalid X-Tenant-ID header")
		return
	}

	resp, refreshToken, err := h.svc.Login(r.Context(), tenantID, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    refreshToken,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(h.svc.refreshTTL),
	})

	writeJSON(w, http.StatusOK, resp)
}

// Logout handles POST /api/v1/auth/logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err == nil {
		_ = h.svc.Logout(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}

// Refresh handles POST /api/v1/auth/refresh
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	newAccess, newRefresh, err := h.svc.Refresh(r.Context(), cookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    newRefresh,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(h.svc.refreshTTL),
	})

	writeJSON(w, http.StatusOK, map[string]string{"access_token": newAccess})
}

// Me handles GET /api/v1/auth/me
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	tenantID, _ := TenantIDFromContext(r.Context())

	user, err := h.svc.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not fetch user")
		return
	}

	roles, _ := h.svc.ListRoles(r.Context(), tenantID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":         user.ID,
		"email":      user.Email,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"roles":      roles,
	})
}

// ListRoles handles GET /api/v1/auth/roles
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := TenantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	roles, err := h.svc.ListRoles(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list roles")
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

// CreateRole handles POST /api/v1/auth/roles
func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := TenantIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	role, err := h.svc.CreateRole(r.Context(), tenantID, req.Name, req.Slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create role")
		return
	}
	writeJSON(w, http.StatusCreated, role)
}

// --- helpers ---

func tenantIDFromRequest(r *http.Request) (uuid.UUID, error) {
	// In production, this comes from the JWT (set by RequireAuthMiddleware).
	// During login (pre-auth), read from X-Tenant-ID header or subdomain.
	h := r.Header.Get("X-Tenant-ID")
	return uuid.Parse(h)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
```

**Step 4: Run tests**

```bash
go test ./internal/auth/... -run TestHandler -v
```

Expected: PASS (integration tests pass with real DB)

**Step 5: Commit**

```bash
git add internal/auth/handler.go internal/auth/handler_test.go
git commit -m "feat(tyr): add auth HTTP handlers"
```

---

## Task 12: Týr's own permissions + wire into main.go

**Files:**
- Create: `internal/auth/permissions.go`
- Modify: `cmd/api/main.go`

**Step 1: Create `internal/auth/permissions.go`**

```go
package auth

func init() {
	RegisterPermission(Permission{Key: "user:create",   Name: "Create Users",            Description: "Create new user accounts within the tenant"})
	RegisterPermission(Permission{Key: "user:read",     Name: "View Users",              Description: "View user profiles"})
	RegisterPermission(Permission{Key: "user:update",   Name: "Update Users",            Description: "Update user details"})
	RegisterPermission(Permission{Key: "user:delete",   Name: "Delete Users",            Description: "Delete user accounts"})
	RegisterPermission(Permission{Key: "role:read",     Name: "View Roles",              Description: "View roles and their permissions"})
	RegisterPermission(Permission{Key: "role:manage",   Name: "Manage Roles",            Description: "Create roles and assign permissions"})
}
```

**Step 2: Update `cmd/api/main.go`**

```go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/schererja/yggdrasil/internal/auth"
	"github.com/schererja/yggdrasil/internal/shared"

	// Import domain packages to trigger their init() permission registrations.
	_ "github.com/schererja/yggdrasil/internal/auth"
)

func main() {
	cfg := shared.LoadConfig()

	db, err := shared.ConnectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	authSvc := auth.NewService(db, cfg.JWTSecret, cfg.JWTExpiry, cfg.RefreshTokenExpiry)

	// Sync all registered permissions to the DB.
	if err := authSvc.SyncPermissions(context.Background()); err != nil {
		log.Fatalf("sync permissions: %v", err)
	}

	authHandler := auth.NewHandler(authSvc)
	requireAuth := auth.RequireAuthMiddleware(authSvc.JWTManager())

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login",   authHandler.Login)
		r.Post("/logout",  authHandler.Logout)
		r.Post("/refresh", authHandler.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(requireAuth)
			r.Get("/me", authHandler.Me)

			r.Get("/roles",  authHandler.ListRoles)
			r.With(auth.RequirePermission(authSvc, "role:manage")).Post("/roles", authHandler.CreateRole)
		})
	})

	log.Printf("starting on %s", cfg.APIHost)
	if err := http.ListenAndServe(cfg.APIHost, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

**Step 3: Expose JWTManager from Service**

Add this to `internal/auth/service.go`:

```go
// JWTManager exposes the JWT manager for use in middleware setup.
func (s *Service) JWTManager() *JWTManager {
	return s.jwtManager
}
```

**Step 4: Build and verify**

```bash
go build ./...
```

Expected: no errors

**Step 5: Smoke test**

```bash
# Start dependencies
docker compose up -d postgres
migrate -path migrations -database "postgres://yggdrasil:yggdrasil@localhost:5432/yggdrasil?sslmode=disable" up

# Start the server
go run cmd/api/main.go &

# Health check
curl -s http://localhost:8080/health
# Expected: 200 OK

# Stop server
kill %1
```

**Step 6: Commit**

```bash
git add internal/auth/permissions.go cmd/api/main.go internal/auth/service.go
git commit -m "feat(tyr): wire auth service into main and register Týr permissions"
```

---

## Task 13: Run full test suite and clean up

**Step 1: Run all unit tests**

```bash
go test ./... -short -v
```

Expected: PASS — all unit tests green

**Step 2: Run all integration tests**

```bash
docker compose up -d postgres
migrate -path migrations -database "postgres://yggdrasil:yggdrasil@localhost:5432/yggdrasil?sslmode=disable" up
go test ./... -v
```

Expected: PASS — all tests green

**Step 3: Run linter**

```bash
golangci-lint run ./...
```

Expected: no errors. Fix any issues before committing.

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat(tyr): complete Týr auth service implementation"
```

---

## Summary

After completing all tasks, Týr will provide:

- `POST /api/v1/auth/login` — email + password login, returns JWT + httpOnly refresh cookie
- `POST /api/v1/auth/logout` — revokes session
- `POST /api/v1/auth/refresh` — rotates refresh token
- `GET  /api/v1/auth/me` — returns current user
- `GET  /api/v1/auth/roles` — lists tenant roles
- `POST /api/v1/auth/roles` — creates a tenant role (requires `role:manage`)

All other domain services can register permissions via `init()` in their own `permissions.go` files, and protect routes with `auth.RequirePermission(authSvc, "ticket:create")`.
