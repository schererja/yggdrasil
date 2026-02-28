# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Local Dev Setup

```bash
cp .env.example .env
docker compose up -d postgres nats
migrate -path migrations -database "postgres://localhost/yggdrasil?sslmode=disable" up
```

### Control Plane (Go)

```bash
go mod download
sqlc generate          # Must run after any SQL changes
go run cmd/api/main.go
go test ./...          # Run all tests
go test ./internal/ticket/... # Run a single domain's tests
golangci-lint run      # Lint (run before committing)
```

### Agent (Go)

```bash
go run cmd/agent/main.go
```

### Web App (React/TypeScript)

```bash
cd web
npm ci
npm run dev
npm test
npm run lint           # Run before committing
```

### Protobuf

```bash
buf generate           # Regenerate protobuf/gRPC code
```

## Architecture

Yggdrasil is a **modular monolith** designed to evolve into microservices. Three runtime components:

- **Control Plane** (`cmd/api/`) — single Go binary exposing REST (`/api/v1/...` via Chi) and gRPC (for agents). All domain services live here.
- **Agent** (`cmd/agent/`) — lightweight Go binary running on managed endpoints; connects to control plane via gRPC/mTLS and uses NATS for task queuing.
- **Web App** (`web/`) — React/TypeScript SPA with Vite, TailwindCSS, shadcn/ui, TanStack Query/Table.

### Domain Services (Norse-Themed)

All domain logic lives under `internal/` using **vertical slice architecture** (group by domain, not layer):

| Service | Package | Responsibility |
|---------|---------|----------------|
| **Týr** | `internal/auth/` | Auth (JWT), users, roles, plugin-based ACL |
| **Mímir** | `internal/ticket/` | Tickets, tasks, time tracking |
| **Valhalla** | `internal/platform/` | Tenant admin, system configuration |
| **Bifröst** | (portal sub-package) | Read-only client portal access |
| **Heimdallr** | `internal/agent/` | Agent metrics (TimescaleDB), health checks |
| **Urd** | (audit sub-package) | Subscribes to all NATS events for compliance logging |
| **Smidr** | (agent sub-package) | Agent lifecycle and orchestration |

### Data & Messaging

- **PostgreSQL + TimescaleDB** — primary store. Multi-tenancy enforced via **Row-Level Security (RLS)**; every query must respect tenant context.
- **NATS** — async event bus. Subject hierarchy: `{domain}.{entity}.{action}` (e.g. `ticket.ticket.created`, `agent.agent.registered`). Urd subscribes to all events (`*`).
- **sqlc** — all DB queries are written as raw SQL under `internal/*/queries/` and code-generated. Never write raw `database/sql` calls by hand.

### ACL System (Týr)

Plugin-based permission registry. Permissions use `resource:action` format (`ticket:create`, `client:read`, `agent:configure`). New domains register their permissions via `init()` functions into the central registry.

```go
type PermissionRegistry interface {
    Register(permission Permission)
    HasPermission(user *User, permission string) bool
}
```

### Agent Communication

Agents connect to the control plane over **gRPC with mTLS**. Protobuf definitions live in `packages/`. For local dev, mTLS cert paths can be left blank (see `.env.example`).

## Git Workflow

### Branch Naming

Branches must follow the pattern: `<type>/<short-description>`

| Type | Use for |
|------|---------|
| `feature/` | New functionality |
| `fix/` | Bug fixes |
| `chore/` | Maintenance, deps, tooling |
| `docs/` | Documentation only |
| `refactor/` | Code restructuring without behavior change |
| `test/` | Adding or fixing tests |
| `ci/` | CI/CD pipeline changes |

Examples: `feature/tyr-jwt-auth`, `fix/ticket-rls-query`, `chore/update-go-deps`

### Conventional Commits

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/) spec:

```
<type>(<optional scope>): <description>

[optional body]

[optional footer]
```

**Types:** `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `perf`, `style`, `build`

**Scopes** (optional, use the service/domain name): `tyr`, `mimir`, `valhalla`, `bifrost`, `heimdallr`, `urd`, `agent`, `web`, `api`, `migrations`

Examples:
```
feat(mimir): add time tracking to tickets
fix(tyr): correct JWT expiry validation
chore(deps): update Go modules
docs(api): document ticket endpoints
```

Breaking changes must include `BREAKING CHANGE:` in the footer or append `!` after the type/scope:
```
feat(tyr)!: replace session auth with JWT
```

### Pull Request Process

1. **Branch from `develop`** — never work directly on `main` or `develop`.
2. **Keep PRs focused** — one feature/fix per PR; avoid mixing concerns.
3. **PR title** must follow Conventional Commits format (same as commit messages).
4. **PR description** must include:
   - Summary of changes
   - Testing steps or test plan
   - Any migration steps required
5. **Before opening a PR**, ensure:
   - All tests pass: `go test ./...` and `npm test`
   - Linters pass: `golangci-lint run` and `npm run lint`
   - `sqlc generate` is up to date if SQL changed
6. **Target branch:** `develop` (PRs to `main` are release-only).
7. **Squash or rebase** — keep commit history clean; no merge commits on feature branches.

## Key Conventions

- **Multi-tenancy**: Every DB query must filter by tenant. RLS is the enforcement layer but application code must set the correct tenant context.
- **API versioning**: All REST routes are prefixed `/api/v1/`.
- **Data portability**: Avoid proprietary formats; all data must be exportable/importable.
- **Migrations**: Use `golang-migrate` for all schema changes. Migration files go in `migrations/`.
- **Type-safe SQL**: Add queries in SQL files, run `sqlc generate`, use the generated types — never bypass sqlc with raw queries.
- **Frontend data fetching**: Always use TanStack Query; follow shadcn/ui patterns for components.

## MVP Build Order

When adding new features, build in this dependency order:

1. Infrastructure (Docker, migrations, sqlc config)
2. Týr (auth must be first — everything depends on it)
3. Valhalla (tenant management)
4. Mímir (tickets)
5. Bifröst (client portal)
6. Heimdallr (monitoring)
7. Agent
8. Web UI

## Documentation

Architecture decisions and full API docs are in `docs/`:

- `docs/architecture/` — system design, service boundaries, event catalog, DB schema
- `docs/api/` — REST endpoint reference
- `docs/security/SECURITY.md` — auth, mTLS, RLS guidelines
- `docs/deployment/DEPLOYMENT.md` — production deployment

Check `docs/architecture/` before making significant structural changes.
