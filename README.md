# Aegis

A user management microservice built with Go. I created this primarily as a learning exercise and to have a solid template I can reference when building microservices in the future.

## What it does

- User registration, authentication (JWT with refresh token rotation)
- Role-based access control (RBAC)
- Both REST and gRPC APIs
- PostgreSQL for storage

## Project Structure

```
aegis/
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── domain/          # Core business entities (User, Role, Permission)
│   ├── service/         # Business logic
│   ├── storage/         # Database layer (PostgreSQL)
│   ├── transport/       # HTTP and gRPC handlers
│   ├── auth/            # JWT and password utilities
│   └── config/          # Configuration
├── migrations/          # SQL migrations
└── api/proto/           # gRPC definitions
```

**Why this structure?**

I followed a layered approach where each layer only knows about the layer below it. The domain layer has zero dependencies, services orchestrate business logic, and transport handles HTTP/gRPC specifics. This keeps things testable and makes it easy to swap out implementations (different database, different transport, etc).

Repository interfaces are in `storage/repository.go` with PostgreSQL implementations in `storage/postgres/`. This isn't over-engineering—it genuinely helps when writing tests and leaves the door open for other databases.

## Getting Started

**Prerequisites:** Go 1.22+, PostgreSQL, Docker (optional)

```bash
# Start PostgreSQL (or use your own)
docker compose up -d postgres

# Run migrations
make migrate-up

# Run the service
make run
```

The service starts on `:8080` (HTTP) and `:9090` (gRPC).

**Environment variables:**

| Variable | Default |
|----------|---------|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/userservice?sslmode=disable` |
| `JWT_SECRET_KEY` | (required) |
| `HTTP_PORT` | `8080` |
| `GRPC_PORT` | `9090` |

## Quick API Reference

```bash
# Register
curl -X POST localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Password123","username":"testuser","full_name":"Test User"}'

# Login
curl -X POST localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Password123"}'

# Get current user (use token from login response)
curl localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer <access_token>"
```

## Notes

- Passwords require 8+ chars with uppercase, lowercase, and a digit
- Access tokens expire in 15 minutes, refresh tokens in 7 days
- Refresh tokens rotate on each use (old token becomes invalid)
- gRPC handlers are stubbed out—run `buf generate` after installing buf to generate the protobuf code

