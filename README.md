<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25.5-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/Gin-Framework-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Gin"/>
  <img src="https://img.shields.io/badge/PostgreSQL-16-316192?style=for-the-badge&logo=postgresql&logoColor=white" alt="PostgreSQL"/>
  <img src="https://img.shields.io/badge/Redis-8-DC382D?style=for-the-badge&logo=redis&logoColor=white" alt="Redis"/>
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Docker"/>
</p>

# ⚡ Elysian Backend

A high-performance Go-based REST API server for **AI-powered workflow automation** — built with Clean Architecture principles, featuring document intelligence, RAG-based search, and multi-tenant execution engine.

---

## 🏗️ Tech Stack

| Layer | Technology |
|---|---|
| **Language** | Go 1.25.5 |
| **Framework** | Gin (HTTP) + GORM (ORM) |
| **Database** | PostgreSQL 16 with pgvector |
| **Cache** | Redis 8 |
| **Message Queue** | Asynq (Redis-backed) |
| **Object Storage** | MinIO (S3-compatible) |
| **AI Integration** | Google Gemini AI + Agent SDK |
| **Auth** | JWT (Access + Refresh tokens) |
| **Monitoring** | Prometheus + OpenTelemetry |
| **Docs** | Swagger/OpenAPI |
| **Deployment** | Docker + Docker Compose |

---

## 📁 Project Structure

```
Backend-Elysian-/
├── cmd/
│   ├── server/              # Application entrypoint (main.go)
│   └── seed_admin/          # Admin seeder CLI tool
├── config/
│   ├── config.yml           # Development configuration
│   ├── config.dev.yml       # Dev overrides
│   └── config.prod.yml      # Production overrides
├── docs/
│   ├── architecture/        # Architecture documentation
│   ├── docs.go              # Swagger generated docs
│   ├── swagger.json
│   └── swagger.yaml
├── internal/
│   ├── config/              # Config loader & validator
│   ├── delivery/
│   │   └── http/
│   │       ├── dto/         # Data Transfer Objects
│   │       ├── handler/     # HTTP handlers (auth, workflow, document, RAG, etc.)
│   │       └── routes/      # Route definitions
│   ├── domain/              # Domain entities (user, workflow, document, execution, etc.)
│   │   └── repository/      # Repository interfaces
│   ├── infrastructure/
│   │   ├── agent/           # AI Agent SDK integration
│   │   ├── ai/              # Google Gemini AI service
│   │   ├── cache/           # Redis cache layer
│   │   ├── database/        # Database connection & pooling
│   │   ├── mq/              # Message queue (Asynq)
│   │   ├── parsing/         # Document parsing engine
│   │   ├── storage/         # MinIO object storage
│   │   └── telemetry/       # Prometheus + OpenTelemetry
│   ├── middleware/           # Auth, RBAC, Logger, Recovery
│   ├── repository/
│   │   └── postgres/        # PostgreSQL implementations
│   └── usecase/             # Business logic (auth, document, engine, RAG, workflow)
├── migrations/              # SQL migrations (goose)
├── toolbox/                 # DevOps monitoring dashboard
├── .env.example             # Environment template
├── .env.production.example  # Production env template
├── Dockerfile               # Multi-stage Docker build
├── docker-compose.yml       # Development services
├── docker-compose.prod.yml  # Production deployment
├── Makefile                 # Build & task automation
├── railway.toml             # Railway deployment config
├── go.mod
└── go.sum
```

---

## 🚀 Quick Start

### Prerequisites

- Go 1.25.5+
- Docker & Docker Compose
- Make (optional, recommended)

### 1. Clone & Setup

```bash
git clone https://github.com/MattYudha/Backend-Elysian-.git
cd Backend-Elysian-
cp .env.example .env
```

### 2. Start Infrastructure

```bash
# Start PostgreSQL, Redis, MinIO, and Toolbox
make docker-up
```

