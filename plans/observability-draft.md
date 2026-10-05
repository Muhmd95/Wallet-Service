# Wallet observability draft

Status: implemented on 2026-09-28 with minimal instrumentation. No new tests were created, as requested.

Goal: bring Wallet logging, tracing, and metrics up to the Users service conventions, with fixes for gaps shared by both implementations. Adapt this work to the planned Users gateway and Wallet gRPC architecture.

## Proposed work

1. Use consistent structured logs with API, service, and repository layer fields. Record gRPC method, result code, duration, and trace identifiers. Keep routine detailed success logs at debug level. Avoid logging raw Kafka payloads or unnecessary personal data.
2. Add service and repository spans under incoming gRPC traces. Record unexpected failures on spans. Give Kafka record processing its own trace unless an actual upstream trace context is present; do not invent continuity across CDC.
3. Add bounded-label gRPC request counts and duration histograms, observe the existing Mongo duration metric, and add Redis hit/miss/error and Kafka outcome/retry/duration metrics. Never use wallet IDs, phone numbers, or transaction IDs as metric labels.
4. Where HTTP instrumentation remains, use matched route patterns rather than raw paths and ensure request logs can see the request span. Wallet business HTTP instrumentation may become unnecessary after the gateway change.
5. Load environment configuration before initializing logging and tracing. Bound trace-export shutdown time. Keep Prometheus scraping functional.
6. Verify trace correlation, status and duration fields, bounded metric labels, and dependency-failure visibility with focused tests. Run live smoke checks only when dependencies are available.
7. Update the Wallet README with configuration, metrics, and trace/log examples. Report every changed file and validation result for review.

## Boundaries

Do not change money calculations, cache fallback behavior, or Kafka retry/commit policies as part of this observability work. Document any discovered behavior problems separately.

Likely files: `svc-wallet/util/{logger,tracer,metrics}`, `svc-wallet/api/grpcserver`, `svc-wallet/internal/wallet/service.go`, `svc-wallet/external/{mongodb,redis,kafka}`, `svc-wallet/cmd/main.go`, relevant tests, and `README.md`.

The active gateway implementation proposal is in `users-gateway-plan.md`.
