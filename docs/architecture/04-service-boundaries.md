# Service Boundaries & Communication

## Overview

Yggdrasil v0.5 is structured as a **modular monolith** with clear service boundaries. All domains run in a single Go binary, but are organized as independent services communicating via internal interfaces and NATS events. This design allows seamless extraction to full microservices in v1.0+.

## Service Directory

### Týr (Authentication & ACL)

- **Responsibility**: User authentication, JWT token generation, permission verification, ACL management
- **Owner**: Authentication bounded context
- **Scope**: Multi-tenant user/role/permission system
- **Port**: `/api/v1/auth/*` (endpoints only, runs in shared binary)

**Exposed Interface**:

```go
type AuthService interface {
    Login(ctx context.Context, email, password string) (*JWT, error)
    Refresh(ctx context.Context, token string) (*JWT, error)
    GetUser(ctx context.Context, userID uuid.UUID) (*User, error)
    HasPermission(ctx context.Context, userID uuid.UUID, permission string) (bool, error)
    RegisterPermission(perm Permission)
}
```

**Dependencies**: None (foundational service)

---

### Mímir (Ticket Management)

- **Responsibility**: Ticket CRUD, task management, status workflows
- **Owner**: Ticket/Task bounded context
- **Scope**: Creating, updating, assigning, resolving tickets
- **Port**: `/api/v1/tickets/*` (endpoints)

**Exposed Interface**:

```go
type TicketService interface {
    CreateTicket(ctx context.Context, ticket *Ticket) (*Ticket, error)
    GetTicket(ctx context.Context, ticketID uuid.UUID) (*Ticket, error)
    ListTickets(ctx context.Context, filter TicketFilter) ([]*Ticket, error)
    UpdateTicket(ctx context.Context, ticketID uuid.UUID, updates map[string]interface{}) (*Ticket, error)
    AssignTicket(ctx context.Context, ticketID uuid.UUID, userID uuid.UUID) (*Ticket, error)
    CloseTicket(ctx context.Context, ticketID uuid.UUID) (*Ticket, error)
}
```

**Dependencies**:

- **Synchronous**: Calls `AuthService.HasPermission()` for authorization
- **Asynchronous**: Publishes events to NATS (`ticket.*` topics)

---

### Valhalla (Tenant Administration)

- **Responsibility**: Tenant creation, configuration, settings management
- **Owner**: Tenant/Admin bounded context
- **Scope**: System-wide tenant administration
- **Port**: `/api/v1/admin/*` (endpoints)

**Exposed Interface**:

```go
type TenantService interface {
    CreateTenant(ctx context.Context, name, slug string) (*Tenant, error)
    GetTenant(ctx context.Context, tenantID uuid.UUID) (*Tenant, error)
    UpdateTenant(ctx context.Context, tenantID uuid.UUID, updates map[string]interface{}) (*Tenant, error)
    ListTenants(ctx context.Context) ([]*Tenant, error)
}
```

**Dependencies**:

- **Synchronous**: Calls `AuthService.HasPermission()` for admin checks

---

### Bifröst (Client Portal)

- **Responsibility**: Read-only portal access for clients, self-service views
- **Owner**: UI/Portal bounded context
- **Scope**: Client-facing ticket views, profile management
- **Port**: `/api/v1/portal/*` (endpoints)

**Exposed Interface**:

```go
type PortalService interface {
    GetMyTickets(ctx context.Context, userID uuid.UUID) ([]*Ticket, error)
    GetTicketDetails(ctx context.Context, userID, ticketID uuid.UUID) (*Ticket, error)
    UpdateProfile(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) (*User, error)
}
```

**Dependencies**:

- **Synchronous**: Calls `AuthService.HasPermission()`, reads from `TicketService`

---

### Heimdallr (Monitoring & Metrics)

- **Responsibility**: Agent metric aggregation, health checks, performance analytics
- **Owner**: Monitoring bounded context
- **Scope**: Metric collection, storage, and retrieval
- **Port**: `/api/v1/metrics/*` (REST endpoints), gRPC for agent communication

**Exposed Interface**:

