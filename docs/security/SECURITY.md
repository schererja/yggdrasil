# Security Guidelines

## Overview

Security is a top priority for Yggdrasil. This document outlines security practices and requirements.

## Authentication

### Password Requirements

- Minimum 12 characters
- Require mixed case, numbers, and special characters
- Use bcrypt (cost 12+) or argon2 for hashing
- Never store plain text passwords

### JWT Tokens

- Short expiry (24 hours default)
- Secure secret (minimum 32 characters, randomly generated)
- Store in httpOnly cookies for web clients
- Implement token refresh mechanism

### Agent mTLS

All agent connections must use mutual TLS:

```go
// Agent TLS configuration
tlsConfig := &tls.Config{
    ClientCAs:  caCertPool,
    ClientAuth: tls.RequireAndVerifyClientCert,
}
```

## Authorization

### ACL System

Permission keys follow `resource:action` format:

| Permission | Description |
|------------|-------------|
| `ticket:create` | Create tickets |
| `ticket:read` | View tickets |
| `ticket:update` | Update tickets |
| `ticket:delete` | Delete tickets |
| `ticket:assign` | Assign tickets |
| `client:create` | Create clients |
| `client:read` | View clients |
| `agent:configure` | Configure agents |
| `admin:*` | Full admin access |

### Role-Based Access

| Role | Permissions |
|------|-------------|
| `admin` | Full access to all resources |
| `staff` | Ticket management, client access |
| `client` | Read own tickets, self-service |

## Database Security

### Row-Level Security (RLS)

All tenant-scoped tables must have RLS enabled:

```sql
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE tickets ENABLE ROW LEVEL SECURITY;
-- etc.
```

### Connection Security

- Use SSL/TLS for all database connections
- Use connection pooling with PgBouncer in production
- Rotate database credentials regularly

## API Security

### HTTPS Only

- Force HTTPS in production
- Use HSTS headers
- Implement proper TLS 1.2+

### Input Validation

- Validate all inputs on the server side
- Use parameterized queries (sqlc handles this)
- Sanitize user-generated content
- Implement rate limiting

### CORS

Configure CORS carefully:

```go
cors := cors.New(cors.Options{
    AllowedOrigins: []string{"https://app.yggdrasil.com"},
    AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE"},
    AllowedHeaders: []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
})
```

## Agent Security

### Certificate Management

- Use short-lived certificates for agents
- Implement certificate rotation
- Store certificates securely

### Agent Communication

- All agent traffic over TLS/mTLS
- Verify agent identity on connection
- Implement connection limits per agent

## Audit Logging

Log all security-relevant events:

```sql
CREATE TABLE audit_log (
    -- ... columns
    action VARCHAR(100) NOT NULL, -- login, logout, create, update, delete
    entity_type VARCHAR(100) NOT NULL,
    entity_id UUID,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

Required audit events:
- Authentication (login, logout, failed attempts)
- Authorization failures
- Data access and modifications
- Configuration changes
- Agent registration/removal

## Data Protection

### Encryption at Rest

- Use PostgreSQL encryption or filesystem-level encryption
- Protect backup files

### Sensitive Data

- Never log passwords or secrets
- Mask sensitive data in logs
- Use environment variables for secrets

## Incident Response

1. **Detect** - Monitor for anomalies
2. **Contain** - Isolate affected systems
3. **Eradicate** - Remove threat
4. **Recover** - Restore normal operations
5. **Lessons Learned** - Document and improve

## Security Checklist

Before production:

- [ ] Enable and test RLS on all tables
- [ ] Configure HTTPS/TLS properly
- [ ] Set up agent mTLS
- [ ] Implement audit logging
- [ ] Configure rate limiting
- [ ] Set up monitoring/alerting
- [ ] Review user permissions
- [ ] Test authentication flows
- [ ] Secure backup procedures
- [ ] Document incident response plan

## Reporting Security Issues

If you discover a security vulnerability, please report it responsibly:

1. Do not disclose publicly
2. Contact maintainers directly
3. Provide detailed reproduction steps
4. Allow time for remediation

## Dependencies

Keep dependencies up to date:

```bash
# Go
go mod tidy
go list -m -u all

# Node
cd web && npm outdated
```

Run security scans:

```bash
# Go
go run github.com/securego/gosec@latest ./...

# Node
cd web && npm audit
```
