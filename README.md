# Yggdrasil

A modern MSP/CRM/ERP platform with a Go control plane, React/TypeScript web app, and Go agent. The repository is a monorepo with shared tooling, CI/CD, and architecture docs.

## Overview

Yggdrasil is designed for MSPs, home businesses, and small businesses. It provides a comprehensive platform for ticket management, client portal access, and remote agent monitoring with a focus on data portability, transparency, and extensibility.

### Core Principles

- **No black boxes**: Data is fully exportable and importable
- **Extensible**: Plugin-based ACL system for fine-grained permissions
- **Transparent**: Clear architecture with documented APIs

## Components

### Control Plane (Go)

Core APIs built with Chi router, sqlc for type-safe SQL, gRPC for agents, multi-tenant RLS on PostgreSQL/TimescaleDB.

### Web App (React/TypeScript)

Staff and client portal UI built with Vite, TailwindCSS, shadcn/ui, TanStack Query/Table.

### Agent (Go)

Lightweight system agent with multi-language plugin support (go-plugin), mTLS to control plane, NATS task queue integration. Currently collects: disk, CPU, memory, temperature, and network metrics.

### Shared Packages

Protobuf definitions, SDKs (agent, control plane), shared TypeScript utilities.

## Architecture

```
┌─────────────┐     ┌─────────────────┐     ┌─────────────┐
│   Web App   │────▶│  Control Plane  │◀────│    Agent    │
│  (React)    │     │      (Go)       │     │    (Go)     │
└─────────────┘     └────────┬────────┘     └─────────────┘
                            │
                     ┌──────▼──────┐
                     │ PostgreSQL  │
                     │ (Timescale) │
                     └─────────────┘
```

## Services

The system is organized into domain services, each named after Norse concepts:

- **Týr**: Authentication & Identity (users, roles, permissions, ACL plugins)
- **Mímir**: Work & Knowledge (tickets, tasks, time tracking)
- **Valhalla**: Administration (tenant management, system configuration)
- **Bifröst**: Customer Portal (client access, self-service)
- **Heimdallr**: Monitoring (health, metrics)
- **Urd**: Audit & Logging (compliance, trails)
- **Smidr**: Edge Computing (agent orchestration)

## Repository Layout

```
yggdrasil/
├── cmd/
│   ├── api/              # Control plane entry point
│   └── agent/            # Agent entry point
├── internal/             # Private application code
│   ├── ticket/           # Ticket domain (vertical slice)
│   ├── client/           # Client domain
│   ├── agent/            # Agent domain
│   ├── auth/             # Authentication domain
│   ├── platform/         # Platform infrastructure
│   └── shared/           # Shared utilities
├── pkg/                  # Public packages (if needed)
├── web/                  # React frontend
├── packages/             # proto definitions, agent-sdk
├── migrations/           # Database migrations (golang-migrate)
├── docs/                 # architecture, guides, API docs
├── scripts/              # setup, cert generation, seed data
├── .github/workflows/    # CI/CD pipelines
├── docker-compose.yml    # local dev stack
├── go.mod                # Go module (root level)
├── sqlc.yaml             # sqlc configuration
├── Makefile              # build automation
└── .env.example          # sample env vars
```

## Prerequisites

- Node.js 20+
- Go 1.22+
- Docker + Docker Compose
- `npm`, `buf` (for protobuf), `make`, `sqlc`

## Quick Start (Local)

1. Clone and copy environment variables

```bash
cp .env.example .env
```

2. Start backing services (PostgreSQL, NATS)

```bash
docker compose up -d postgres nats
```

3. Run database migrations

```bash
migrate -path migrations -database "postgres://localhost/yggdrasil?sslmode=disable" up
```

4. Control Plane (Go)

```bash
go mod download
sqlc generate
go run cmd/api/main.go
```

5. Web App (React)

```bash
cd web
npm ci
npm run dev
```

6. Agent (Go)

```bash
go run cmd/agent/main.go
```

## Testing

- Control Plane & Agent: `go- Web App: test ./...`
 `cd web && npm test`

Coverage targets: backend 80%+, frontend 70%+.

## CI/CD

GitHub Actions run lint, tests, integration tests, build verification, security scans, and release workflows (Docker images + binaries). See docs/architecture/09-ci-cd-pipeline.md for full pipeline and branch protection rules.

## Documentation

Comprehensive documentation is available in the `docs/` directory:

- **[Documentation Overview](docs/README.md)** - All documentation organized by topic
- **[Architecture](docs/architecture/README.md)** - System design and technical architecture
- **[Deployment](docs/deployment/DEPLOYMENT.md)** - Production deployment guides
- **[API Integration](docs/api/)** - API documentation and SDKs
- **[Security](docs/security/)** - Security guidelines and compliance

Start with the [Documentation Overview](docs/README.md) for complete navigation.

## MVP Roadmap

### v0.5 (MVP, 14-16 weeks)

- Multi-tenant ticketing with create, assignment, status updates
- Client and system management
- Agent metrics collection (disk, CPU, memory, network)
- Row-Level Security (RLS) on PostgreSQL
- Basic dashboards

### v1.0

- Billing and invoicing
- Task execution via agent
- Email-to-ticket
- Client portal with full ticket view
- OAuth/SSO integration

### v1.5+

- Plugin SDK/marketplace
- Advanced metrics and alerting
- Reporting and analytics

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## Code of Conduct

Please follow our [Code of Conduct](CODE_OF_CONDUCT.md) to keep our community approachable and respectable.

## License

[License name] - see LICENSE file for details.
