# Event Catalog (NATS)

## Overview

Yggdrasil uses NATS for internal event publishing. Services publish domain events after state changes, allowing other services to react asynchronously without tight coupling.

## Event Subject Hierarchy

```
{domain}.{entity}.{action}

Examples:
- ticket.ticket.created       (Ticket domain, Ticket entity, created action)
- ticket.ticket.assigned      (Ticket domain, Ticket entity, assigned action)
- audit.auditlog.entry        (Audit domain, AuditLog entity, entry action)
- agent.agent.registered      (Agent domain, Agent entity, registered action)
```

## Event Format

All events follow this structure:

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2025-02-27T10:30:00Z",
  "domain": "ticket",
  "entity": "ticket",
  "action": "created",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440001",
  "user_id": "550e8400-e29b-41d4-a716-446655440002",
  "data": {
    "ticket_id": "550e8400-e29b-41d4-a716-446655440003",
    "title": "Network outage",
    "client_id": "550e8400-e29b-41d4-a716-446655440004",
    "status": "open"
  }
}
```

## Ticket Domain Events

### ticket.ticket.created

**Published by**: Mímir (TicketService)
**Subscribers**:

- Urd (audit logging)
- Heimdallr (stats/dashboards)

**Payload**:

```json
{
  "ticket_id": "uuid",
  "client_id": "uuid",
  "title": "string",
  "description": "string",
  "priority": "low|medium|high|urgent",
  "status": "open",
  "created_by": "uuid"
}
```

---

### ticket.ticket.assigned

**Published by**: Mímir (TicketService)
**Subscribers**:

- Urd (audit logging)
- Future: Verdandi (notifications)

**Payload**:

```json
{
  "ticket_id": "uuid",
  "assigned_to": "uuid",
  "assigned_by": "uuid",
  "assigned_at": "timestamp"
}
```

**Trigger**: `TicketService.AssignTicket()` called

---

### ticket.ticket.updated

**Published by**: Mímir (TicketService)
**Subscribers**:

- Urd (audit logging)
- Heimdallr (stats)

**Payload**:

```json
{
  "ticket_id": "uuid",
  "changes": {
    "title": "before -> after",
    "description": "before -> after",
    "priority": "before -> after",
    "status": "before -> after"
  }
}
```

**Trigger**: `TicketService.UpdateTicket()` called with modifications

---

### ticket.ticket.resolved

**Published by**: Mímir (TicketService)
**Subscribers**:

- Urd (audit logging)
- Future: Verdandi (notifications to client)

**Payload**:

```json
{
  "ticket_id": "uuid",
  "resolved_by": "uuid",
  "resolution_notes": "string (optional)",
  "resolved_at": "timestamp"
}
```

**Trigger**: Ticket status changed to "resolved"

---

### ticket.ticket.closed

**Published by**: Mímir (TicketService)
**Subscribers**:

- Urd (audit logging)
- Heimdallr (stats/SLA tracking)

**Payload**:

```json
{
  "ticket_id": "uuid",
  "closed_by": "uuid",
  "closed_at": "timestamp",
  "time_to_resolution_seconds": "integer"
}
```

**Trigger**: Ticket status changed to "closed"

---

## Client Domain Events

### client.client.created

**Published by**: App (ClientService)
**Subscribers**:

- Urd (audit logging)

**Payload**:

```json
{
  "client_id": "uuid",
  "name": "string",
  "email": "string",
  "created_by": "uuid"
}
```

---

### client.client.updated

**Published by**: App (ClientService)
**Subscribers**:

- Urd (audit logging)

**Payload**:

```json
{
  "client_id": "uuid",
  "changes": {
    "name": "before -> after",
    "email": "before -> after"
  }
}
```

---

## User Domain Events

### user.user.invited

**Published by**: Týr (AuthService)
**Subscribers**:

- Urd (audit logging)
- Future: Verdandi (send invite email)

**Payload**:

```json
{
  "user_id": "uuid",
  "email": "string",
  "role": "admin|staff|client",
  "invited_by": "uuid",
  "invited_at": "timestamp"
}
```

---

### user.user.activated

**Published by**: Týr (AuthService)
**Subscribers**:

- Urd (audit logging)

**Payload**:

```json
{
  "user_id": "uuid",
  "email": "string",
  "activated_at": "timestamp"
}
```

---

### user.user.role_changed

**Published by**: Týr (AuthService)
**Subscribers**:

- Urd (audit logging)

**Payload**:

```json
{
  "user_id": "uuid",
  "old_role": "string",
  "new_role": "string",
  "changed_by": "uuid",
  "changed_at": "timestamp"
}
```

---

## Agent Domain Events (Heimdallr)

### agent.agent.registered

**Published by**: Heimdallr (AgentService)
**Subscribers**:

- Urd (audit logging)

**Payload**:

```json
{
  "agent_id": "uuid",
  "hostname": "string",
  "ip_address": "string",
  "client_id": "uuid",
  "registered_at": "timestamp"
}
```

**Trigger**: New agent connects and registers via gRPC

---

### agent.agent.online

**Published by**: Heimdallr (AgentService)
**Subscribers**:

- Urd (audit logging)

**Payload**:

```json
{
  "agent_id": "uuid",
  "went_online_at": "timestamp"
}
```

**Trigger**: Agent sends heartbeat after being offline

---

### agent.agent.offline

**Published by**: Heimdallr (AgentService)
**Subscribers**:

- Urd (audit logging)
- Future: Verdandi (alert on critical agents)

**Payload**:

```json
{
  "agent_id": "uuid",
  "went_offline_at": "timestamp",
  "last_heartbeat": "timestamp"
}
```

**Trigger**: Agent missed heartbeat threshold

---

### agent.metrics.received

**Published by**: Heimdallr (MetricsCollector gRPC)
**Subscribers**:

- Heimdallr (stores in TimescaleDB)
- Urd (optional logging)

**Payload**:

```json
{
  "agent_id": "uuid",
  "cpu_percent": "float",
  "memory_percent": "float",
  "disk_percent": "float",
  "network_bytes_in": "int64",
  "network_bytes_out": "int64",
  "temperature_celsius": "float (optional)",
  "received_at": "timestamp"
}
```

**Note**: This event is high-volume (every metric interval); consider batching.

---

## Tenant Domain Events (Valhalla)

### tenant.tenant.created

**Published by**: Valhalla (TenantService)
**Subscribers**:

- Urd (audit logging)

**Payload**:

```json
{
  "tenant_id": "uuid",
  "name": "string",
  "slug": "string",
  "created_by": "uuid",
  "created_at": "timestamp"
}
```

---

### tenant.tenant.updated

**Published by**: Valhalla (TenantService)
**Subscribers**:

- Urd (audit logging)

**Payload**:

```json
{
  "tenant_id": "uuid",
  "changes": {
    "name": "before -> after",
    "settings": "before -> after"
  }
}
```

---

## Audit Domain Events (Urd)

### audit.auditlog.entry

**Published by**: Urd (AuditService)
**Subscribers**: None (terminal event)

**Payload**:

```json
{
  "audit_id": "uuid",
  "user_id": "uuid",
  "action": "create|update|delete|login|logout",
  "entity_type": "ticket|client|user|agent",
  "entity_id": "uuid",
  "changes": "object (optional)",
  "ip_address": "string",
  "user_agent": "string",
  "created_at": "timestamp"
}
```

**Trigger**: Any domain event is received; Urd logs it to database

---

## Subscription Patterns

### All Events (Urd - Audit)

```go
// Urd subscribes to all events for comprehensive audit trail
nats.Subscribe("*", auditHandler)
```

### Domain-Specific (Services)

```go
// Mímir subscribes to ticket events
nats.Subscribe("ticket.ticket.*", ticketEventHandler)

// Heimdallr subscribes to agent events
nats.Subscribe("agent.*", agentEventHandler)

// Verdandi (future) subscribes to notifications
nats.Subscribe("*.*.assigned", notificationHandler)
nats.Subscribe("*.*.resolved", notificationHandler)
```

---

## Future Events (Post-MVP)

- `comment.comment.created` - Comments on tickets
- `timeentry.timeentry.created` - Time tracking
- `attachment.attachment.uploaded` - File uploads
- `notification.notification.sent` - Delivery confirmations
- `sla.sla.breached` - SLA violations
- `schedule.oncall.changed` - On-call rotations
