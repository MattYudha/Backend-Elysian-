# Elysian Backend

A Go-based API server for workflow management and execution, built with Fiber framework.

## Tech Stack

- **Backend**: Go 1.25.5, Fiber v2
- **Database**: PostgreSQL with migrations (goose)
- **Cache**: Redis
- **Architecture**: REST API with role-based access control

## Quick Start

1. **Clone and setup**:
   ```bash
   git clone https://github.com/Elysian-Rebirth/backend-go.git
   cd backend-go
   cp .env.example .env
   ```

2. **Start services**:
   ```bash
   make docker-up
   make migrate-up # Run database migrations
   ```

3. **Run the application**:
   ```bash
   make run
   ```
4. **Open the Toolbox dev**:
```
http://localhost
```

The API will be available at `http://localhost:7777`

## Development

- **Build**: `make build`
- **Test**: `make test`
- **Format code**: `go fmt ./...`
- **Lint**: `go vet ./...`

## Project Structure

```
├── cmd/server/          # Application entrypoint
├── internal/            # Private application code
│   ├── handlers/        # HTTP handlers
│   ├── services/        # Business logic
│   ├── repositories/    # Data access layer
│   └── models/          # Data models
├── pkg/                 # Public packages
├── migrations/          # Database migrations
└── docs/                # Documentation
```

## API Documentation
