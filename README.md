# 💳 Wallet Service (`svc-wallet`)

> **Status:** Internal microservice — Wallet business operations are exposed through gRPC. Port 8000 serves Prometheus metrics only; clients use the Users REST gateway on port 8001.

---

## 📌 Overview

The **Wallet Service** is a high-performance Go microservice responsible for creating and querying digital wallet accounts. It consumes transaction events from Kafka (via CDC) to synchronize wallet balances, acting as a read-optimized projection of the authoritative ledger maintained by the Transactions Service.

Key responsibilities:
- **Wallet Lifecycle:** Creates user-owned wallets from `user_id` and phone number over gRPC, with duplicate-phone prevention. The disabled REST handler retains the older identity-based creation flow for possible reuse.
- **Owned Wallets:** Stores each wallet's Users-service `user_id` for creation, user listing, and delete-all operations. Balance and single-wallet deletion use `wallet_id` only.
- **CDC Balance Synchronization:** Consumes `POSTED` transaction events from Kafka topic `transactions_db.transactions` and atomically updates wallet balances using `$set: { balance: balance_after }` with a `processed_refs` sliding window for idempotency.
- **Distributed Observability:** The active gRPC server uses OpenTelemetry instrumentation. The metrics-only HTTP server uses structured request logging and Prometheus middleware; the commented REST routes retain their previous `otelhttp` wrappers.

---

## 🏗 Architecture

```
svc-wallet/
├── cmd/
│   └── main.go                     # Application entry point, wiring, & graceful shutdown
├── api/
│   ├── rest/                       # HTTP Delivery Layer
│   │   ├── routes.go               # Active /metrics plus commented business/Swagger routes
│   │   ├── controller.go           # WalletController wrapper
│   │   └── request_response_handler.go # HTTP request binding, validation, & response encoding
│   └── grpcserver/                 # gRPC Delivery Layer
│       └── wallet_server.go        # Implements all WalletService RPCs and maps domain errors
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
│   └── kafka/                      # Kafka Consumer Integration
│       └── consumer/
│           └── consumer.go         # Sarama Kafka consumer for CDC transaction events
├── util/
│   ├── logger/                     # Zerolog wrapper with context trace injection
│   └── tracer/                     # OpenTelemetry tracer provider setup
├── tests/
│   ├── wallet_acid_test.go         # Integration test suite
│   ├── wallet_acid_test_docs.md    # Integration test documentation & expected states
│   └── run_tests.ps1               # Automated test execution script
├── docs/                           # Historical Swagger files; UI is not served
├── Dockerfile                      # Multi-stage Alpine Docker build
└── go.mod
```

### Clean Architecture Layers

| Layer | Package | Responsibility |
|-------|---------|----------------|
| **Transport / Delivery** | `api/rest`, `api/grpcserver` | Exposes internal metrics over HTTP and wallet business operations over gRPC. |
| **Domain (Core)** | `internal/wallet` | Contains wallet entity, validation rules, business logic, National ID decoding, and repository interfaces. Free of external framework dependencies. |
| **Infrastructure** | `external/mongodb` | Implements repository interfaces using MongoDB Go Driver, manages unique indexes and atomic updates. |
| **Messaging** | `external/kafka/consumer` | Kafka consumer group that ingests CDC events from `transactions_db.transactions`, normalizes MongoDB extended JSON `_id`, and feeds events to the domain service for balance updates. |
| **Cross-Cutting Utilities** | `util/logger`, `util/tracer` | Structured logging with Zerolog and OpenTelemetry distributed tracing context. |

---

## ⚙️ How It Works

### 1. Wallet creation
When Users creates a wallet through the internal `CreateWallet` RPC:
1. Validates the MongoDB-format `user_id` and Egyptian mobile number.
2. Creates an EGP wallet with balance `0`, the supplied owner ID and phone number, and an empty `processed_refs` array.
3. Returns `wallet_id`, `balance`, and `created_at`.
4. A unique `phone_number` index prevents two wallet documents from using the same number.

The older REST creation handler and DTO remain in the source but their route is commented. If restored, that path also accepts owner name, currency, and national ID and derives birth date from the national ID. Gateway-created wallets currently leave those legacy identity fields empty.

