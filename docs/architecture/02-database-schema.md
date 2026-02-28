# Database Schema

## Overview

Yggdrasil uses PostgreSQL with TimescaleDB for time-series data. All tables include `tenant_id` for Row-Level Security (RLS) isolation.

## Core Tables

### Tenants

```sql
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Users (Týr)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    role VARCHAR(50) DEFAULT 'staff', -- staff, admin, client
    is_active BOOLEAN DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, email)
);
```

### Clients

```sql
CREATE TABLE clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(50),
    address TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Tickets (Mímir)

```sql
CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID REFERENCES clients(id),
    assigned_to UUID REFERENCES users(id),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(50) DEFAULT 'open', -- open, in_progress, resolved, closed
    priority VARCHAR(50) DEFAULT 'medium', -- low, medium, high, urgent
    created_by UUID REFERENCES users(id),
    assigned_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Agents

```sql
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    client_id UUID REFERENCES clients(id),
    name VARCHAR(255) NOT NULL,
    hostname VARCHAR(255),
    ip_address INET,
    status VARCHAR(50) DEFAULT 'offline', -- online, offline, error
    last_seen_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Agent Metrics (TimescaleDB)

```sql
CREATE TABLE agent_metrics (
    time TIMESTAMPTZ NOT NULL,
    agent_id UUID NOT NULL,
    cpu_percent DOUBLE PRECISION,
    memory_percent DOUBLE PRECISION,
    disk_percent DOUBLE PRECISION,
    network_bytes_in BIGINT,
    network_bytes_out BIGINT,
    temperature_celsius DOUBLE PRECISION
);

-- Convert to hypertable
SELECT create_hypertable('agent_metrics', 'time');

-- Index for efficient queries by agent
CREATE INDEX idx_agent_metrics_agent_id ON agent_metrics (agent_id, time DESC);
```

### Audit Log (Urd)

```sql
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id UUID REFERENCES users(id),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id UUID,
    changes JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Permissions (Týr ACL)

```sql
CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(100) UNIQUE NOT NULL, -- e.g., "ticket:create", "ticket:read"
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE role_permissions (
    role VARCHAR(50) NOT NULL,
    permission_id UUID NOT NULL REFERENCES permissions(id),
    PRIMARY KEY (role, permission_id)
);
```

## Row-Level Security (RLS)

All tenant-scoped tables have RLS enabled:

```sql
-- Example RLS policy
CREATE POLICY tenant_isolation ON tickets
    USING (tenant_id = current_setting('app.tenant_id')::UUID);
```

Enable RLS per table:

```sql
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE clients ENABLE ROW LEVEL SECURITY;
-- etc.
```

## Migrations

All migrations are in the `migrations/` directory using golang-migrate.

Naming convention: `{version}_{description}.sql`

Example: `001_create_tenants.sql`, `002_create_users.sql`

## Indexes

Key indexes for performance:

```sql
-- Tenant lookups
CREATE INDEX idx_users_tenant_email ON users (tenant_id, email);
CREATE INDEX idx_tickets_tenant_status ON tickets (tenant_id, status);
CREATE INDEX idx_audit_log_tenant_created ON audit_log (tenant_id, created_at DESC);

-- Agent lookups
CREATE INDEX idx_agents_tenant_client ON agents (tenant_id, client_id);
```

## Future Tables (v1.0+)

- `time_entries` - Time tracking on tickets
- `comments` - Ticket comments/notes
- `attachments` - File uploads
- `schedules` - On-call schedules
- `slas` - SLA definitions
- `notifications` - User notifications
