# System Overview

## Architecture Diagram (v0.5 - Modular Monolith)

```
┌─────────────────────────────────────────────────────────────────────┐
│                              Users                                   │
├─────────────────────────────┬───────────────────────────────────────┤
│        Staff Users          │           Client Users                │
│    (Web App - Dashboard)   │      (Web App - Client Portal)        │
└──────────────┬──────────────┴──────────────┬────────────────────────┘
               │                              │
               │         HTTPS/REST           │
               ▼                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Control Plane (Single Binary)                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │   Týr        │  │  Mímir       │  │  Valhalla    │               │
│  │ (Auth/ACL)   │  │ (Tickets)    │  │ (Tenants)    │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│  │  Bifröst     │  │ Heimdallr    │  │    Urd       │               │
│  │ (Portal)     │  │ (Monitoring) │  │  (Audit)     │               │
│  └──────────────┘  └──────────────┘  └──────────────┘               │
│                                                                       │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Internal Communication:                                      │  │
│  │  - Service-to-service: Direct function calls                 │  │
│  │  - Async: NATS event bus (ticket.*, agent.*, etc.)          │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                              │                                       │
│                    ┌─────────┴─────────┐                            │
│                    │   Chi Router      │                            │
│                    │   (REST API)       │                            │
│                    └─────────┬─────────┘                            │
└──────────────────────────────┼──────────────────────────────────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
              ▼                ▼                ▼
    ┌─────────────────┐ ┌───────────┐ ┌─────────────────┐
    │  PostgreSQL     │ │   NATS    │ │  TimescaleDB    │
    │  (Shared DB)    │ │  (Events) │ │  (Metrics)      │
    └─────────────────┘ └───────────┘ └─────────────────┘
              │
              │ gRPC/mTLS
              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                           Agents                                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐                 │
│  │   Agent 1   │  │   Agent 2   │  │   Agent N   │                 │
│  │ (Metrics:   │  │ (Metrics:    │  │ (Metrics:    │                 │
│  │  CPU, Disk, │  │  CPU, Disk, │  │  CPU, Disk,  │                 │
│  │  Mem, Net)  │  │  Mem, Net)  │  │  Mem, Net)   │                 │
│  └─────────────┘  └─────────────┘  └─────────────┘                 │
└─────────────────────────────────────────────────────────────────────┘
```

**Key Design Choice**: All services run in a **single Go binary** (monolith) but are organized as **independent domains** with clear boundaries. This design allows seamless extraction to full microservices in v1.0+ without rewriting core logic.

## Components

### Control Plane (Go - Single Binary, Multiple Services)

All services run within a single Go binary but are organized as independent domains with clear boundaries. This allows seamless extraction to separate microservices in v1.0+.

- **Týr** - Authentication, JWT tokens, user management, ACL plugin system
- **Mímir** - Tickets, tasks, ticket lifecycle management
- **Valhalla** - Tenant management, system configuration
- **Bifröst** - Client portal access and views
- **Heimdallr** - Health checks, agent lifecycle, metrics aggregation
- **Urd** - Audit logging (event-driven, reactive)

**See also**: [Service Boundaries & Communication](04-service-boundaries.md) for detailed service contracts.

### Web App (React/TypeScript)

- Staff dashboard for ticket management
- Client portal for self-service
- Real-time updates via polling/websockets

### Agent (Go)

Lightweight collector running on client machines:

- CPU metrics
- Memory usage
- Disk space
- Network statistics
- Temperature (where available)

### Database Layer

- **PostgreSQL** - Main relational data (users, tickets, clients)
- **TimescaleDB** - Time-series metrics from agents
- **Row-Level Security** - Multi-tenant isolation

### Message Queue

- **NATS** - Agent command/control, async task processing

## Data Flow

### Ticket Creation Flow

1. Staff user logs in via Týr (JWT issued)
2. User creates ticket via REST API (Mímir handler)
3. Mímir verifies permissions with Týr
4. Ticket stored in PostgreSQL
5. **Mímir publishes `ticket.ticket.created` event to NATS**
6. **Urd subscribes and creates audit log entry**
7. Response returned to client

### Agent Metrics Flow

1. Agent connects via mTLS/gRPC
2. Agent sends metrics every interval
3. Heimdallr stores metrics in TimescaleDB
4. **Heimdallr publishes `agent.metrics.received` event to NATS**
5. API exposes metrics via `/api/v1/metrics/*` endpoints
6. Dashboard displays charts

**See also**: [Event Catalog](05-event-catalog.md) for all events and subscribers.

## API Design

- RESTful with JSON
- Versioned: `/api/v1/...`
- gRPC for agent communication
- JWT Bearer tokens for auth

## Security

- mTLS for agent connections
- JWT for API authentication
- RLS for tenant isolation
- Password hashing (bcrypt/argon2)
- HTTPS in production

## Documentation Guide

- **[Database Schema](02-database-schema.md)** - Tables, relationships, RLS policies
- **[Authentication & ACL](03-auth-acl.md)** - Týr service, permission system, multi-tenant isolation
- **[Service Boundaries](04-service-boundaries.md)** - Service contracts, dependencies, communication patterns
- **[Event Catalog](05-event-catalog.md)** - All NATS events, publishers, subscribers
- **[Code Structure](06-code-structure.md)** - Project layout, package organization, domain service pattern

## Scalability Considerations

### v0.5 (Modular Monolith)

- Single binary can be scaled horizontally (run multiple instances behind load balancer)
- NATS for distributed event processing
- TimescaleDB for efficient time-series queries
- Connection pooling via PgBouncer (production)

### v1.0+ (Microservices)

- Each domain service (Týr, Mímir, Valhalla, etc.) becomes independent
- Separate Docker containers per service
- Database per service (or schema per service) with shared NATS bus
- Service discovery (Docker DNS locally, Consul/K8s in production)
- **See**: [Service Boundaries & Communication](04-service-boundaries.md) for extraction plan
