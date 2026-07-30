# 💳 Wallet Service (`svc-wallet`)

> **Status:** 🚧 Work in Progress — this service is part of a larger Wallet & Transactions microservice system and is not yet complete.

## Overview

The Wallet Service is a Go microservice responsible for managing digital wallets. It exposes both a **REST API** and a **gRPC API**, uses **MongoDB** as its data store, and includes **OpenTelemetry** tracing for distributed observability.

## Architecture

```
svc-wallet/
├── cmd/                     # Application entry point
│   └── main.go
├── api/                     # Transport / delivery layer
│   ├── rest/                # REST API (HTTP handlers, routes, controller)
│   └── grpcserver/          # gRPC API server
├── internal/
│   └── wallet/              # Core domain (model, service, DTOs, repository interface)
├── external/
│   └── mongodb/             # Repository implementation (MongoDB)
├── util/
│   ├── logger/              # Structured logging (zerolog + trace context)
│   └── tracer/              # OpenTelemetry tracer setup
├── docs/                    # Swagger / OpenAPI auto-generated docs
├── Jobs/                    # CI/CD helper scripts
├── Dockerfile               # Multi-stage Docker build
└── go.mod
```

The project follows a **clean-architecture** style layout:

| Layer | Package | Responsibility |
|-------|---------|----------------|
| **Transport** | `api/rest`, `api/grpcserver` | Handle HTTP / gRPC requests, validate input, map errors to status codes |
| **Domain** | `internal/wallet` | Business logic, domain model, DTOs, repository interface |
| **Infrastructure** | `external/mongodb` | MongoDB implementation of the repository interface |
| **Utilities** | `util/logger`, `util/tracer` | Cross-cutting concerns (logging, tracing) |

## Tech Stack

- **Language:** Go 1.26
- **Database:** MongoDB (with unique index on `phone_number`)
- **REST Framework:** Go standard library (`net/http`)
- **gRPC:** google.golang.org/grpc + Protobuf contracts (`github.com/Muhmd95/Contracts`)
- **Logging:** [zerolog](https://github.com/rs/zerolog) with console writer
- **Tracing:** OpenTelemetry SDK (local provider, Kibana exporter planned)
- **API Docs:** Swagger via [swaggo](https://github.com/swaggo/swag)
- **Containerization:** Docker (multi-stage Alpine build)

## API Endpoints

### REST (default port `8000`)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/wallet` | Create a new wallet |
| `GET` | `/v1/wallet/{phone_number}` | Get wallet by phone number |
| `PATCH` | `/v1/wallet/balance` | Modify wallet balance (deposit / withdraw) |
| `GET` | `/v1/swagger/` | Swagger UI |

### gRPC (default port `50051`)

| RPC | Description |
|-----|-------------|
| `ModifyBalance` | Deposit to or withdraw from a wallet |

> Phone numbers must follow the Egyptian format: `+20XXXXXXXXXX` (13 characters total).

## Getting Started

### Prerequisites

- Go 1.26+
- A running MongoDB instance (local or Atlas)

### Configuration

Copy the example env file and fill in your values:

```bash
cp .ENV.example .ENV
```

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `MONGO_URI` | ✅ | — | MongoDB connection string |
| `MONGO_DB_NAME` | ❌ | `wallet_db` | Database name |
| `SERVER_PORT` | ❌ | `8000` | HTTP server port |
| `GRPC_SERVER_PORT` | ❌ | `50051` | gRPC server port |

### Run Locally

```bash
cd svc-wallet
go run ./cmd
```

### Run with Docker

```bash
cd svc-wallet
docker build -t wallet-service .
docker run -p 8000:8000 -p 50051:50051 \
  -e MONGO_URI="your_mongo_uri" \
  -e MONGO_DB_NAME="wallet_db" \
  wallet-service
```

## Domain Errors

| Error | Meaning |
|-------|---------|
| `wallet not found` | No wallet exists for the given phone number |
| `phone number is already registered` | Duplicate wallet creation attempt |
| `invalid phone number format` | Phone number doesn't match `+20XXXXXXXXXX` |
| `insufficient balance` | Withdrawal would result in a negative balance |
| `deposit exceeds maximum wallet capacity` | Deposit would overflow `int64` max |

## Roadmap

- [ ] Family wallet support (`family_id`)
- [ ] Currency code on balance operations
- [ ] Transactions service integration
- [ ] OpenTelemetry exporter (Kibana / Jaeger)
- [ ] Phase 2 refactor: atomic MongoDB transactions for balance updates
