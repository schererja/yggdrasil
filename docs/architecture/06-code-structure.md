# Code Structure & Organization

## Project Layout (v0.5 Modular Monolith)

```
yggdrasil/
├── cmd/
│   └── api/
│       ├── main.go              # Entry point for control plane
│       └── handlers.go           # HTTP route definitions
│
├── internal/                     # Private packages (not importable from outside)
│   ├── auth/                     # Týr - Authentication & ACL
│   │   ├── handler.go            # HTTP handlers (/api/v1/auth/*)
│   │   ├── service.go            # Business logic interface
│   │   ├── repository.go         # Data access layer
│   │   ├── types.go              # Domain models & structs
│   │   ├── permission.go         # Permission registry & checking
│   │   └── jwt.go                # JWT token management
│   │
│   ├── ticket/                   # Mímir - Tickets & Tasks
│   │   ├── handler.go            # HTTP handlers (/api/v1/tickets/*)
│   │   ├── service.go            # Business logic
│   │   ├── repository.go         # Data access (queries, mutations)
│   │   ├── types.go              # Domain models
│   │   ├── events.go             # Event publishing
│   │   └── workflow.go           # State machine (open → resolved → closed)
│   │
│   ├── tenant/                   # Valhalla - Tenant Administration
│   │   ├── handler.go            # HTTP handlers (/api/v1/admin/*)
│   │   ├── service.go            # Business logic
│   │   ├── repository.go         # Data access
│   │   ├── types.go              # Domain models
│   │   └── events.go             # Event publishing
│   │
│   ├── client/                   # Client Management
│   │   ├── handler.go            # HTTP handlers (/api/v1/clients/*)
│   │   ├── service.go            # Business logic
│   │   ├── repository.go         # Data access
│   │   ├── types.go              # Domain models
│   │   └── events.go             # Event publishing
│   │
│   ├── monitoring/               # Heimdallr - Metrics & Monitoring
│   │   ├── handler.go            # HTTP handlers (/api/v1/metrics/*)
│   │   ├── service.go            # Business logic
│   │   ├── repository.go         # Data access
│   │   ├── types.go              # Domain models
│   │   ├── events.go             # Event publishing
│   │   └── grpc.go               # gRPC handlers for agents
│   │
│   ├── audit/                    # Urd - Audit Logging
│   │   ├── handler.go            # HTTP handlers (/api/v1/audit/*)
│   │   ├── service.go            # Business logic (event subscriber)
│   │   ├── repository.go         # Data access
│   │   ├── types.go              # Domain models
│   │   └── subscriber.go         # NATS event subscriber
│   │
│   ├── portal/                   # Bifröst - Client Portal
│   │   ├── handler.go            # HTTP handlers (/api/v1/portal/*)
│   │   ├── service.go            # Business logic (read-only views)
│   │   ├── repository.go         # Data access
│   │   └── types.go              # Domain models
│   │
│   └── shared/                   # Cross-cutting concerns
│       ├── middleware/           # HTTP middleware
│       │   ├── auth.go           # JWT verification middleware
│       │   ├── tenant.go         # Tenant context extraction
│       │   ├── cors.go           # CORS headers
│       │   ├── logging.go        # Request logging
│       │   └── errors.go         # Error response formatting
│       │
│       ├── database.go           # PostgreSQL connection & pooling
│       ├── config.go             # Configuration management (env, files)
│       └── types.go              # Shared types (errors, responses)
│
├── pkg/                          # Public packages (reusable, importable)
│   ├── db/
│   │   ├── query.go              # sqlc-generated queries
│   │   ├── models.go             # sqlc-generated models
│   │   └── conn.go               # Connection helpers
│   │
│   ├── events/
│   │   ├── broker.go             # NATS client wrapper
│   │   ├── publisher.go          # Event publishing utilities
│   │   ├── subscriber.go         # Event subscription utilities
│   │   └── types.go              # Event structures
│   │
│   ├── errors/
│   │   ├── errors.go             # Custom error types
│   │   └── http.go               # HTTP error responses
│   │
│   ├── http/
│   │   ├── response.go           # JSON response wrappers
│   │   ├── request.go            # JSON request parsing
│   │   └── server.go             # HTTP server setup
│   │
│   └── grpc/
│       ├── interceptors.go       # gRPC middleware (auth, logging)
│       └── server.go             # gRPC server setup
│
├── migrations/
│   ├── 001_initial_schema.up.sql
│   ├── 001_initial_schema.down.sql
│   ├── 002_auth_tables.up.sql
│   ├── 002_auth_tables.down.sql
│   └── ...
│
├── proto/                        # Protocol Buffer definitions
│   ├── agent/
│   │   └── metrics.proto         # gRPC metrics service
│   └── BUILD                     # (if using Bazel)
│
├── tests/
│   ├── unit/
│   │   ├── auth_test.go
│   │   ├── ticket_test.go
│   │   └── ...
│   │
│   ├── integration/
│   │   ├── ticket_flow_test.go
│   │   ├── auth_flow_test.go
│   │   └── ...
│   │
│   └── fixtures/
│       ├── seeds.sql             # Test data
│       └── mocks.go              # Mock implementations
│
├── docker-compose.yml            # Local dev environment
├── Makefile                      # Build targets & commands
├── go.mod                        # Go module definition
├── go.sum                        # Dependency checksums
├── golangci.yml                  # Linter configuration
├── sqlc.yaml                     # sqlc configuration
└── README.md
```