### 2. CDC Balance Synchronization (Kafka Consumer)
The normal wallet-balance path consumes transaction events from Kafka:
1. The Transactions Service commits an immutable ledger entry with `status: POSTED` and a calculated `balance_after`.
2. Kafka Connect captures the MongoDB insert via change streams and publishes the event to `transactions_db.transactions`, keyed by `wallet_id`.
3. The Wallet Service Kafka consumer deserializes the event, normalizes the MongoDB extended JSON `_id` field, and calls `ProcessTransactionEvent`.
4. Executes an atomic `FindOneAndUpdate` on MongoDB:
   - **Filter:** `{ phone_number: evt.PhoneNumber, processed_refs: { $ne: evt.ID } }`
   - **Update:** `{ $set: { balance: evt.BalanceAfter, updated_at: now }, $push: { processed_refs: { $each: [evt.ID], $slice: -80 } } }`
5. **Idempotent Replay Handling:** If `FindOneAndUpdate` returns no documents, the repository checks if `processed_refs` already contains the event ID. If so, the event was already processed and the current wallet state is returned safely.
6. **Retry Resilience:** The consumer retries failed events up to 10 times with 500ms backoff before committing the offset.

---

## 📡 API Endpoints

### Operational HTTP API (Default Port: `8000`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/metrics` | Prometheus metrics for internal scraping |

The previous `/v1/wallet...` routes and Wallet Swagger UI are intentionally not registered. Their source is retained for reference. In Compose, neither Wallet port is published to the host; Users and Transactions reach gRPC through the internal Docker network.

To restore the HTTP business API later, uncomment the saved imports and route registrations in `api/rest/routes.go`. The controller and handler source remain present and wired.

> ⚠️ **Note on Balance Modification:**  
> Normal balance synchronization uses the **CDC pipeline**. The legacy `ModifyBalance` RPC remains registered for existing internal compatibility.

### ⚡ gRPC API (Default Port: `50051`)

Defined in contract `wallet.v1.WalletService` (`github.com/Muhmd95/Contracts/wallet/v1`):

| RPC Method | Main request fields | Description |
|------------|---------------------|-------------|
| `CreateWallet` | `user_id`, `phone_number` | Creates a zero-balance EGP wallet and stores its owner ID. |
| `GetUserWallets` | `user_id` | Lists wallets using the `user_id` index. |
| `GetWalletBalance` | `wallet_id` | Uses the existing Redis/Mongo balance lookup. |
| `DeleteWallet` | `wallet_id` | Permanently deletes one wallet and invalidates its cache key. |
| `DeleteUserWallets` | `user_id` | Reads the user's wallet IDs, deletes the documents with `DeleteMany`, then invalidates their Redis keys. Repeated calls succeed. |
| `GetWallet` | `phone_number` | Retrieves wallet ID, owner name, and balance for existing internal consumers. |
| `ModifyBalance` | `phone_number`, `amount`, `referenceID` | Legacy internal method retained for compatibility; CDC remains the normal balance path. |

---

## 🗄 Data Model (`wallets` collection)

```json
{
  "_id": {"$oid": "6701a2b3c4d5e6f7a8b9c0d1"},
  "user_id": {"$oid": "6701a2b3c4d5e6f7a8b9c0c0"},
  "phone_number": "01012345678",
  "owner_name": "",
  "balance": 5000,
  "currency_code": "EGP",
  "national_id": "",
  "birth_date": "0001-01-01T00:00:00Z",
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
| `wallets_by_user` | `{"user_id": 1}` | Non-unique | Makes owner-scoped listing and bulk deletion efficient. |

Wallet documents created before this ownership change have no `user_id`. They remain available to the legacy phone/ID lookups but do not appear in Users owner-scoped operations. Recreate or explicitly migrate development data when testing the gateway.

### Deletion and cache invalidation

- `DeleteWallet` deletes one document by `wallet_id`, then removes `wallet:<wallet_id>` from Redis.
- `DeleteUserWallets` first reads the IDs indexed by `user_id`, executes one `DeleteMany` for that user, then deletes all corresponding Redis keys.
- Balance reads remain cache-first. A cache miss loads the wallet from MongoDB and repopulates Redis.
- Redis invalidation failures are logged after MongoDB deletion; the deletion is not rolled back.

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
KAFKA_BROKERS=localhost:9092
KAFKA_GROUP_ID=wallet-balance-consumer
KAFKA_TOPIC=transactions_db.transactions
REDIS_ADDR=localhost:6379
APP_ENV=development
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318
```

