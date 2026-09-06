# 💳 Wallet Service (`svc-wallet`)

> **Status:** Active Microservice — Manages wallet accounts, identity verification, balance state, and internal balance modifications for the Wallet & Transactions system.

---

## 📌 Overview

The **Wallet Service** is a high-performance Go microservice responsible for creating and querying digital wallet accounts, as well as executing atomic balance modifications requested by internal services.

Key responsibilities:
- **Wallet Lifecycle:** Creates wallets with Egyptian National ID birthdate calculation and duplicate phone prevention.
- **Account Queries:** Look up wallets by phone number or internal MongoDB ObjectID via REST and gRPC.
- **Atomic Balance Updates:** Provides a concurrency-safe, idempotent gRPC endpoint (`ModifyBalance`) with upper-bound limits and overdraft protection.
- **Distributed Observability:** Instrumented with OpenTelemetry tracing (`otelhttp`, `otelgrpc`) and structured JSON logging with Zerolog.

---

## 🏗 Architecture

```
svc-wallet/
├── cmd/
│   └── main.go                     # Application entry point, wiring, & graceful shutdown
├── api/
│   ├── rest/                       # HTTP Delivery Layer
│   │   ├── routes.go               # REST route registration & Swagger mounting
│   │   ├── controller.go           # WalletController wrapper
│   │   └── request_response_handler.go # HTTP request binding, validation, & response encoding
│   └── grpcserver/                 # gRPC Delivery Layer
│       └── wallet_server.go        # Implements walletv1.WalletServiceServer (ModifyBalance, GetWallet)
├── internal/
│   └── wallet/                     # Domain Layer (Pure Business Logic)
│       ├── model.go                # Wallet schema, domain error definitions, & constraints
│       ├── dto.go                  # Request/Response data transfer objects & validators
│       ├── repository.go           # Database interface contract (Repository)
│       ├── service.go              # Core domain services & National ID parsing
│       └── tx_manager.go           # Future multi-step transaction interface
├── external/
│   └── mongodb/                    # Infrastructure Layer (Data Access)
│       ├── connection.go           # MongoDB client initialization & health check
│       ├── repo.go                 # MongoDB repository implementation & atomic updates
│       └── tx_manager.go           # MongoDB transaction manager stub
├── util/
│   ├── logger/                     # Zerolog wrapper with context trace injection
│   └── tracer/                     # OpenTelemetry tracer provider setup
├── tests/
│   ├── wallet_acid_test.go         # Integration test suite
│   ├── wallet_acid_test_docs.md    # Integration test documentation & expected states
│   └── run_tests.ps1               # Automated test execution script
├── docs/                           # Auto-generated Swagger/OpenAPI documentation
├── Dockerfile                      # Multi-stage Alpine Docker build
└── go.mod
```

### Clean Architecture Layers

| Layer | Package | Responsibility |
|-------|---------|----------------|
| **Transport / Delivery** | `api/rest`, `api/grpcserver` | Handles HTTP and gRPC protocols, validates inputs, maps domain errors to HTTP/gRPC status codes. |
| **Domain (Core)** | `internal/wallet` | Contains wallet entity, validation rules, business logic, National ID decoding, and repository interfaces. Free of external framework dependencies. |
| **Infrastructure** | `external/mongodb` | Implements repository interfaces using MongoDB Go Driver, manages unique indexes and atomic updates. |
| **Cross-Cutting Utilities** | `util/logger`, `util/tracer` | Structured logging with Zerolog and OpenTelemetry distributed tracing context. |

---

## ⚙️ How It Works

### 1. Wallet Registration & Egyptian National ID Extraction
When a new wallet is created via `POST /v1/wallet`:
1. Validates the Egyptian phone number format (`+20XXXXXXXXXX` or `01XXXXXXXXX`).
2. Validates the 14-digit Egyptian National ID format.
3. Automatically derives the user's birthdate from the National ID:
   - First digit `2` indicates birth century 1900–1999 (e.g. `290...` $\rightarrow$ `1990`).
   - First digit `3` indicates birth century 2000–2099 (e.g. `301...` $\rightarrow$ `2001`).
   - Digits 2–3 indicate Year, 4–5 Month, and 6–7 Day.
4. Inserts the new wallet into MongoDB with an initial balance of `0` and an empty `processed_refs` array.
5. A MongoDB unique index on `phone_number` (`unique_phone`) guarantees no duplicate wallets can be registered for the same phone number.

### 2. Atomic Balance Modification (gRPC Only)
Balance changes are **never executed directly via public REST endpoints** to maintain consistency and enforce a single source of truth through the Transactions Service.
When the Transactions Service invokes `ModifyBalance(ModifyBalanceRequest)`:
1. Validates the amount ($\neq 0$) and phone number format.
2. Executes an atomic `FindOneAndUpdate` operation on MongoDB:
   - **For Deposits ($amount > 0$):**
     $$\text{balance} \le \text{WalletMax} - amount \quad (\text{WalletMax} = 9{,}000{,}000{,}000{,}000{,}000)$$
   - **For Withdrawals ($amount < 0$):**
     $$\text{balance} \ge -amount \quad (\text{overdraft prevention})$$
   - **Idempotency Condition:**
     `processed_refs` does not already contain the incoming `referenceID`.
3. In a single atomic operation:
   - Increments/decrements balance: `$inc: { balance: amount }`
   - Records the reference ID in a bounded sliding window array: `$push: { processed_refs: { $each: [refID], $slice: -80 } }`
   - Sets `updated_at: time.Now()`.
