# task

Status: **legacy**. Copy patterns from auth/download, not from here. Catalog: [service-catalog](../architecture/service-catalog.md#task-legacy).

## Responsibility
Source of truth for download tasks: CRUD, lifecycle state, download-URL issuing. Does NOT download bytes.

## Owned data
- MySQL `tasks`: [internal/task/mysql/sqlc/schema.sql](../../internal/task/mysql/sqlc/schema.sql) (same DB as auth).
- Redis: download tokens (key = HMAC-SHA256 of token with `TOKEN_HMAC_SECRET`). See [internal/task/tokenstore.go](../../internal/task/tokenstore.go).
- MinIO bucket `task-sources` (`MINIO_TASK_SOURCES_BUCKET`): uploaded `.torrent` payloads, 24h expiry.

## Communication
- In: gRPC `task.v1.TaskService` on `GRPC_ADDRESS` (`:8082`). Contract: [api/task.proto](../../api/task.proto).
- Publishes: `task.created`, `task.paused`, `task.resumed`, `task.cancelled`, `task.status.updated` (nobody consumes it). See [internal/task/event_publisher.go](../../internal/task/event_publisher.go).
- Consumes (group `task-service-group`): `task.progress.updated`, `task.completed`, `task.failed`. See [internal/task/transport/event_consumer.go](../../internal/task/transport/event_consumer.go).

## State machine
`PENDING → (DOWNLOADING → PAUSED ⇄ DOWNLOADING) → COMPLETED | FAILED | CANCELLED`. `FAILED → PENDING` via `RetryTask`.
In practice `DOWNLOADING` is never set from download events. See [data-flows #3](../architecture/data-flows.md#3-create-task--download--complete).

## Folder structure
| Path | Role | Status |
|---|---|---|
| [internal/task/service.go](../../internal/task/service.go) | Use cases, ports, in-mem token store, URL logic (860 lines) | legacy |
| `internal/task/param_output.go` | Domain types and statuses | mixed |
| `internal/task/event_publisher.go` | Event publisher adapter | mixed |
| `internal/task/endpoint` | go-kit endpoints + pb mapping (847 lines) | legacy: mapping belongs in transport |
| `internal/task/transport/grpc.go` | gRPC server/client | mixed |
| `internal/task/transport/event_consumer.go` | Kafka consumer | mixed: ack policy good, no idempotency |
| `internal/task/mysql` | Repo + tx manager | legacy: `Update` drops fields, `Begin()` without ctx |
| `internal/task/sqlite` | Pocket repo | mixed |
| `internal/task/mysql/sqlc`, `internal/task/pb` | Generated | do not edit |

## Gotchas
- `CreateTask`/`Pause`/`Resume`/`Cancel`/`Retry` publish to Kafka inside the DB tx. Publish success + commit failure leaves a phantom event.
- `GetTask`/`DeleteTask` map any repo error to NOT_FOUND.
- `UpdateTaskChecksum`/`UpdateTaskMetadata` are no-ops on MySQL (`taskRepo.Update` ignores those fields).
- One shared `rate.NewLimiter(1, 100)` wraps every gRPC endpoint.
- `Update*`/`CompleteTask` gRPC RPCs are unused; download reports via Kafka only.
- `cmd/task/main.go` sets `WithPresigner(nil)`, so URLs always use the token path.
- `TOKEN_HMAC_SECRET` falls back to `default-secret-change-me` when empty.

## Config (env)
`MYSQL_*`, `REDIS_*`, `KAFKA_BROKERS`, `KAFKA_VERSION`, `KAFKA_MAX_RETRY`, `GRPC_ADDRESS`, `TOKEN_HMAC_SECRET`, `MINIO_*` (incl. `MINIO_PRESIGN_*`, `MINIO_TASK_SOURCES_BUCKET`), `LOG_LEVEL`.

## Commands
- Build: `go build ./cmd/task`
- Test: `go test ./internal/task/...`
- Regenerate sqlc/proto: `make generate`
