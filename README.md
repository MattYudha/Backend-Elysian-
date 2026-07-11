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

### EIP-1559 Dynamic Gas Transaction:
Sistem menggunakan transaksi tipe **EIP-1559 Dynamic Fee** untuk menjamin transaksi terkirim dengan cepat dan tidak *stuck* di mempool Sepolia. Estimasi gas fee ditentukan secara dinamis:
- **Base Fee:** Didapatkan langsung dari block header terbaru.
- **Tip Cap (Max Priority Fee):** Menggunakan estimasi dari client RPC go-ethereum dengan batas minimum (safety floor) 1.5 Gwei.
- **Fee Cap:** Dihitung sebagai `(2 * Base Fee) + Tip Cap`.


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

### 🏛️ Nemesis DB Setup (Ground Truth 1 Juta Data)
Untuk menjalankan fungsi QA Gate dan pencarian berbasis kecocokan samar, Anda perlu memulihkan (restore) data ground truth ke PostgreSQL lokal Anda:
1. Buat database `nemesis_db` (gunakan PostgreSQL CLI atau pgAdmin):
   ```bash
   createdb -U postgres nemesis_db
   ```
2. Jalankan perintah `pg_restore` untuk mengimpor data terkompresi (~40MB) berisi 999.998 baris data RUP/procurement & 3 baris standar harga dari file:
   ```bash
   pg_restore -U postgres -h localhost -d nemesis_db -v migrations/nemesis_db.dump
   ```

# 2. Install dependencies
go mod tidy

# 3. Setup config
cp config/config.yml config/config.dev.yml
# Edit config.yml with your settings

# 4. Run migrations (auto on startup)
go run cmd/server/main.go

# 5. Server starts on
http://localhost:7777

### 🐳 Production Docker Deployment
Infrastruktur Docker produksi telah diselaraskan dengan praktik terbaik keamanan:
- **Jaringan Terisolasi (`backend_net`):** Jaringan internal diatur dengan parameter `internal: true`. PostgreSQL dan Redis hanya terhubung ke jaringan internal (`backend_net`) sehingga terisolasi penuh dari lalu lintas eksternal. Hanya service web backend (`app`) yang memiliki akses ganda ke `frontend_net` dan `backend_net` untuk menerima lalu lintas API secara eksternal.
- **Sistem Pengguna Non-Root (`appuser`):** Container aplikasi berjalan menggunakan system user non-root `appuser` (dalam grup `appgroup`), mencegah privilege escalation pada container host.
- **Resource Limits:** Masing-masing container dibatasi penggunaan CPU dan memory di tingkat orchestrator untuk menjamin stabilitas dan ketahanan sistem.

#### 🚀 Urutan Eksekusi Deployment Produksi
Saat Anda melakukan git push dan menerapkan arsitektur baru ini ke server produksi, jalankan perintah berikut secara berurutan di terminal server:

1. **Jalankan migrasi database baru:**
   ```bash
   # Memasang indeks baru di database produksi menggunakan goose
   goose -dir migrations postgres "postgres://username:password@host:port/dbname?sslmode=disable" up
   ```

2. **Bangun ulang container tanpa menggunakan cache:**
   ```bash
   # Menjamin perubahan non-root user (appuser) dan konfigurasi session pooling termuat sempurna
   docker-compose -f docker-compose.prod.yml build --no-cache
   ```

3. **Nyalakan kembali seluruh layanan:**
   ```bash
   # Jalankan semua layanan yang telah diupdate dalam mode daemon
   docker-compose -f docker-compose.prod.yml up -d
   ```
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

- **Penyelarasan Tenant ID (Strict UUID Type):** Parameter `tenantID` diselaraskan secara ketat sebagai tipe `uuid.UUID` pada layer HTTP handler dan usecase. Ini mencegah risiko SQL injection dan input malformed di level API entry point.
- **API Pagination Protection:** Pembatasan ketat pada parameter `limit` maksimal **100** (default `20`) diterapkan untuk mencegah eksploitasi kehabisan memori (DoS/OOM) pada endpoint list data (misal: `/api/v1/documents`).
- **Sanitasi Error HTTP 500 (Anti Information Disclosure):** Penanganan error internal (HTTP 500) disanitasi secara generik sehingga hanya mengembalikan `"An internal server error occurred"`. Detail error asli dialihkan ke secure logger backend untuk menghindari kebocoran detail infrastruktur internal ke pengguna.
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
