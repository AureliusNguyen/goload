# download

Status: **mixed**. Catalog: [service-catalog](../architecture/service-catalog.md#download-mixed).

## Responsibility
Worker that fetches sources and writes them to storage, reporting progress and outcome as events. Does NOT persist task state.

## Owned data
- MinIO bucket `goload` (`MINIO_BUCKET`), key `{taskID}/{safeName}-{sourceHash}{ext}`, optional expiry `MINIO_FILE_EXPIRY`.
- In-memory `activeTasks` map (lost on restart, not shared across replicas).

## Communication
- Consumes (group `KAFKA_CONSUMER_GROUP`, default `download-service-group`): `task.created`, `task.paused`, `task.resumed`, `task.cancelled`.
- Publishes: `task.status.updated`, `task.progress.updated` (≥3s apart), `task.completed`, `task.failed`, `task.retried` (no topic in compose, no consumer).
- HTTP: `:8083/health` only. No gRPC.

## Folder structure
| Path | Role | Status |
|---|---|---|
| [internal/download/service.go](../../internal/download/service.go) | Execution, concurrency (semaphore, default 5), retry, pause reader | mixed: async ack issue |
| [internal/download/downloader.go](../../internal/download/downloader.go) | `Downloader` port | standard candidate |
| `internal/download/downloader/` | HTTP (SSRF guard, chunked ranges), FTP, BitTorrent adapters | standard candidate |
| `internal/download/event_publisher.go` | Event publisher adapter | mixed |
| [internal/download/transport/event_consumer.go](../../internal/download/transport/event_consumer.go) | Kafka consumer, ack/nack policy | mixed |
| [cmd/download/main.go](../../cmd/download/main.go) | Composition root, health server | mixed |

## Gotchas
- `ExecuteTask` returns once the download goroutine *starts*, and the consumer then acks `task.created`. A crash loses in-flight work. The semaphore blocks the consumer when full, giving backpressure.
- Permanent errors (`INVALID_INPUT`, `CONFLICT`) are acked; others are nacked (redelivered).
- Pause/resume/cancel for unknown task IDs return NOT_FOUND and are acked (no-op on other replicas).
- Only download *start* is retried; mid-stream failures fail the task.
- HTTP downloader refuses private/loopback/link-local targets unless `WithAllowPrivateAddresses` is set.
- Task timeout 30m (`WithTaskTimeout`); progress throttled by `DOWNLOAD_PROGRESS_UPDATE_INTERVAL`.
- Integration tests use testcontainers Kafka and take ~2 minutes; use `-short` to skip.

## Config (env)
`KAFKA_BROKERS`, `KAFKA_VERSION`, `KAFKA_CONSUMER_GROUP`, `MINIO_*`, `MINIO_FILE_EXPIRY`, `LOG_LEVEL`.

## Commands
- Build: `go build ./cmd/download`
- Unit tests: `go test -short ./internal/download/...`
- Integration: `go test ./internal/download/transport/...` (needs Docker)
