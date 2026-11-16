# Research notes (goload)

Collected during production review and context-engineering work. Practical notes tied to this repo — not a generic reading list. Update when a decision lands in an [ADR](adr/) or [lesson](ai/lessons.md).

---

## 1. Dual-write and transactional outbox

**Problem in goload:** `internal/task/service.go` publishes Kafka events while holding or right after DB writes. Commit-without-publish and publish-without-commit both happen under partial failure.

**Pattern:** Transactional outbox — insert domain row + outbox row in one SQL transaction; a relay publishes to Kafka and marks the outbox sent.

**Variants:**
| Approach | Fit for goload |
|---|---|
| Polling relay (`SELECT … FOR UPDATE SKIP LOCKED`) | Simplest; good first step for task service |
| CDC (Debezium → Kafka) | Stronger at scale; more ops |
| Kafka transactions + DB | Poor fit with MySQL as source of truth |

**Refs:** microservices.io Outbox; [ADR-0003](adr/0003-transactional-outbox.md).

---

## 2. Consumer Ack timing for long downloads

**Problem:** Download consumer Ack'd when `ExecuteTask` *started*. Broker believed the message done; pause/cancel and crash recovery were wrong.

**Options considered:**
| Option | Idea | Chosen? |
|---|---|---|
| **A** Ack after completion | Processor owns message until success/terminal handling | **Yes** ([ADR-0004](adr/0004-consumer-ack-after-completion.md)) |
| **B** Ack on start + work lease | Persist in-flight work; separate cancel channel | Deferred (more state) |

**Implication:** Consumer lag ≈ in-flight downloads; need concurrency limits and shutdown drain. Still at-least-once → combine with idempotent handlers.

**pkg/message:** Subscriber docs already say Ack after processing / Nack for redelivery — Option A matches the port contract.

---

## 3. Database per service vs local shared MySQL

**Industry default:** One database (or schema) per service; no cross-service SQL joins.

**goload today:** Single MySQL + shared migrations for auth and task in docker-compose.

**Decision:** Production = DB-per-service ([ADR-0001](adr/0001-database-per-service.md)). Local may share a MySQL *server* ([ADR-0009](adr/0009-local-shared-mysql.md)); prefer separate DB names even locally.

---

## 4. Pause/resume reachability (status events)

**Finding:** Task publishes `task.status.updated` when download reports progress/status, but **no service consumes that topic**. Status never becomes `DOWNLOADING`, so pause/resume guards that require that state never succeed in the intended flow.

**Research takeaway:** “Event-driven” is incomplete without a subscriber inventory. Either:
1. Add a task (or download) consumer for status updates that applies transitions, or
2. Stop publishing and update status via a different path (e.g. completion/failure only + explicit start transition in the start handler).

Tracked in [legacy-map](ai/legacy-map.md) and [data-flows](architecture/data-flows.md).

---

## 5. Idempotent consumers and state machines

**Finding:** Completion/failure handlers can apply updates without checking current status → late `failed` can overwrite `COMPLETED`.

**Pattern:** Explicit transition table (`PENDING→DOWNLOADING`, `DOWNLOADING→COMPLETED`, …); ignore/ack redundant events; reject illegal transitions with metrics.

**Test idea:** Deliver `task.failed` twice and after `COMPLETED`; assert final state and Ack behavior ([TEST-8](standards/testing.md)).

---

## 6. go-kit: what we use and what we don’t yet

**Already in repo:** endpoints, HTTP/gRPC transport, logging middleware, ratelimit endpoint middleware, kit log.

**Useful and not yet wired (aligned with standards):**
- `github.com/go-kit/kit/sd` + Consul or k8s — instance discovery ([MS-17](standards/microservices.md), [ADR-0006](adr/0006-kubernetes-deployment.md))
- `lb.Retry` + `circuitbreaker.Gobreaker` on gateway→auth/task clients ([MS-13](standards/microservices.md), [MS-14](standards/microservices.md))
- `endpoint.Chain` for tracing/metrics once OTel lands

**Logging:** User preference is keep **go-kit/log** (copied lint stack is not a logging decision). Bridge to OTel/`trace_id` via middleware rather than rewriting all call sites to slog.

---

## 7. OpenTelemetry + Prometheus with Go microservices

**Target stack ([ADR-0007](adr/0007-observability-otel-prometheus.md)):**
- OTel SDK: HTTP (otelhttp), gRPC interceptors, optional Kafka instrumentation
- Export OTLP → collector
- Prometheus scrape of `/metrics` (or OTel Prometheus exporter)

**Correlation:** Propagate `X-Request-ID` / W3C `traceparent` on HTTP; gRPC metadata; Kafka headers ([OBS-8](standards/observability.md)). `pkg/middleware/requestid.go` is the HTTP seed — extend outward.

**RED for apigateway; consumer lag + download duration for download/task.**

