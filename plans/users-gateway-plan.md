# Wallet behind the Users gateway: implementation proposal

Status: scope corrected during implementation on 2026-09-28. Users-Service changes were removed. Wallet-ID operations remain keyed only by `wallet_id`; `user_id` is used for create, list, and delete-all.

## Intended behavior

Clients use Users REST endpoints on port 8001. Users calls Wallet over internal gRPC on port 50051. Wallet continues consuming transaction events and exposing internal Prometheus metrics on port 8000. Its business REST routes and Swagger are disabled.

Keep the existing structure:

`gRPC handler -> wallet service -> repository interface -> MongoDB implementation`

Handlers validate/translate requests and map errors. Business rules live in the service. Database queries live in the repository. Redis remains a balance cache; Transactions remains the ledger authority.

## 1. Disable business HTTP access

- Stop registering the four `/v1/wallet` routes and `/v1/swagger/`. Keep their source files for later reuse; do not leave large blocks of commented code.
- Register `/metrics` independently so disabling business routes does not disable monitoring.
- Remove Wallet's host port publications from Compose; other containers still reach `wallet:50051` and Prometheus reaches `wallet:8000`. Document local development access separately.
- Keep the existing gRPC methods used by Transactions available. This work does not make Users a gateway for Transactions REST endpoints.

## 2. Align ownership, creation data, and Contracts

- Add `user_id` to Wallet documents and an index for owner-based queries. Preserve the unique phone-number index.
- Existing wallets have no owner ID. Do not guess ownership or automatically assign them by phone. Leave them available to existing internal lookup flows but exclude them from Users owner-based operations until explicitly migrated.
- Keep the existing creation contract: `user_id` and `phone_number`. Wallet defaults the currency to EGP; the older identity fields are left empty for gateway-created wallets.
- Keep balance and single-wallet deletion keyed only by `wallet_id`. They do not accept or check `user_id`.
- Regenerate the local protobuf bindings and update Wallet to the published Contracts version already used by Users. Keep the existing service Docker build contexts.
- Reuse domain validation from the service boundary so gRPC cannot bypass checks that currently run only in REST handlers.

## 3. Implement the five Users-facing RPCs

| RPC | Proposed behavior |
| --- | --- |
| `CreateWallet` | Validate user ID and phone, create a zero-balance EGP wallet, persist owner ID, and return ID/balance/creation time. |
| `GetUserWallets` | Query active wallets by owner; return an empty list when none exist. Return currency and timestamps from stored data. |
| `GetWalletBalance` | Validate the wallet ID and use the existing Redis/Mongo balance lookup. |
| `DeleteWallet` | Permanently delete the document identified by `wallet_id` and invalidate its cache key. |
| `DeleteUserWallets` | Read the user's wallet IDs, permanently delete them with `DeleteMany`, then invalidate their Redis keys. Repeated calls are safe when none remain. |

Use explicit domain errors and consistent gRPC codes: `InvalidArgument`, `NotFound`, `AlreadyExists`, `FailedPrecondition`, and masked `Internal`. Treat another owner's wallet as not found. Preserve cancellation/deadline semantics.

## 4. Approved deletion scope

Wallet deletion physically removes MongoDB wallet documents and invalidates their Redis balance keys. User account deletion removes all documents indexed by that `user_id` before marking the Users record deleted. Re-registration keeps the Users record identity but starts with no wallet IDs, allowing entirely new wallets.

This deliberately does not coordinate with Transactions. Old ledger records can therefore reference deleted wallet IDs, an in-flight transaction event can target a deleted wallet, and a remaining projected balance is not protected. This is accepted for the learning-project scope and is not production-grade financial account closure.

## 5. Users integration boundary

No Users-Service source, documentation, build, or Swagger changes are part of this implementation. Its existing gRPC client already supplies `user_id` for create/list/delete-all and `wallet_id` for balance/delete-one.

## 6. Verification and documentation

- Verify business HTTP routes and Swagger return 404 while `/metrics` remains accessible internally.
- Test all five RPCs: valid requests, malformed IDs, missing fields, duplicates/retries, empty lists, ownership rejection, dependency failures, and accurate response timestamps.
- Test deletion behavior selected during review, repeated deletion, cache access after deletion, and bulk failure behavior.
- Run affected Go tests and builds, and compile existing Transactions integration against the updated contract. Validate Compose and Docker builds when available.
- With dependencies available, smoke-test Users -> gRPC -> Mongo/Redis and existing transaction-event processing. Report any checks that cannot run.
- Update the Wallet README and mark Wallet REST documentation as disabled. Users documentation and Swagger remain unchanged.

## Expected file areas

| Area | Planned edits |
| --- | --- |
| Wallet HTTP startup | `svc-wallet/cmd/main.go`, `svc-wallet/api/rest/routes.go` |
| Wallet gRPC | `svc-wallet/api/grpcserver/wallet_server.go`, focused tests |
| Wallet domain | `svc-wallet/internal/wallet/model.go`, `dto.go`, `repository.go`, `service.go`, tests |
| Wallet persistence/cache | `svc-wallet/external/mongodb/repo.go`; Redis integration only where lifecycle/cache correctness requires it |
| Contracts | `Contracts/wallet/v1/wallet.proto` and regenerated bindings |
| Builds | Wallet `go.mod` and `go.sum` |
| Documentation | Wallet README only |

## Review and change reporting

The broader logging/tracing/metrics refinement remains deferred in `observability-draft.md`.
