# Deployment Guide

## Overview

This guide covers deploying Yggdrasil to production.

## Prerequisites

- Docker and Docker Compose (or Kubernetes)
- PostgreSQL 15+ with TimescaleDB
- NATS 2.x
- TLS certificates

## Environment Variables

Required environment variables:

```bash
# Database
DATABASE_URL=postgres://user:password@host:5432/yggdrasil?sslmode=require

# NATS
NATS_URL=nats://host:4222

# JWT
JWT_SECRET=your-secret-key-min-32-chars
JWT_EXPIRY=24h

# Server
API_HOST=0.0.0.0:8080
API_BASE_URL=https://your-domain.com

# Agent mTLS
AGENT_CERT_PATH=/path/to/agent.crt
AGENT_KEY_PATH=/path/to/agent.key
CA_CERT_PATH=/path/to/ca.crt
```

## Docker Compose (Development/Small Scale)

```bash
docker compose up -d
```

See [docker-compose.yml](../../docker-compose.yml) for the full configuration.

## Production Deployment

### 1. Database Setup

```bash
# Run migrations
migrate -path migrations -database "$DATABASE_URL" up
```

### 2. Control Plane

```bash
# Build
docker build -t yggdrasil/api:latest -f cmd/api/Dockerfile .

# Run
docker run -d \
  --name yggdrasil-api \
  -p 8080:8080 \
  -e DATABASE_URL="$DATABASE_URL" \
  -e NATS_URL="$NATS_URL" \
  -e JWT_SECRET="$JWT_SECRET" \
  yggdrasil/api:latest
```

### 3. Reverse Proxy

Production requires a reverse proxy (nginx, Traefik, etc.) for TLS termination.

Example nginx configuration:

```nginx
server {
    listen 443 ssl http2;
    server_name api.yggdrasil.com;

    ssl_certificate /etc/ssl/certs/server.crt;
    ssl_certificate_key /etc/ssl/private/server.key;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 4. Agent Deployment

Agents can be deployed via:

- **Systemd** (Linux servers)
- **Windows Service** (Windows)
- **Docker** (containerized)

```bash
# Linux systemd
sudo cp agent.service /etc/systemd/system/
sudo systemctl enable --now yggdrasil-agent
```

### 5. Scaling

For production scale:

- Run multiple API replicas (stateless)
- Use PgBouncer for database connection pooling
- NATS handles message queue scaling
- Consider read replicas for TimescaleDB

## Monitoring

Health check endpoint: `GET /health`

```bash
curl https://api.yggdrasil.com/health
```

Response:

```json
{
  "status": "healthy",
  "version": "0.5.0",
  "checks": {
    "database": "ok",
    "nats": "ok"
  }
}
```

## Backup

PostgreSQL backup:

```bash
pg_dump "$DATABASE_URL" > backup_$(date +%Y%m%d_%H%M%S).sql
```

TimescaleDB backup (use timescaledb-backup tool for best results).

## SSL/TLS Certificates

Use Let's Encrypt or your preferred CA.

For agent mTLS:

```bash
# Generate CA
openssl genrsa -out ca.key 4096
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt

# Generate agent cert
openssl genrsa -out agent.key 2048
openssl req -new -key agent.key -out agent.csr
openssl x509 -req -in agent.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out agent.crt -days 365 -sha256
```

## Security Checklist

- [ ] Use TLS for all connections
- [ ] Enable and enforce RLS on all tables
- [ ] Rotate JWT secrets regularly
- [ ] Use strong password hashing (bcrypt/argon2)
- [ ] Enable agent mTLS
- [ ] Set up audit logging
- [ ] Regular security updates
- [ ] Backup strategy in place