### 3. Run Database Migrations

```bash
make migrate-up
```

### 4. Seed Admin User

```bash
go run cmd/seed_admin/main.go
```

### 5. Run the Server

```bash
make run
```

The API will be available at **`http://localhost:7777`**

---

## 📋 Available Commands

```bash
make help             # Show all available commands
make run              # Run the application
make build            # Build binary to bin/server
make test             # Run all tests
make clean            # Clean build artifacts
make swagger          # Generate Swagger docs
make docker-up        # Start Docker services
make docker-down      # Stop Docker services
make docker-logs      # View Docker logs
make migrate-up       # Run database migrations
make migrate-down     # Rollback last migration
make migrate-status   # Check migration status
make migrate-reset    # Reset all migrations
make migrate-create NAME=xxx  # Create new migration
```

---

## 🔌 API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/api/v1/auth/register` | Register user |
| `POST` | `/api/v1/auth/login` | Login (returns JWT) |
| `POST` | `/api/v1/auth/refresh` | Refresh access token |
| `GET` | `/api/v1/workflows` | List workflows |
| `POST` | `/api/v1/workflows` | Create workflow |
| `POST` | `/api/v1/executions` | Execute workflow |
| `POST` | `/api/v1/documents` | Upload document |
| `GET` | `/api/v1/rag/search` | RAG-powered search |
| `GET` | `/api/v1/users` | User management |
| `GET` | `/swagger/*` | Swagger UI |

> Full API documentation available at `http://localhost:7777/swagger/index.html`

---

## 🏛️ Architecture

This project follows **Clean Architecture** with clear separation of concerns:

```
┌─────────────────────────────────────────────────────┐
│                   Delivery (HTTP)                    │
│          Handlers → DTOs → Routes                    │
├─────────────────────────────────────────────────────┤
│                   Use Cases                          │
│     Auth · Workflow · Document · Engine · RAG        │
├─────────────────────────────────────────────────────┤
│                    Domain                            │
│   Entities · Repository Interfaces · Value Objects   │
├─────────────────────────────────────────────────────┤
│                Infrastructure                        │
│  Database · Cache · MQ · AI · Storage · Telemetry    │
└─────────────────────────────────────────────────────┘
```

**Key Patterns:**
- **Repository Pattern** — data access abstracted through interfaces
- **Dependency Injection** — all dependencies injected at startup
- **Middleware Chain** — Auth → RBAC → Logger → Recovery
- **Multi-tenant** — tenant isolation via UUID-based scoping

---

## 🐳 Docker Deployment

### Development

```bash
docker-compose up -d
```

### Production

```bash
docker-compose -f docker-compose.prod.yml up -d --build
```

Services included:
- **PostgreSQL 16** (pgvector) — vector search enabled
- **Redis 8** — caching & message queue
- **MinIO** — S3-compatible object storage
- **App** — Go backend server (production only)

---

## 🔐 Security Features

- **JWT Authentication** — access + refresh token rotation
- **RBAC Middleware** — role-based access control per endpoint
- **CORS Configuration** — configurable allowed origins
- **Rate Limiting** — request throttling per IP
- **Input Validation** — structured validation on all endpoints
- **SQL Injection Prevention** — parameterized queries via GORM

---

## 📊 Monitoring & Observability

- **Prometheus Metrics** — endpoint at `/metrics`
- **OpenTelemetry Tracing** — distributed tracing support
- **Health Check** — endpoint at `/health` with dependency status
- **Structured Logging** — JSON-formatted logs in production

---

## 👥 Contributors

<table>
  <tr>
    <td align="center"><a href="https://github.com/MattYudha"><b>MattYudha</b></a><br/>Matt</td>
    <td align="center"><a href="https://github.com/wreckitral"><b>wreckitral</b></a><br/>Darhanaya Sofhiaa</td>
  </tr>
</table>

---

## 📄 License

This project is for educational and development purposes.
