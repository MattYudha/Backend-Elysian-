<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25.5-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/Gin-Framework-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Gin"/>
  <img src="https://img.shields.io/badge/PostgreSQL-16-316192?style=for-the-badge&logo=postgresql&logoColor=white" alt="PostgreSQL"/>
  <img src="https://img.shields.io/badge/Redis-8-DC382D?style=for-the-badge&logo=redis&logoColor=white" alt="Redis"/>
  <img src="https://img.shields.io/badge/Blockchain-Sepolia-3C3C3D?style=for-the-badge&logo=ethereum&logoColor=white" alt="Blockchain"/>
  <img src="https://img.shields.io/badge/Solidity-0.8.28-363636?style=for-the-badge&logo=solidity&logoColor=white" alt="Solidity"/>
</p>

# ⚡ Elysian Rebirth — Backend v3.0

> **Go Backend untuk Infrastruktur Audit Finansial Otonom berbasis Multi-Agent Swarm Intelligence + Blockchain Audit Trail**

---

## 🎯 Apa itu Elysian?

**Elysian Rebirth** mendeteksi **markup anggaran** pada tahap perencanaan (Pre-Audit) di Pemerintah Daerah Indonesia menggunakan:
- 🤖 **Multi-Agent Swarm** (Auditor → Compliance → Manager)
- 🔗 **Blockchain Audit Trail** (Immutable hash storage)
- 📊 **Real-time SSE Streaming** (Live debate logs)

---

## 🏗️ Architecture v3.0

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Elysian Backend (Go / Gin) — The Orchestrator & Interface                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                │
│  │ HTTP Server  │◄──►│ Swarm        │◄──►│ Blockchain   │                │
│  │ · Auth       │    │   Usecase    │    │   Service    │                │
│  │ · Documents  │    │ · Trigger    │    │ · InsertLog  │                │
│  │ · RAG Search │    │ · Callback   │    │ · VerifyHash │                │
│  │ · Workflows  │    │ · SSE Stream │    │ · WaitConf   │                │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘                │
│         │                   │                   │                          │
│    ┌────▼────┐         ┌────▼────┐         ┌────▼────┐                   │
│    │PostgreSQL│         │  Redis  │         │ Sepolia │                   │
│    │· IAM    │         │· Queue  │         │ Testnet │                   │
│    │· Docs   │         │· PubSub │         │· Audit  │                   │
│    │· Swarm  │         │· Cache  │         │  Trail  │                   │
│    └─────────┘         └─────────┘         └─────────┘                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Blockchain Integration Flow:
```
Python Worker Callback
    ↓
Go HandleCallback()
    ↓
Save hashes to DB → Publish SSE to FE
    ↓
Spawn Goroutine → InsertLog() to Sepolia
    ↓
Wait Confirmation → Update DB: VERIFIED
```

---

## 🛠️ Tech Stack

| Layer | Technology |
|-------|-----------|
| **Language** | Go 1.25.5 |
| **Framework** | Gin (HTTP) + GORM (ORM) |
| **Database** | PostgreSQL 16 with pgvector |
| **Cache/Queue** | Redis 8 (go-redis + Asynq) |
| **Blockchain** | go-ethereum v1.15.11 |
| **Smart Contract** | Solidity 0.8.28 (Hardhat) |
| **Auth** | JWT (RS256) + Argon2id + HTTP-Only Cookies |
| **Migration** | Goose (embedded) |
| **Docs** | Swagger/OpenAPI |
| **Monitoring** | Prometheus metrics |

---

## 📁 Project Structure

```
be/
├── cmd/
│   ├── server/              # Entry point (main.go)
│   └── seed_admin/          # Admin seeder CLI
├── config/
│   ├── config.yml           # Main config (blockchain enabled)
│   └── config.dev.yml       # Development overrides
├── internal/
│   ├── config/              # Config loader + blockchain config
│   ├── domain/              # Entities (SwarmTask, User, Document)
│   ├── delivery/http/       # Handlers + Routes
│   ├── usecase/             # Business logic
│   │   ├── auth/            # Auth + JWT + Argon2id
│   │   ├── swarm/           # Swarm trigger + callback + blockchain
│   │   ├── document/        # Document + RAG
│   │   └── workflow/        # Workflow engine
│   ├── repository/postgres/ # DB implementations
│   ├── infrastructure/      # External services
│   │   ├── blockchain/      # 🔗 go-ethereum AuditTrail binding
│   │   ├── cache/           # Redis client
│   │   ├── database/        # PostgreSQL connection
│   │   └── mq/              # Asynq queue
│   └── middleware/          # Auth, RBAC, Logger, Recovery
├── migrations/              # Goose SQL migrations
├── docs/                    # Swagger docs
└── README.md                # This file
```

