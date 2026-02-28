# AI Assistant Guide (AGENTS.md)

This file provides guidance for AI assistants (like yourself) working on the Yggdrasil project.

## Project Overview

Yggdrasil is an MSP/CRM/ERP platform with:
- **Control Plane (Go)**: REST APIs with Chi router, sqlc for type-safe SQL, gRPC for agent communication
- **Web App (React/TypeScript)**: Staff and client portal with Vite, TailwindCSS, shadcn/ui
- **Agent (Go)**: System metrics collector with mTLS to control plane, NATS integration
- **Database**: PostgreSQL with TimescaleDB for time-series data, multi-tenant RLS

## MVP Scope (v0.5)

### Build Order

1. **Infrastructure First**
   - Docker (PostgreSQL compose setup, NATS)
   - Database migrations with golang-migrate
   - sqlc configuration for type-safe SQL

2. **Týr (Authentication)**
   - User management with tenant isolation
   - JWT-based authentication
   - **ACL Plugin System**: Permission keys like `ticket:create`, `ticket:read`, `ticket:assign`
   - Implement as a registry/plugin pattern for extensibility

3. **Valhalla (Administration)**
   - Tenant management
   - System configuration

4. **Mímir (Tickets)**
   - CRUD operations
   - Assignment workflow
   - Status updates (open, in-progress, resolved, closed)

5. **Bifröst (Client Portal)**
   - Read-only access for clients to view their tickets
   - Client self-service basics

6. **Heimdallr (Monitoring)**
   - Health check endpoints
   - Agent metrics storage (TimescaleDB)

7. **Agent**
   - Collect: disk, CPU, memory, temperature, network metrics
   - mTLS connection to control plane
   - NATS task queue for command/control

8. **Web UI**
   - Staff dashboard
   - Client portal views

## Tech Stack

| Component | Technology |
|-----------|-------------|
| Backend | Go 1.22+, Chi router, sqlc, gRPC |
| Database | PostgreSQL, TimescaleDB |
| Message Queue | NATS |
| Frontend | React 18+, TypeScript, Vite, TailwindCSS, shadcn/ui |
| Testing | Go testing, Vitest/React Testing Library |
| Migrations | golang-migrate |
| Protobuf | buf, grpc |

## Key Conventions

### Go Code

- Use `sqlc generate` after any SQL changes
- Run `golangci-lint run` before committing
- Follow standard Go project layout (cmd/, internal/, pkg/)
- Use vertical slice architecture (group by domain, not layer)

### TypeScript/React Code

- Run `npm run lint` before committing
- Use TypeScript for all files
- Follow shadcn/ui patterns for components
- Use TanStack Query for data fetching

### Database

- All migrations in `migrations/` directory
- Use golang-migrate for migration management
- Multi-tenant via RLS (Row-Level Security)
- Use sqlc for type-safe queries

### API Design

- RESTful endpoints with Chi router
- JSON request/response format
- Version API as `/api/v1/...`
- Use gRPC for agent communication

### ACL System (Týr)

The ACL system should be plugin-based with permission keys:

```go
// Example permission structure
type Permission struct {
    Key   string // e.g., "ticket:create", "ticket:read", "ticket:assign"
    Name  string
    Description string
}

// Permission registry - extensible via plugins
type PermissionRegistry interface {
    Register(permission Permission)
    GetAll() []Permission
    HasPermission(user *User, permission string) bool
}
```

Permission format: `resource:action` (e.g., `ticket:create`, `client:read`, `agent:configure`)

## File Locations

```
yggdrasil/
├── cmd/api/main.go          # Control plane entry
├── cmd/agent/main.go        # Agent entry
├── internal/
│   ├── auth/                # Týr - Authentication & ACL
│   ├── ticket/              # Mímir - Tickets
│   ├── client/              # Client management
│   ├── agent/               # Agent management
│   ├── platform/            # Infrastructure
│   └── shared/              # Utilities
├── web/src/                 # React frontend
├── packages/                # Protobuf definitions
├── migrations/              # Database migrations
└── docs/                    # Architecture docs
```

## Commands

```bash
# Backend
go test ./...                           # Run all tests
go run cmd/api/main.go                   # Start control plane
go run cmd/agent/main.go                 # Start agent
make lint                                # Run linter (if Makefile exists)

# Frontend
cd web && npm run dev                   # Start dev server
cd web && npm test                       # Run tests
cd web && npm run lint                   # Lint

# Database
migrate -path migrations -database "postgres://localhost/yggdrasil?sslmode=disable" up
sqlc generate                            # Generate type-safe SQL
```

## Important Notes

1. **Data Portability**: Design with export/import in mind. Avoid proprietary data formats.
2. **No Black Boxes**: Keep APIs documented, data structures clear.
3. **Extensibility**: Use plugin patterns where appropriate (especially ACL).
4. **Multi-tenancy**: Always consider tenant isolation via RLS.
5. **Security**: Use mTLS for agent connections, proper password hashing, secure JWT handling.

## Documentation

Before making significant changes, check the docs folder:
- `docs/architecture/` - System design
- `docs/api/` - API documentation
- `docs/security/` - Security guidelines

## Testing Requirements

- Backend: 80%+ coverage target
- Frontend: 70%+ coverage target
- Include integration tests for critical paths

## Questions?

If you're unsure about architecture decisions or implementation details:
1. Check the docs/architecture/ folder first
2. Look at existing code patterns in the relevant domain
3. Ask clarifying questions before implementing
