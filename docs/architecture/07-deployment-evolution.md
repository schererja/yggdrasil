# Deployment & Evolution Strategy

## v0.5 - Modular Monolith (Current)

### Docker Compose (Local Development)

```yaml
version: "3.8"
services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: yggdrasil
      POSTGRES_PASSWORD: devpass
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  timescaledb:
    image: timescale/timescaledb:latest-pg15
    environment:
      POSTGRES_DB: yggdrasil_metrics
      POSTGRES_PASSWORD: devpass
    volumes:
      - timescale_data:/var/lib/postgresql/data
    ports:
      - "5433:5432"

  nats:
    image: nats:latest
    ports:
      - "4222:4222" # Client connections
      - "8222:8222" # Management UI

  api:
    build: .
    environment:
      DATABASE_URL: postgres://postgres:devpass@postgres:5432/yggdrasil
      METRICS_DATABASE_URL: postgres://postgres:devpass@timescaledb:5432/yggdrasil_metrics
      NATS_URL: nats://nats:4222
      LOG_LEVEL: debug
    ports:
      - "8080:8080" # REST API
      - "50051:50051" # gRPC (agents)
    depends_on:
      - postgres
      - timescaledb
      - nats

volumes:
  postgres_data:
  timescale_data:
```

**One binary handles all services**: Týr, Mímir, Valhalla, Bifröst, Heimdallr, Urd

### Docker Build

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o yggdrasil cmd/api/main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/yggdrasil /usr/local/bin/
EXPOSE 8080 50051
CMD ["yggdrasil"]
```

### Kubernetes Deployment (Production v0.5)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: yggdrasil-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: yggdrasil
  template:
    metadata:
      labels:
        app: yggdrasil
    spec:
      containers:
        - name: api
          image: yggdrasil:v0.5
          ports:
            - containerPort: 8080 # REST
            - containerPort: 50051 # gRPC
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: url
            - name: NATS_URL
              value: nats://nats-cluster:4222
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
```

**Single service scales horizontally**: All 3 replicas run all domains

---

## v1.0+ - Microservices (Target)

### Service Extraction Order

```
1. Extract Urd (Audit)       # No dependencies, only subscribes to events
   ↓
2. Extract Heimdallr         # Depends on Týr (auth), has its own gRPC
   ↓
3. Extract Mímir             # Depends on Týr (auth), publishes many events
   ↓
4. Extract Valhalla          # Depends on Týr (auth)
   ↓
5. Keep Týr + Bifröst        # Týr is foundational, Bifröst reads from Mímir
```

### Microservices Docker Compose (v1.0+)

```yaml
version: "3.8"
services:
  postgres-shared:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: yggdrasil
    ports:
      - "5432:5432"

  nats:
    image: nats:latest
    ports:
      - "4222:4222"
      - "8222:8222"

  # Service 1: Týr (foundational)
  tyr-service:
    build:
      context: .
      dockerfile: services/auth/Dockerfile
    environment:
      SERVICE_NAME: tyr
      DATABASE_URL: postgres://postgres@postgres-shared:5432/yggdrasil
      NATS_URL: nats://nats:4222
    ports:
      - "3001:8080"
    depends_on:
      - postgres-shared
      - nats

  # Service 2: Urd (audit)
  urd-service:
    build:
      context: .
      dockerfile: services/audit/Dockerfile
    environment:
      SERVICE_NAME: urd
      DATABASE_URL: postgres://postgres@postgres-shared:5432/yggdrasil
      NATS_URL: nats://nats:4222
    depends_on:
      - postgres-shared
      - nats
      - tyr-service

  # Service 3: Mímir (tickets)
  mimic-service:
    build:
      context: .
      dockerfile: services/ticket/Dockerfile
    environment:
      SERVICE_NAME: mimic
      DATABASE_URL: postgres://postgres@postgres-shared:5432/yggdrasil
      NATS_URL: nats://nats:4222
      TYR_URL: http://tyr-service:8080
    ports:
      - "3002:8080"
    depends_on:
      - postgres-shared
      - nats
      - tyr-service

  # Service 4: Heimdallr (monitoring)
  heimdallr-service:
    build:
      context: .
      dockerfile: services/monitoring/Dockerfile
    environment:
      SERVICE_NAME: heimdallr
      DATABASE_URL: postgres://postgres@postgres-shared:5432/yggdrasil
      NATS_URL: nats://nats:4222
      TYR_URL: http://tyr-service:8080
    ports:
      - "3005:8080"
      - "50051:50051"
    depends_on:
      - postgres-shared
      - nats
      - tyr-service

  # API Gateway (routes to services)
  api-gateway:
    image: nginx:alpine
    ports:
      - "8080:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - tyr-service
      - mimic-service
      - heimdallr-service

  postgres-adminer:
    image: adminer
    ports:
      - "8081:8080"
```