```go
type MonitoringService interface {
    StoreMetrics(ctx context.Context, agentID uuid.UUID, metrics *AgentMetrics) error
    GetMetrics(ctx context.Context, agentID uuid.UUID, timeRange TimeRange) ([]*AgentMetrics, error)
    GetAgentHealth(ctx context.Context, agentID uuid.UUID) (*AgentHealth, error)
    ListAgents(ctx context.Context, tenantID uuid.UUID) ([]*AgentHealth, error)
}

// gRPC service for agents
type MetricsCollector interface {
    ReportMetrics(stream MetricsCollector_ReportMetricsServer) error
}
```

**Dependencies**:

- **Synchronous**: Calls `AuthService.HasPermission()` for API requests
- **Asynchronous**: Listens for `agent.registered` events on NATS
- **Data**: Writes to TimescaleDB (agent_metrics table)

---

### Urd (Audit Logging)

- **Responsibility**: Event logging, audit trail maintenance, compliance tracking
- **Owner**: Audit/Compliance bounded context
- **Scope**: Recording all system actions for audit purposes
- **Port**: N/A (event-driven, no HTTP endpoints)

**Exposed Interface**:

```go
type AuditService interface {
    LogAction(ctx context.Context, action *AuditEntry) error
    GetAuditLog(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error)
}
```

**Dependencies**:

- **Asynchronous**: Subscribes to all NATS events (`*` topics)
- **Data**: Writes to PostgreSQL (audit_log table)

---

### Agent Service (Heimdallr)

- **Responsibility**: Agent lifecycle management, registration, heartbeats
- **Owner**: Agent/Infrastructure bounded context
- **Scope**: Agent tracking and lifecycle
- **Port**: gRPC for agent connections

**Exposed Interface**:

```go
type AgentService interface {
    RegisterAgent(ctx context.Context, agent *Agent) (*Agent, error)
    HeartbeatAgent(ctx context.Context, agentID uuid.UUID) error
    GetAgent(ctx context.Context, agentID uuid.UUID) (*Agent, error)
    DeregisterAgent(ctx context.Context, agentID uuid.UUID) error
}
```

**Dependencies**:

- **Synchronous**: None
- **Asynchronous**: Publishes `agent.*` events to NATS

---

## Communication Patterns

### Synchronous (Function Calls / REST within service)

```
Web App Request
    │
    ├─→ Bifröst Handler
    │      │
    │      ├─ Calls→ AuthService.HasPermission()
    │      ├─ Calls→ TicketService.GetTicket()
    │      └─ Calls→ TenantService.GetTenant()
    │
    └─→ Update cache / return response
```

**Rules**:

- Handlers call services in the same binary
- Services call downstream services synchronously for critical operations
- Always check permissions first

### Asynchronous (NATS Event Bus)

```
Mímir Service creates ticket
    │
    ├─ Publishes: "ticket.created"
    │
    └─→ NATS Event Bus
           │
           ├─→ Urd (Audit) subscribes & logs
           │
           ├─→ Heimdallr subscribes & updates stats
           │
           └─→ Future: Verdandi (Alerts) subscribes & sends notifications
```

**Rules**:

- Services publish domain events after state changes
- Subscribers react independently
- One-way communication (no response expected)
- Use NATS subject hierarchy: `{domain}.{entity}.{action}`

---

## Data Ownership

| Service       | Owns Tables                                           | Read Access              |
| ------------- | ----------------------------------------------------- | ------------------------ |
| **Týr**       | `tenants`, `users`, `permissions`, `role_permissions` | All services             |
| **Mímir**     | `tickets`, `ticket_comments` (future)                 | All services             |
| **Valhalla**  | `tenants` (config)                                    | All services             |
| **Heimdallr** | `agents`, `agent_metrics`                             | All services             |
| **Urd**       | `audit_log`                                           | All services (read-only) |

**Multi-Tenant Rule**: All tables include `tenant_id`; RLS enforces tenant isolation automatically.

---

## Future Microservices Extraction

When evolving to v1.0 microservices, the boundaries are already in place:

```
Monolith (v0.5)          →        Microservices (v1.0+)
─────────────────────────          ──────────────────────
internal/auth/           →        services/auth-service/
internal/ticket/         →        services/ticket-service/
internal/tenant/         →        services/tenant-service/
internal/audit/          →        services/audit-service/
internal/monitoring/     →        services/monitoring-service/
pkg/events/ (NATS)       →        Remains (shared NATS cluster)
pkg/db/ (shared DB)      →        Split per service (schema per service)
```

Each service becomes a separate Go binary with its own Docker image, while NATS remains the central event bus.
