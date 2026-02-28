# Contributing to Yggdrasil

Thank you for your interest in contributing to Yggdrasil!

## Getting Started

1. **Fork the repository**
2. **Clone your fork**: `git clone https://github.com/YOUR_USERNAME/yggdrasil.git`
3. **Create a branch**: `git checkout -b feature/your-feature-name` or `fix/issue-description`

## Development Setup

See [README.md](README.md#prerequisites) for the full list of prerequisites.

### Local Development Environment

```bash
# 1. Copy environment variables
cp .env.example .env

# 2. Start PostgreSQL and NATS
docker compose up -d postgres nats

# 3. Run migrations
migrate -path migrations -database "postgres://localhost/yggdrasil?sslmode=disable" up

# 4. Start the control plane
go mod download
sqlc generate
go run cmd/api/main.go

# 5. Start the web app (in another terminal)
cd web
npm ci
npm run dev
```

## Code Standards

### Go (Control Plane & Agent)

- Use [golangci-lint](https://golangci-lint.run/) for linting
- Run `make lint` before committing
- Follow standard Go conventions (run `go fmt` and `go vet`)
- Target 80%+ test coverage for backend code
- Use `sqlc generate` after any SQL changes

### React/TypeScript (Web App)

- Use ESLint and Prettier (configured in the project)
- Run `npm run lint` before committing
- Target 70%+ test coverage for frontend code
- Use TypeScript for all new code

### General

- Keep documentation in sync with code changes
- Write meaningful commit messages
- Small, focused PRs are preferred over large, monolithic changes
- All CI checks must pass before merging

## Testing

```bash
# Backend tests
go test ./...

# Frontend tests
cd web && npm test
```

## Submitting Changes

1. Push your branch to your fork
2. Open a Pull Request against `main`
3. Fill out the PR template completely
4. Ensure all status checks pass
5. Request review from maintainers

## Commit Message Format

Use conventional commits:

```
type(scope): description

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Example:
```
feat(tickets): add status update endpoint

Add PATCH /api/v1/tickets/:id/status endpoint for updating
ticket status in the workflow.

Closes #123
```

## Pull Request Guidelines

- **One feature/fix per PR**: Keeps reviews focused and reduces risk
- **Include tests**: Bug fixes should include regression tests
- **Update docs**: If your change affects the API or architecture, update relevant docs
- **Be responsive**: Address review comments promptly

## Code Review Process

1. Automated checks run (lint, tests, security scans)
2. At least one maintainer review required
3. Address feedback and request re-review
4. Squash and merge

## Questions?

- Open an issue for bugs or feature requests
- Use discussions for questions
- Don't hesitate to ask for help!