**Multiple services, each scaled independently**: Mímir can scale to 5 replicas while Urd runs 1

### API Gateway (nginx config for v1.0+)

```nginx
upstream tyr {
    server tyr-service:8080;
}
upstream mimic {
    server mimic-service:8080;
}
upstream heimdallr {
    server heimdallr-service:8080;
}

server {
    listen 80;

    # Route to Týr (auth)
    location /api/v1/auth/ {
        proxy_pass http://tyr/api/v1/auth/;
    }

    # Route to Mímir (tickets)
    location /api/v1/tickets/ {
        proxy_pass http://mimic/api/v1/tickets/;
    }

    # Route to Heimdallr (metrics)
    location /api/v1/metrics/ {
        proxy_pass http://heimdallr/api/v1/metrics/;
    }

    # Health check
    location /healthz {
        access_log off;
        return 200 "healthy\n";
    }
}
```

---

## Database Strategy

### v0.5 (Shared Database)

```
PostgreSQL (yggdrasil)
├── public schema (Týr)
│   ├── tenants
│   ├── users
│   ├── permissions
│   └── role_permissions
├── ticket schema (Mímir)
│   ├── tickets
│   └── (future: comments, time_entries)
├── audit schema (Urd)
│   └── audit_log
├── agent schema (Heimdallr)
│   └── agents
└── (other schemas per service)

Run migrations with: migrate -path migrations -database "postgres://..." up
```

**Pros**: Simpler transactions, no distributed consistency issues, shared connection pool
**Cons**: More tightly coupled, harder to scale database per service later

### v1.0+ (Database Per Service, Option 1)

```
postgres-tyr
├── auth_db
│   ├── tenants
│   ├── users
│   ├── permissions
│   └── role_permissions

postgres-ticket
├── ticket_db
│   ├── tickets
│   └── (comments, time_entries)

postgres-monitoring
├── monitoring_db
│   ├── agents
│   └── agent_metrics (TimescaleDB hypertable)

postgres-audit
├── audit_db
│   └── audit_log
```

**Migration**: Each service still connects to shared postgres, but eventually to separate instances

### v1.0+ (Database Per Service, Option 2 - Schema Per Service in Shared DB)

```
postgres-shared (same instance, different schemas)
├── auth schema (Týr service owns)
├── ticket schema (Mímir service owns)
├── monitoring schema (Heimdallr service owns)
└── audit schema (Urd service owns)

Each service: DATABASE_URL=postgres://...?schema=ticket
```

**Pros**: No new database infrastructure needed, easier migration
**Cons**: Shared backup/restore, harder to scale

---

## Migration Path: v0.5 → v1.0

### Step 1: Codify Service Boundaries (DONE)

- Clear interfaces between services
- NATS events for async communication
- No cross-domain imports (except shared())

### Step 2: Extract First Service (Urd - Audit)

1. Move `internal/audit/` → `services/audit/`
2. Create `services/audit/main.go` entry point
3. Uses existing PostgreSQL, subscribes to NATS
4. Deploy alongside monolith
5. Both versions serve requests (canary deployment)

### Step 3: Extract More Services (Heimdallr, Mímir, Valhalla)

Repeat step 2 for each service

### Step 4: Refactor Remaining Monolith

- Týr + Bifröst remain in `cmd/api/main.go`
- Or split further if needed

### Step 5: Optional - Database Per Service

- Migrate each service's tables to separate database
- Update connection strings
- Shared NATS remains central

---

## Key Deployment Principles

1. **Shared NATS** - Central event bus, never changes
2. **Shared PostgreSQL Initially** - Schemas per service for v0.5
3. **Stateless Services** - Can scale horizontally
4. **Health Checks** - Each service exposes `/healthz`
5. **Gradual Extraction** - Canary deployments during transition
6. **Backward Compatibility** - Services work with monolith and microservices

---

## Monitoring & Observability

### Logging

All services log to stdout (picked up by Docker/K8s):

```go
logger.Info("service_started",
    "service", "mimic",
    "version", "v0.5",
    "tenant_id", tenantID,
)
```

### Metrics

Expose Prometheus metrics on `/metrics`:

```
yggdrasil_http_requests_total{handler="create_ticket",status="200"}
yggdrasil_ticket_created_total{tenant_id="..."}
yggdrasil_nats_events_published_total{subject="ticket.ticket.created"}
yggdrasil_database_connection_pool_size{service="mimic"}
```

### Tracing (Future)

Use OpenTelemetry for distributed tracing across service boundaries:

```go
tracer.WithSpan(ctx, "create_ticket", func(ctx context.Context, span trace.Span) error {
    // Business logic
})
```
