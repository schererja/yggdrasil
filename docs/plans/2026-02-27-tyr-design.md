# Týr Auth Service Design

**Date:** 2026-02-27
**Status:** Approved

## Overview

Týr is the centralized authentication and authorization service for Yggdrasil. It provides JWT-based auth, tenant-configurable roles, a plugin-based permission registry, and a path to OAuth without structural changes.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Role model | Tenant-configurable | Each tenant defines their own roles and assigns permissions |
| Permission resolution | Hybrid in-memory cache | JWT carries only user_id + tenant_id; permissions cached 5min TTL, DB on miss |
| Token refresh | Stored refresh tokens | Revocable sessions; DB stores SHA-256 hash, raw token in httpOnly cookie |
| Auth method | Email + password (argon2id) | OAuth-ready via `identities` table; add providers without schema changes |
| Identity model | `users` + `identities` tables | One user, multiple credential providers; clean OAuth expansion path |

## Data Model

### Tables

```sql
-- Canonical user record (no credentials)
users (
    id          UUID PRIMARY KEY,
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    email       VARCHAR(255) NOT NULL,
    first_name  VARCHAR(100),
    last_name   VARCHAR(100),
    is_active   BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, email)
)

-- Credentials and future OAuth identities
identities (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id),
    provider        VARCHAR(50) NOT NULL,   -- 'local' | 'google' | 'github'
    provider_id     VARCHAR(255) NOT NULL,  -- email for local, OAuth subject for others
    credential_hash VARCHAR(255),           -- argon2id hash for 'local', NULL for OAuth
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(provider, provider_id)
)

-- Tenant-defined roles
roles (
    id          UUID PRIMARY KEY,
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(100) NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, slug)
)

-- User-to-role assignments
user_roles (
    user_id UUID NOT NULL REFERENCES users(id),
    role_id UUID NOT NULL REFERENCES roles(id),
    PRIMARY KEY (user_id, role_id)
)

-- Global permission registry (synced from code at startup)
permissions (
    id          UUID PRIMARY KEY,
    key         VARCHAR(100) UNIQUE NOT NULL,  -- 'ticket:create', 'agent:configure'
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW()
)

-- Role-to-permission assignments (per tenant, via role ownership)
role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id),
    permission_id UUID NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role_id, permission_id)
)

-- Active refresh token sessions
user_sessions (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id),
    token_hash  VARCHAR(64) NOT NULL,  -- SHA-256 of raw refresh token
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    revoked_at  TIMESTAMPTZ
)
```

### RLS

RLS enabled on `users`, `roles`, `user_roles`, `role_permissions`, `user_sessions` — all scoped to `tenant_id` (directly or via join through `users.tenant_id`).

`permissions` has no `tenant_id` — it is a global registry. Tenants assign permissions to roles but cannot create new permission keys.

## Token Strategy

### Access Token (JWT)

- Expiry: 15 minutes
- Algorithm: HS256
- Claims: `sub` (user_id), `tid` (tenant_id), `exp`, `iat`
- No permissions embedded — resolved via cache on each request

### Refresh Token

- Format: 32 random bytes, base64url-encoded
- Transport: `httpOnly`, `Secure`, `SameSite=Strict` cookie
- Storage: SHA-256 hash stored in `user_sessions`; raw token never persisted
- Expiry: 30 days
- Rotation: on each refresh, old session is revoked and a new session row is inserted

## Permission Cache

- Storage: in-process `sync.Map` keyed by `user_id`
- Value: `[]string` of permission keys + timestamp
- TTL: 5 minutes
- Invalidation: eager invalidation on role or permission assignment changes
- Miss path: DB query joining `user_roles → role_permissions → permissions`

## Plugin Permission Registry

Each domain registers its permissions via `init()` before `main()` runs:

```go
// internal/ticket/permissions.go
func init() {
    auth.RegisterPermission(auth.Permission{Key: "ticket:create", Name: "Create Tickets"})
    auth.RegisterPermission(auth.Permission{Key: "ticket:read",   Name: "View Tickets"})
    // ...
}
```

At startup, the service syncs the in-memory registry to the `permissions` table — inserting new keys, leaving existing ones untouched. The DB always reflects what the running binary supports.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/login` | Email + password login; returns access JWT + sets refresh cookie |
| POST | `/api/v1/auth/logout` | Revokes current session; clears cookie |
| POST | `/api/v1/auth/refresh` | Rotates refresh token; returns new access JWT |
| GET  | `/api/v1/auth/me` | Returns current user profile and roles |
| GET  | `/api/v1/auth/roles` | Lists tenant roles (requires `role:read`) |
| POST | `/api/v1/auth/roles` | Creates a tenant role (requires `role:manage`) |
| PUT  | `/api/v1/auth/roles/:id/permissions` | Assigns permissions to a role (requires `role:manage`) |

## Code Structure

```
internal/auth/
├── handler.go       # HTTP handlers: login, logout, refresh, me, roles
├── service.go       # Business logic interface + implementation
├── repository.go    # DB queries (sqlc-backed)
├── permission.go    # Registry, RegisterPermission(), HasPermission()
├── cache.go         # In-memory permission cache (sync.Map + TTL)
├── jwt.go           # Token generation + verification
├── middleware.go    # RequireAuth, RequirePermission() middleware
├── types.go         # User, Identity, Role, Session, Permission structs
└── permissions.go   # Týr's own permission keys (user:create, role:manage, etc.)
```

### Middleware Usage

```go
r.With(auth.RequireAuth).Get("/api/v1/auth/me", authHandler.Me)
r.With(auth.RequireAuth, auth.RequirePermission("ticket:create")).Post("/api/v1/tickets", ticketHandler.Create)
```

`RequireAuth` validates the JWT and sets `userID` + `tenantID` in the request context. `RequirePermission` calls `HasPermission` using those context values.

## OAuth Expansion Path

To add Google OAuth:
1. Add `provider: 'google'` row handling in the login/callback flow
2. Insert a new `identities` row linking the Google subject to an existing or new `users` row
3. No changes to roles, permissions, sessions, or JWT structure

## Migration Plan

```
migrations/
├── 001_tenants.up.sql          # (existing)
├── 002_users.up.sql            # users table + RLS
├── 003_identities.up.sql       # identities table
├── 004_roles.up.sql            # roles, user_roles + RLS
├── 005_permissions.up.sql      # permissions, role_permissions
├── 006_user_sessions.up.sql    # user_sessions + RLS
```