---

## 🔗 Blockchain Service

### Contract: AuditTrail.sol

| Function | Description |
|----------|-------------|
| `insertLog(taskId, rationaleHash, consensusHash)` | Simpan hash ke blockchain |
| `correctLog(oldTaskId, ...)` | Revisi hash (supersede) |
| `getActiveLog(taskId)` | Ambil log terbaru |
| `verifyHashes(taskId, ...)` | Verifikasi hash match |

### Deployment:
| Network | Sepolia Testnet |
|---------|----------------|
| Chain ID | 11155111 |
| Contract | `0x50d7A710C1a06b15Ee61669007279E03E4B2f233` |
| Deployer | `0x03252339418744A98F03D4ED979dF36Cd75308D4` |

### Config (`config.yml`):
```yaml
blockchain:
  enabled: true
  rpc_url: "https://eth-sepolia.g.alchemy.com/v2/YphaD1AyIb34KtIp9xpXD"
  contract_addr: "0x50d7A710C1a06b15Ee61669007279E03E4B2f233"
  private_key: "0x..."
  network: "sepolia"
```

---

## 🚀 Quick Start

```bash
# 1. Setup PostgreSQL + Redis
# PostgreSQL: localhost:5432 (trust auth for dev)
# Redis: localhost:6379

# 2. Install dependencies
go mod tidy

# 3. Setup config
cp config/config.yml config/config.dev.yml
# Edit config.yml with your settings

# 4. Run migrations (auto on startup)
go run cmd/server/main.go

# 5. Server starts on
http://localhost:7777
```

---

## 📡 API Endpoints

### Auth:
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/register` | Public | Register user |
| POST | `/api/v1/auth/login` | Public | Login + set cookies |
| POST | `/api/v1/auth/refresh` | Public | Refresh token |
| POST | `/api/v1/auth/logout` | Public | Clear cookies |

### Swarm:
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/swarm/upload` | Bearer | Trigger Swarm Review |
| POST | `/api/v1/swarm/callback` | Internal | Python worker callback |
| GET | `/api/v1/swarm/events` | Open | SSE streaming |

### Documents:
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/documents/presign` | Bearer | Presigned S3 URL |
| POST | `/api/v1/documents/confirm` | Bearer | Confirm upload |
| POST | `/api/v1/documents/search` | Bearer | Hybrid RAG Search |

---

## 🔐 Security

- **Password Hashing:** Argon2id (memory=64MB, iterations=3, parallelism=4)
- **JWT:** RS256 asymmetric, 15min access / 30day refresh
- **Cookies:** HttpOnly, Secure, SameSite=Strict
- **Rate Limiting:** Auth endpoints rate-limited per IP
- **Blockchain:** Private key never exposed to client

---

## 🏛️ Elysian Ecosystem

| Repo | Role | Stack |
|------|------|-------|
| [Frontend](https://github.com/MattYudha/Frontend-Elysian-Rebirth) | Next.js 14 UI | TypeScript + Tailwind |
| [Backend](https://github.com/MattYudha/Backend-Elysian-) | Go API Server | Go + Gin + PostgreSQL |
| [ML](https://github.com/MattYudha/ML-ELYSIAN) | Python Swarm | Flask + OpenAI |
| [Trust Layer](https://github.com/MattYudha/Backend-Elysian-/tree/main/trust-layer) | Smart Contract | Solidity + Hardhat |

---

> **Versi:** 3.0.0 (Blockchain-Integrated)  
> **Tanggal:** Mei 2026  
> **Pemilik:** Matt (Team Elysian)