4. **Idempotent Retry Handling:** If no document is modified, the repository checks if `processed_refs` already contains `referenceID`. If it does, the wallet was already credited/debited previously and the current state is returned safely without duplicate modification.

---

## 📡 API Endpoints

### 🌐 REST API (Default Port: `8000`)

| Method | Path | Description | Response Status |
|--------|------|-------------|-----------------|
| `POST` | `/v1/wallet` | Create a new wallet account | `201 Created`, `400 Bad Request`, `409 Conflict` |
| `GET` | `/v1/wallet/phone/{phone_number}` | Retrieve wallet by phone number | `200 OK`, `400 Bad Request`, `404 Not Found` |
| `GET` | `/v1/wallet/{wallet_id}` | Retrieve wallet by MongoDB Hex ObjectID | `200 OK`, `400 Bad Request`, `404 Not Found` |
| `GET` | `/v1/swagger/` | Interactive Swagger API documentation UI | `200 OK` |

> ⚠️ **Note on Balance Modification:**  
> The previous `PATCH /v1/wallet/balance` REST endpoint has been intentionally closed. All balance modifications must go through the **Transactions Service** (`svc-transactions`), which guarantees ACID ledger integrity and calls the Wallet Service via internal gRPC.

### ⚡ gRPC API (Default Port: `50051`)

Defined in contract `wallet.v1.WalletService` (`github.com/Muhmd95/Contracts/wallet/v1`):

| RPC Method | Request Parameters | Response | Description |
|------------|--------------------|----------|-------------|
| `ModifyBalance` | `phone_number`, `amount`, `referenceID` | `wallet_id`, `balance`, `updated_at` | Atomically deposits or withdraws funds with built-in idempotency check. |
| `GetWallet` | `phone_number` | `wallet_id`, `owner_name` | Retrieves wallet ID and owner metadata for inter-service lookups. |

---

## 🗄 Data Model (`wallets` collection)

```json
{
  "_id": {"$oid": "6701a2b3c4d5e6f7a8b9c0d1"},
  "phone_number": "+201012345678",
  "owner_name": "Ahmed Mohamed",
  "balance": 5000,
  "currency_code": "EGP",
  "national_id": "29505121234567",
  "birth_date": "1995-05-12T00:00:00Z",
  "processed_refs": ["6701a2c0c4d5e6f7a8b9c0d2"],
  "family_id": null,
  "created_at": "2026-09-06T10:00:00Z",
  "updated_at": "2026-09-06T10:05:00Z"
}
```

### Database Indexes

| Index Name | Keys | Properties | Purpose |
|------------|------|------------|---------|
| `unique_phone` | `{"phone_number": 1}` | `Unique: true` | Prevents duplicate wallet creation for the same phone number. |

---

## 🚀 Getting Started

### Prerequisites
- **Go 1.22+** (configured for Go 1.26 toolchain)
- **MongoDB** instance (local or MongoDB Atlas replica set)
- Shared Contracts module (`github.com/Muhmd95/Contracts`)

### Environment Configuration

Create a `.ENV` file inside `svc-wallet/`:

```env
SERVER_PORT=8000
GRPC_SERVER_PORT=50051
MONGO_URI=mongodb://localhost:27017
MONGO_DB_NAME=wallet_db
```

| Variable | Required | Default | Description |
|----------|:--------:|:-------:|-------------|
| `MONGO_URI` | ✅ | — | MongoDB connection string (supports MongoDB Atlas replica set) |
| `MONGO_DB_NAME` | ❌ | `wallet_db` | Target database name |
| `SERVER_PORT` | ❌ | `8000` | HTTP REST server port |
| `GRPC_SERVER_PORT` | ❌ | `50051` | gRPC server port for inter-service communication |

### Run Locally

```bash
cd Wallet-Service/svc-wallet
go run ./cmd
```

### Run with Docker

```bash
cd Wallet-Service/svc-wallet
docker build -t svc-wallet .
docker run -p 8000:8000 -p 50051:50051 \
  -e MONGO_URI="mongodb+srv://<user>:<password>@cluster.mongodb.net" \
  -e MONGO_DB_NAME="wallet_db" \
  svc-wallet
```

---

## 🧪 Integration Testing

The service includes integration tests validating wallet creation and uniqueness constraints:

```bash
cd Wallet-Service/svc-wallet
go test -v -tags=integration -count=1 ./tests/
```

*For detailed test scenarios and expected database state, refer to [wallet_acid_test_docs.md](svc-wallet/tests/wallet_acid_test_docs.md).*

---

## ⚠️ Domain Errors

| Error | Meaning | REST Status | gRPC Code |
|-------|---------|:-----------:|:---------:|
| `wallet not found` | No wallet matching the provided identifier exists | `404 Not Found` | `NotFound (5)` |
| `phone number is already registered` | Wallet creation conflict on phone number | `409 Conflict` | — |
| `invalid phone number format` | Phone number failed validation rules | `400 Bad Request` | `InvalidArgument (3)` |
| `insufficient balance for the requested operation` | Withdrawal would cause negative balance | `400 Bad Request` | `FailedPrecondition (9)` |
| `deposit exceeds maximum wallet capacity` | Deposit would exceed maximum allowable balance | `400 Bad Request` | `FailedPrecondition (9)` |
| `invalid national id format` | National ID failed validation rules | `400 Bad Request` | `InvalidArgument (3)` |
| `invalid wallet id format` | Provided Hex string cannot convert to ObjectID | `400 Bad Request` | `InvalidArgument (3)` |
