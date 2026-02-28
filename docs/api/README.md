# API Documentation

## Overview

Yggdrasil provides a RESTful API for the control plane. All endpoints require authentication unless otherwise noted.

## Base URL

```
Production: https://api.yggdrasil.com/api/v1
Development: http://localhost:8080/api/v1
```

## Authentication

### Login

```http
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securepassword"
}
```

Response:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2024-01-15T12:00:00Z",
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "role": "staff"
  }
}
```

### Using the Token

Include the token in the Authorization header:

```http
GET /api/v1/tickets
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

## Common Headers

| Header | Value |
|--------|-------|
| `Content-Type` | `application/json` |
| `Authorization` | `Bearer <token>` |
| `X-Tenant-ID` | `<tenant-uuid>` (for multi-tenant) |

## Endpoints

### Health

```http
GET /health
```

No authentication required.

### Auth

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /auth/login | Login |
| POST | /auth/logout | Logout |
| POST | /auth/refresh | Refresh token |
| GET | /auth/me | Current user info |

### Users (Týr)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /users | List users |
| GET | /users/:id | Get user |
| POST | /users | Create user |
| PATCH | /users/:id | Update user |
| DELETE | /users/:id | Delete user |

### Tenants (Valhalla)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /tenants | List tenants |
| GET | /tenants/:id | Get tenant |
| POST | /tenants | Create tenant |
| PATCH | /tenants/:id | Update tenant |

### Clients

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /clients | List clients |
| GET | /clients/:id | Get client |
| POST | /clients | Create client |
| PATCH | /clients/:id | Update client |
| DELETE | /clients/:id | Delete client |

### Tickets (Mímir)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /tickets | List tickets |
| GET | /tickets/:id | Get ticket |
| POST | /tickets | Create ticket |
| PATCH | /tickets/:id | Update ticket |
| DELETE | /tickets/:id | Delete ticket |
| PATCH | /tickets/:id/status | Update ticket status |
| PATCH | /tickets/:id/assign | Assign ticket |

### Agents (Smidr)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /agents | List agents |
| GET | /agents/:id | Get agent |
| GET | /agents/:id/metrics | Get agent metrics |
| POST | /agents | Register agent |
| DELETE | /agents/:id | Remove agent |

### Metrics (Heimdallr)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /metrics | Get aggregated metrics |
| GET | /metrics/agents/:id | Get specific agent metrics |

### Audit (Urd)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /audit | List audit entries |

## Response Formats

### Success

```json
{
  "data": { ... },
  "meta": {
    "total": 100,
    "page": 1,
    "per_page": 20
  }
}
```

### Error

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Email is required",
    "details": [...]
  }
}
```

### List Response

```json
{
  "data": [
    { "id": "uuid", "title": "Ticket 1" },
    { "id": "uuid", "title": "Ticket 2" }
  ],
  "meta": {
    "total": 2,
    "page": 1,
    "per_page": 20
  }
}
```

## Pagination

Query parameters:

| Param | Default | Max |
|-------|---------|-----|
| `page` | 1 | - |
| `per_page` | 20 | 100 |

```http
GET /api/v1/tickets?page=2&per_page=50
```

## Filtering

Common query parameters:

| Param | Description |
|-------|-------------|
| `status` | Filter by status |
| `tenant_id` | Filter by tenant |
| `client_id` | Filter by client |
| `assigned_to` | Filter by assignee |
| `search` | Text search |

```http
GET /api/v1/tickets?status=open&priority=high
```

## Rate Limiting

TBD - implement based on production needs.

## Webhooks

TBD - for ticket events, etc.

## SDKs

- [Agent SDK](../packages/agent-sdk) - Go SDK for agent communication
- [Control Plane SDK](../packages) - TBD
