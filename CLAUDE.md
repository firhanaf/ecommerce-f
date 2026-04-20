# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the application
make run

# Build binary (outputs to bin/api)
make build

# Start local PostgreSQL container
make db

# Apply database schema
make migrate

# Stop and remove database container
make db-stop

# Tidy Go modules
make tidy

# Standard Go commands (no Makefile targets)
go test ./...
go vet ./...
go fmt ./...
```

## Architecture

This is a **Go 1.22** REST API using **Chi v5** for routing and **PostgreSQL 16** (via `pgx/v5`) as the datastore. It follows clean architecture with strict layering:

```
cmd/api/main.go          → entry point, wires all dependencies and starts server
config/                  → Viper-based env config (.env file)
internal/
  domain.go              → all domain models (User, Product, Order, Payment, etc.)
  handler/               → HTTP handlers (parse request → call usecase → write response)
  middleware/            → Auth (JWT), RequireRole, RequestLogger, Recoverer
  usecase/               → business logic, one package per domain (auth, product, cart, order, payment)
  repository/
    interfaces.go        → repository contracts (interfaces)
    postgres/            → PostgreSQL implementations of repository interfaces
  router/                → Chi route definitions, middleware wiring
pkg/
  database/              → pgx connection pool setup
  jwt/                   → token generation and validation (HS256, access=60min, refresh=7days)
  payment/               → Midtrans Snap API integration
  storage/               → AWS S3 upload/delete operations
```

**Dependency flow:** `main` instantiates infra (DB, S3, JWT, payment) → creates repositories → creates usecases → creates handlers → mounts router.

**Interfaces are defined in `internal/repository/interfaces.go`.** All usecases depend on these interfaces, not concrete types — allowing implementations to be swapped without touching business logic.

## Key Design Decisions

- **No ORM** — raw parameterized SQL only (prevent injection, full query control)
- **UUID primary keys** everywhere (distributed-friendly)
- **JSONB** for address snapshots in orders (immutable at checkout time) and audit log payloads
- **Soft deletes** via `is_active` flag; `variant_id` in `OrderItem` is nullable to handle deleted variants
- **Roles:** `buyer`, `seller`, `admin` — enforced in middleware via `RequireRole()`
- **Context values** after Auth middleware: `user_id` (uuid.UUID) and `role` (string)

## Route Structure

| Prefix | Auth Required | Role Guard |
|---|---|---|
| `/api/v1/auth/*`, `/api/v1/products` (GET) | No | — |
| `/api/v1/cart/*`, `/api/v1/orders/*` | JWT | buyer+ |
| `/api/v1/seller/*` | JWT | seller or admin |
| `/api/v1/admin/*` | JWT | admin only |
| `/api/v1/payments/webhook` | No | — |

## Configuration

Copy `.env.example` to `.env`. Required variables:

- `DB_*` — PostgreSQL connection (host, port, user, password, name, sslmode)
- `JWT_SECRET` — minimum 32 characters
- `AWS_*` — region, access key, secret key, S3 bucket name
- `MIDTRANS_SERVER_KEY` / `MIDTRANS_CLIENT_KEY` — use `SB-` prefix for sandbox
- `APP_PORT` — defaults to `8080`