---

## 8. API versioning, RFC 7807, path style

**RFC 7807 (`application/problem+json`):** Standard fields `type`, `title`, `status`, `detail`; extensions for `code`, `request_id`, `errors[]`. Good fit for v2 ([ADR-0008](adr/0008-api-v1-deprecate-v2-problem-json.md)).

**Path style research:** Resource-oriented REST (`POST /tasks/{id}/pause`) is common in greenfield APIs. This project chose **consistency with existing RPC-style** (`/tasks/pause?id=`) across v1/v2 to avoid dual client conventions.

**ID encoding:** JSON numbers cannot safely represent all uint64 values; v2 should serialize 64-bit IDs as strings.

**Deprecation mechanics:** `Deprecation: true` + eventual `Sunset: <HTTP-date>` (RFC 8594).

---

## 9. SSRF on user-controlled download URLs

**Already mitigated in** `internal/download/downloader/http.go` (IP allow/deny after resolve, dialer `Control`, redirect checks).

**Keep in mind:** DNS rebinding (re-check IP on redirect); IPv6 mapped addresses; disable or tightly proxy `file://` and metadata hosts (`169.254.169.254`). Regression tests belong in the downloader package.

---

## 10. Rate limiting placement

**Finding:** Auth/task endpoint limiters at ~1 req/s globally are far too aggressive and mismapped to HTTP 500.

**Research default:** Edge rate limit (apigateway / ingress) per account or IP; service-local limits only for expensive internals. Map overload to `TOO_MANY_REQUESTS` / gRPC `ResourceExhausted`.

---

## 11. Pocket / pocketsrv as deployment modes

**Model:** Same domain cores; different composition roots (`cmd/pocket`, `cmd/pocketsrv`) with SQLite/local adapters vs full MySQL/Kafka/MinIO.

**Risk:** Duplicated DDL in `main.go` drifts from MySQL migrations ([CC-18](standards/clean-code.md)).

**Direction:** One schema source; adapters differ; core must not branch on “pocket vs prod”.

---

## 12. Testing pyramid for this codebase

| Layer | Current state | Direction |
|---|---|---|
| Unit + fakes | Auth service tests are the best exemplar | Expand task/download with fakes |
| Race | PausableProgressReader race fixed; CI should run `-race` | Always in CI |
| Integration | Kafka package uses testcontainers | Tag `integration`; MySQL/MinIO similarly |
| Contract | OpenAPI/proto breaking not enforced in CI | `buf breaking` + openapi filter tests |
| E2E | Manual / compose | One `make smoke` path |

User preference: don’t over-invest in process theater; keep guidance actionable ([testing](standards/testing.md)).

---

## 13. Linting

`.golangci.yml` was copied from an online template — many linters are advisory. **Gate on:** `errcheck`, `govet`, `staticcheck`, `gosec`, `errorlint`, `bodyclose`. Soft on line-count / style nits ([CC-10](standards/clean-code.md), [CC-16](standards/clean-code.md)).

---

## 14. Dead API surface

Task gRPC exposes Update*/CompleteTask-style RPCs that download does not call (Kafka is the path). Keeping unused RPCs expands attack and maintenance surface ([API-16](standards/api-design.md)). Prefer delete or a clearly named internal worker service.

---

## 15. Storage and download URL issuance

Gateway reading MinIO/Redis for download URLs crosses ownership. Cleaner model: task (or download) issues a short-lived URL or stream token; gateway only proxies authn/authz.

`Presigner` in `internal/storage` is the right port; wire it deliberately (avoid `WithPresigner(nil)` confusion in `cmd/task`).

---

## 16. Kubernetes vs Consul

**Decision lean:** Kubernetes as target ([ADR-0006](adr/0006-kubernetes-deployment.md)); DNS-based discovery first. Consul + go-kit `sd` remains valid for hybrid/bare-metal.

**Probes:** liveness = process up (`/health`); readiness = deps (DB/Kafka) when failing closed is correct for that service.

---

## 17. Error code matrix (HTTP ↔ gRPC)

Canonical mapping lives in [error-handling](standards/error-handling.md). Review work extended `EncodeGRPCError` and HTTP domain mapping so pocket and gateway agree. Remaining gap: domain `internal/errors` still imports gRPC — move encode helpers to transport.

---

## 18. Priority backlog (from review, not yet all fixed)

Highest leverage for correctness:
1. Download consumer Option A (Ack after completion)
2. Consume or delete `task.status.updated` path (pause/resume)
3. Task outbox
4. State-guarded completion/failure
5. Split MySQL DBs + per-service migrations
6. OTel + request ID propagation
7. Gateway v2 problem+json + deprecate v1
8. Remove unused task Update RPCs / fix checksum Update noop

---

## Document history
- 2026-11-15: Initial capture from production review + Phases 1–4 context engineering.