| Variable | Required | Default | Description |
|----------|:--------:|:-------:|-------------|
| `MONGO_URI` | ✅ | — | MongoDB connection string (supports MongoDB Atlas replica set) |
| `MONGO_DB_NAME` | ❌ | `wallet_db` | Target database name |
| `SERVER_PORT` | ❌ | `8000` | Internal HTTP metrics server port |
| `GRPC_SERVER_PORT` | ❌ | `50051` | gRPC server port for inter-service communication |
| `KAFKA_BROKERS` | ✅ | — | Kafka broker addresses (e.g. `localhost:9092` or `kafka:9092`) |
| `KAFKA_GROUP_ID` | ✅ | — | Consumer group ID for CDC balance synchronization |
| `KAFKA_TOPIC` | ✅ | — | Kafka topic containing transaction CDC events (`transactions_db.transactions`) |
| `REDIS_ADDR` | ❌ | `localhost:6379` | Redis address used by the balance cache |
| `APP_ENV` | ❌ | `development` | Uses JSON/info logging when set to `production`; otherwise uses debug console logging |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | ❌ | — | OTLP HTTP collector address, such as `jaeger:4318`; tracing remains local when omitted |

### Observability

- gRPC logs and metrics include the method, result code, and duration. Correlated logs include `trace_id` and `span_id`.
- Service and MongoDB repository work appears as child spans of incoming gRPC traces. Kafka messages start independent traces because CDC records do not currently carry upstream trace context.
- `/metrics` exposes bounded-label HTTP/gRPC latency and request counts, MongoDB operation duration, Redis outcomes, and Kafka outcome/retry/duration metrics. IDs and phone numbers are not metric labels.
- Kafka errors log topic, partition, and offset without logging the raw message payload.

### Run Locally

```bash
cd Wallet-Service/svc-wallet
go run ./cmd
```

### Run with Docker

From the repository root, use `docker compose build wallet` or `docker compose up wallet`. Wallet uses the published Contracts version listed in `go.mod`.

---

## 🧪 Testing

Run the normal suite for the gRPC wallet lifecycle and the internal-only HTTP surface:

```bash
cd Wallet-Service/svc-wallet
go test ./...
```

The older `tests/wallet_acid_test.go` suite targets the disabled Wallet REST API and is kept behind the `integration` build tag as historical material. It must be migrated to Users REST or Wallet gRPC before it can be used again:

```bash
cd Wallet-Service/svc-wallet
go test -v -tags=integration -count=1 ./tests/
```

*Its previous scenarios are described in [wallet_acid_test_docs.md](svc-wallet/tests/wallet_acid_test_docs.md).*

---

## ⚠️ Domain Errors

| Error | Meaning | Commented REST mapping | gRPC Code |
|-------|---------|:-----------:|:---------:|
| `wallet not found` | No wallet matching the provided identifier exists | `404 Not Found` | `NotFound (5)` |
| `phone number is already registered` | Wallet creation conflict on phone number | `409 Conflict` | `AlreadyExists (6)` |
| `invalid phone number format` | Phone number failed validation rules | `400 Bad Request` | `InvalidArgument (3)` |
| `insufficient balance for the requested operation` | Withdrawal would cause negative balance | `400 Bad Request` | `FailedPrecondition (9)` |
| `deposit exceeds maximum wallet capacity` | Deposit would exceed maximum allowable balance | `400 Bad Request` | `FailedPrecondition (9)` |
| `invalid national id format` | National ID failed validation rules | `400 Bad Request` | `InvalidArgument (3)` |
| `invalid wallet id format` | Provided Hex string cannot convert to ObjectID | `400 Bad Request` | `InvalidArgument (3)` |
| `invalid user id format` | Provided Users-service ID is not a MongoDB ObjectID | No legacy mapping | `InvalidArgument (3)` |

### Deletion scope

Deletion is deliberately a hard delete for this learning project. Re-registering a deleted user can create completely new wallets. This does not coordinate with Transactions: historical ledger records may reference deleted wallet IDs, in-flight CDC events may target a deleted phone, and remaining balances are not protected. Treat it as account-data cleanup rather than production financial account closure.
