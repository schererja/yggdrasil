# Authentication & ACL Architecture

## Overview

Týr provides centralized authentication with an extensible ACL (Access Control List) plugin system.

## Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Clients                                     │
│  ┌─────────────────┐              ┌─────────────────┐                   │
│  │   Web App       │              │     Agent       │                   │
│  │ (React/TypeScript)│            │     (Go)        │                   │
│  └────────┬────────┘              └────────┬────────┘                   │
│           │                                │                            │
│           │         JWT / mTLS              │                            │
└───────────┼────────────────────────────────┼────────────────────────────┘
            │                                │
            ▼                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         Týr (Auth Service)                              │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                      Auth Handlers                               │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐            │   │
│  │  │  /login      │  │  /logout     │  │  /refresh   │            │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘            │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                    │                                     │
│                                    ▼                                     │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    Permission Registry                          │   │
│  │  ┌───────────────────────────────────────────────────────────┐  │   │
│  │  │  Registered Permissions (Plugin-style)                    │  │   │
│  │  │                                                           │  │   │
│  │  │   ticket:create    ticket:read      ticket:assign        │  │   │
│  │  │   ticket:update    ticket:delete    ticket:close         │  │   │
│  │  │   client:create   client:read      client:update        │  │   │
│  │  │   agent:configure agent:read       agent:delete         │  │   │
│  │  │   admin:*                                             │  │   │
│  │  │                                                           │  │   │
│  │  │   [+ extensible plugins can register more...]           │  │   │
│  │  └───────────────────────────────────────────────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                    │                                     │
│                                    ▼                                     │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    Permission Checker                           │   │
│  │                                                                  │   │
│  │   func HasPermission(user, permission) bool                    │   │
│  │                                                                  │   │
│  │   1. Get user's role                                            │   │
│  │   2. Look up role → permissions mapping                         │   │
│  │   3. Check if permission key exists                              │   │
│  │                                                                  │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         Database (PostgreSQL)                          │
│                                                                          │
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│   │   tenants   │  │    users    │  │permissions  │  │role_perms  │     │
│   └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘     │
│        │                │                │                │             │
│        │                │                │         ┌──────┴──────┐      │
│        │                │                │         │             │      │
│        └────────────────┴────────────────┴─────────┤             │      │
│                                                    │             │      │
│                                              ┌─────┴─────┐  ┌────┴────┐ │
│                                              │  tenant1  │  │ tenant2 │ │
│                                              │  (RLS)    │  │  (RLS)  │ │
│                                              └───────────┘  └─────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

## Permission Flow

```
Client Request
     │
     ▼
┌────────────────┐
│  Auth Middleware│
│  (Verify JWT)  │
└───────┬────────┘
        │
        ▼ (valid token)
┌────────────────┐
│  Extract User  │
│  + Tenant ID   │
└───────┬────────┘
        │
        ▼
┌────────────────┐     ┌─────────────────┐
│  Handler       │────▶│ Permission      │
│  (e.g.,        │     │ Checker         │
│   CreateTicket)│     │                 │
└───────┬────────┘     │ HasPermission(  │
        │              │   user,         │
        │              │   "ticket:create"│
        │              │ ) = true/false  │
        │              └────────┬────────┘
        │                       │
        ▼                       ▼
┌────────────────┐     ┌────────────────┐
│  Permission    │     │   403 Forbidden│
│  Granted!      │     │   (or continue)│
│  Execute       │     └────────────────┘
│  business logic│
└────────────────┘
```

## Role-to-Permission Mapping

| Role    | Permissions |
|---------|-------------|
| `admin` | All permissions (`admin:*`) |
| `staff` | `ticket:*`, `client:read`, `agent:read`, `user:read` |
| `client` | `ticket:read:own`, `client:read:own` |

## Adding New Permissions (Plugin Style)

```go
// In a domain module (e.g., ticket service)
func init() {
    auth.RegisterPermission(auth.Permission{
        Key:         "ticket:close",
        Name:        "Close Tickets",
        Description: "Allow closing tickets",
    })
}
```

## Multi-Tenant Isolation

Every permission check operates within a tenant context:

```go
func (s *Service) HasPermission(ctx context.Context, userID uuid.UUID, perm string) bool {
    // 1. Get user's tenant_id from database
    // 2. Check role_permissions within that tenant
    // 3. Return true/false
}
```

RLS ensures data is automatically scoped to the user's tenant.
