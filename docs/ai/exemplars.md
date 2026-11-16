# Exemplars

Copy these when writing new code. Only files that largely follow [docs/standards](../standards/) are listed. If a concern has **no exemplar**, do not invent from legacy code — propose a golden-path refactor in that service first.

| Concern | Exemplar | Why | Gaps vs standards |
|---|---|---|---|
| Domain ports (core) | [internal/download/downloader.go](../../internal/download/downloader.go) | Small role interface, ctx first, no framework imports | — |
| Storage ports | [internal/storage/storage.go](../../internal/storage/storage.go) | Reader/Writer split, optional Presigner | `FileMetadata` has JSON tags (acceptable as shared DTO; prefer mapping at boundary for new types) |
| Messaging ports | [pkg/message/pubsub.go](../../pkg/message/pubsub.go) | Publisher/Subscriber contracts, Ack/Nack model | — |
| Use case / service | [internal/auth/service.go](../../internal/auth/service.go) | Ports owned by core, tx for multi-write, typed `errors.Error` | Login enumeration (ERR-12); depends on `internal/errors` which still imports gRPC |
| HTTP auth middleware | [internal/apigateway/middleware.go](../../internal/apigateway/middleware.go) | Thin, returns domain errors | — |
| Ownership check | [internal/apigateway/owner_middleware.go](../../internal/apigateway/owner_middleware.go) | Clear authz boundary | Extra gRPC hop (MS-15); imports `task` domain (CC-3) |
| Request ID | [pkg/middleware/requestid.go](../../pkg/middleware/requestid.go) | Generate/propagate, sanitize, context helpers | Not yet forwarded over gRPC/Kafka (OBS-8) |
| Panic recovery | [pkg/middleware/recovery.go](../../pkg/middleware/recovery.go) | HTTP + gRPC recover → safe INTERNAL | — |
| HTTP error mapping tests | [internal/apigateway/transport_error_test.go](../../internal/apigateway/transport_error_test.go) | Table-driven status/body/`request_id` | — |
| Service unit tests | [internal/auth/service_test.go](../../internal/auth/service_test.go) | Hand fakes for ports, asserts error codes | — |
| Kafka adapter tests | [pkg/message/kafka/kafka_test.go](../../pkg/message/kafka/kafka_test.go) | testcontainers, Ack/Nack redelivery | Uses `-short` skip; prefer `//go:build integration` (TEST-7) |
| SSRF-safe HTTP client | [internal/download/downloader/http.go](../../internal/download/downloader/http.go) (`isDisallowedIP`, dialer Control) | Blocks private/link-local targets | File is large (CC-10 guideline); split probes vs download when touching |

## No exemplar yet (do not copy legacy)
| Concern | Closest code | Why not exemplar | Golden-path target |
|---|---|---|---|
| Repository (MySQL) | `internal/auth/mysql/*`, `internal/task/mysql/task_repo.go` | Mixed error typing; task `Update` drops fields; `Begin()` without ctx | Refactor auth mysql stores first (smaller) |
| Event producer | `internal/*/event_publisher.go` | Thin wrappers OK, but callers publish inside DB tx (MS-5) | Add outbox in **task**, then producer |
| Event consumer | `internal/task/transport/event_consumer.go` | Ack/Nack policy decent; no idempotency/state guard (MS-7) | Fix task consumer + status transitions |
| Download consumer | `internal/download/transport/event_consumer.go` | Acks before work finishes (MS-6) | Refactor to Option A (ack after done) |
| gRPC transport | `internal/auth/transport/grpc.go` | Usable encode path; dead Update RPCs in task | Strip unused task RPCs; move encode out of `internal/errors` |
| Full HTTP transport | `internal/apigateway/transport.go` | Domain mapping improved; decode errors → 500; `/download` plain text | Clean when building `/api/v2` |
| Composition root | `cmd/*/main.go` | No deadlines, no OTel, shared secrets defaults | Harden after k8s scaffolding |

## How to use
1. Open the exemplar before writing.
2. Match its structure (ports, error style, test shape).
3. If the closest file is in "No exemplar", stop and propose a small golden-path PR instead of spreading the anti-pattern.