---

## Service Initialization Flow

```go
// cmd/api/main.go
func main() {
    // 1. Load configuration
    cfg := shared.LoadConfig()

    // 2. Initialize database connection
    db := shared.ConnectDB(cfg.DatabaseURL)
    defer db.Close()

    // 3. Initialize NATS event broker
    eventBroker := events.NewBroker(cfg.NatsURL)
    defer eventBroker.Close()

    // 4. Initialize services (dependency injection)
    authSvc := auth.NewService(db)
    ticketSvc := ticket.NewService(db, eventBroker, authSvc)
    tenantSvc := tenant.NewService(db, eventBroker)
    clientSvc := client.NewService(db, eventBroker, authSvc)
    monitoringSvc := monitoring.NewService(db, eventBroker)
    auditSvc := audit.NewService(db, eventBroker)
    portalSvc := portal.NewService(db, authSvc, ticketSvc)

    // 5. Start NATS subscribers
    auditSvc.StartSubscriber()  // Listens to all events
    monitoringSvc.StartAgentListener()  // Listens to agent.* events

    // 6. Setup HTTP router with handlers
    router := setupRouter(
        authSvc, ticketSvc, tenantSvc, clientSvc,
        monitoringSvc, auditSvc, portalSvc,
    )

    // 7. Setup gRPC server for agents
    grpcServer := setupGRPCServer(monitoringSvc)

    // 8. Start servers
    go func() {
        grpcServer.Serve(":50051")  // gRPC for agents
    }()

    http.ListenAndServe(":8080", router)  // REST API
}
```

---

## Domain Service Pattern

Each domain service follows this structure:

```go
// internal/{domain}/handler.go
type Handler struct {
    svc *Service
    auth auth.Service  // Dependency
}

func (h *Handler) CreateResource(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    // 2. Check permissions (call auth.HasPermission)
    // 3. Call service method
    // 4. Write response
}

// internal/{domain}/service.go
type Service struct {
    repo   Repository
    broker events.Broker
    auth   auth.Service
}

func (s *Service) CreateResource(ctx context.Context, r *Resource) error {
    // 1. Validate input
    // 2. Execute business logic
    // 3. Store in database
    // 4. Publish event
    // 5. Return result
}

// internal/{domain}/repository.go
type Repository interface {
    GetResource(ctx context.Context, id uuid.UUID) (*Resource, error)
    CreateResource(ctx context.Context, r *Resource) (*Resource, error)
    UpdateResource(ctx context.Context, id uuid.UUID, updates map[string]interface{}) (*Resource, error)
    DeleteResource(ctx context.Context, id uuid.UUID) error
}

// internal/{domain}/types.go
type Resource struct {
    ID        uuid.UUID
    TenantID  uuid.UUID
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

---

## Dependency Injection Pattern

Services receive their depenencies in constructors:

```go
// ✓ Good - Constructor injection
func NewTicketService(repo repository.Ticket, broker events.Broker, auth auth.Service) *Service {
    return &Service{
        repo:   repo,
        broker: broker,
        auth:   auth,
    }
}

// ✗ Bad - Global singletons
var GlobalTicketService *Service  // Avoid!
```

---

## Package Organization Rules

### Internal

- Only importable within this service
- Domain-specific logic, repositories, types
- No cross-domain imports (except `shared`)
- Example: `ticket/service.go` cannot import `monitoring/types.go`

### Pkg

- Public and reusable
- No domain-specific logic
- Can be used by external services in v1.0
- Example: `pkg/events/broker.go` is used by all domains

### Shared

- Internal cross-cutting concerns
- Middleware, configuration, database connection
- Limited to this monolith

---

## Adding a New Domain Service

1. Create `internal/{domain}/` directory
2. Add these files:
   - `handler.go` - HTTP handlers
   - `service.go` - Business logic
   - `repository.go` - Data access (or interface)
   - `types.go` - Domain models
   - `events.go` - Event publishing (if needed)
3. Register service in `cmd/api/main.go`
4. Add routes to router setup
5. Add permissions to Týr if needed
6. Add tests in `tests/unit/{domain}_test.go`

---

## Testing Strategy

```
tests/
├── unit/
│   ├── auth_test.go              # Test service.go
│   ├── ticket_test.go            # Test service.go
│   └── ...
│
├── integration/
│   ├── ticket_flow_test.go       # Test whole flow (handler → service → db)
│   ├── event_flow_test.go        # Test event publishing
│   └── ...
│
└── fixtures/
    ├── seeds.sql                 # Test data
    └── mocks.go                  # Mock implementations
```

**Unit Tests**: Test service methods in isolation with mocked dependencies
**Integration Tests**: Test handler → service → database with real DB/NATS

---

## Migration Organization

Organize migrations by domain:

```
migrations/
├── 001_tenants.up.sql
├── 001_tenants.down.sql
├── 002_users.up.sql              # Týr
├── 002_users.down.sql
├── 003_permissions.up.sql        # Týr
├── 003_permissions.down.sql
├── 004_tickets.up.sql            # Mímir
├── 004_tickets.down.sql
├── 005_agents.up.sql             # Heimdallr
├── 005_agents.down.sql
├── 006_audit_log.up.sql          # Urd
├── 006_audit_log.down.sql
```
