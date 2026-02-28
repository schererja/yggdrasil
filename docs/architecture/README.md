# Architecture Documentation

This folder contains system architecture and design documentation for Yggdrasil.

**Current State**: v0.5 - Modular Monolith (single binary, multiple domain services ready for extraction)

## Contents

| Document                                             | Description                                             |
| ---------------------------------------------------- | ------------------------------------------------------- |
| [01-system-overview.md](01-system-overview.md)       | High-level architecture, component overview, data flows |
| [02-database-schema.md](02-database-schema.md)       | Database schema, tables, indexes, Row-Level Security    |
| [03-auth-acl.md](03-auth-acl.md)                     | Týr authentication and ACL plugin system                |
| [04-service-boundaries.md](04-service-boundaries.md) | Service contracts, dependencies, communication patterns |
| [05-event-catalog.md](05-event-catalog.md)           | NATS event definitions, publishers, subscribers         |
| [06-code-structure.md](06-code-structure.md)         | Project layout, package organization, design patterns   |
| [07-deployment-evolution.md](07-deployment-evolution.md) | Deployment configurations, v0.5 to v1.0+ migration path |

## Quick Start

1. **New to the project?** Start with [01-system-overview.md](01-system-overview.md)
2. **Building a domain service?** Read [04-service-boundaries.md](04-service-boundaries.md) and [06-code-structure.md](06-code-structure.md)
3. **Publishing/subscribing to events?** Check [05-event-catalog.md](05-event-catalog.md)
4. **Database queries?** See [02-database-schema.md](02-database-schema.md)
5. **User authentication/permissions?** Review [03-auth-acl.md](03-auth-acl.md)
6. **Deploying locally or to production?** See [07-deployment-evolution.md](07-deployment-evolution.md)

## Related Documentation

- [API Documentation](../api/README.md)
- [Security Guidelines](../security/SECURITY.md)
- [Deployment Guide](../deployment/DEPLOYMENT.md)

## Architecture Principles

1. **Modular Monolith** (v0.5) → **Microservices** (v1.0+) - Clear domain boundaries enable seamless extraction
2. **Vertical Slice Architecture** - Group code by domain, not by layer
3. **Multi-tenant via RLS** - Row-Level Security for tenant isolation
4. **Async-First** - NATS events for loose coupling between services
5. **Plugin-based Extensibility** - ACL system, event subscribers, agent plugins
6. **Data Portability** - Exportable/importable data, no proprietary formats
7. **Clear Boundaries** - Norse-themed service names for intuitive domain separation

## Vision

Yggdrasil is designed to scale from a modular monolith (v0.5) to independent microservices (v1.0+) without requiring core logic rewrites. The NATS event bus and clear service boundaries ensure that extracting a service is straightforward: spin up a new binary, point it at its database schema, and let events flow.
