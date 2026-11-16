# Service catalog

Quality status: **standard** = follows [docs/standards](../standards/) · **mixed** = sound structure, known violations · **legacy** = do not copy; refactor before extending.
Per-service detail: [docs/services/](../services/).

## apigateway: mixed
- **Does:** public HTTP API, Bearer-token check via auth, task ownership check, CORS, request IDs, `/download?token=` streaming, embedded OpenAPI at `/docs`.
- **Does NOT:** own any data, contain business rules, or talk to Kafka.
- **Data owned:** none.
- **APIs:** REST in [api/openapi.yaml](../../api/openapi.yaml) (`/api/v1/auth/*`, `/api/v1/tasks/*`, `/download`, `/health`).
- **Events:** none.
- **Depends on:** auth (gRPC), task (gRPC), Redis (task's token keys), MinIO (download's bucket).
- **Violations:** imports `internal/task` and `internal/auth` domain packages; reads other services' datastores.

## auth: mixed (closest to standard)
- **Does:** account registration, password hashing (bcrypt), JWT RS512 issue and verify.
- **Does NOT:** authorize resource access (ownership is checked in the gateway) or manage user profiles.
- **Data owned:** MySQL `accounts`, `account_passwords`, `token_public_keys`; Redis `auth:account_name:*`, `auth:token_public_key:*`.
- **APIs:** gRPC `auth.v1.AuthService`: `CreateAccount`, `CreateSession`, `VerifySession` ([api/auth.proto](../../api/auth.proto)).
- **Events:** none.
- **Depends on:** MySQL, Redis.
- **Violations:** shared MySQL database with task; domain struct carries `sql` tags.

## task: legacy
- **Does:** task CRUD, lifecycle state machine (pause/resume/cancel/retry), download-URL generation (presigned or token).
- **Does NOT:** download bytes or write files (except uploaded `.torrent` sources).
- **Data owned:** MySQL `tasks`; Redis download tokens (HMAC-keyed); MinIO bucket `task-sources`.
- **APIs:** gRPC `task.v1.TaskService`, 18 RPCs ([api/task.proto](../../api/task.proto)). 7 `Update*`/`CompleteTask` RPCs have no callers.
- **Publishes:** `task.created`, `task.paused`, `task.resumed`, `task.cancelled`, `task.status.updated` (no consumer).
- **Consumes:** `task.progress.updated`, `task.completed`, `task.failed`.
- **Depends on:** MySQL, Redis, Kafka, MinIO.
- **Violations:** publish inside DB tx (no outbox); non-idempotent consumers with no state guard; repo errors masked as NOT_FOUND; `Update` drops checksum/metadata; never reaches `DOWNLOADING`; 860-line service file.

## download: mixed
- **Does:** executes downloads (HTTP/HTTPS with SSRF guard, FTP, BitTorrent), retries with backoff, pause/resume/cancel of in-flight work, streams to storage, MD5 checksum.
- **Does NOT:** persist task state; it reports state only via events.
- **Data owned:** MinIO bucket `goload` (downloaded files); in-memory active-task map.
- **APIs:** `GET :8083/health` only.
- **Publishes:** `task.status.updated`, `task.progress.updated`, `task.completed`, `task.failed`, `task.retried` (no topic, no consumer).
- **Consumes:** `task.created`, `task.paused`, `task.resumed`, `task.cancelled` (consumer group `download-service-group`).
- **Depends on:** Kafka, MinIO.
- **Violations:** `task.created` is acked when the download *starts*, so a crash loses work; in-flight state lives only in memory, so pause/cancel after restart are no-ops.

## Shared libraries
| Package | Role | Status |
|---|---|---|
| [internal/errors](../../internal/errors/errors.go) | Domain error codes + gRPC mapping | mixed: domain package imports gRPC |
| [internal/events](../../internal/events/event.go) | Event payload structs (the async contract) | mixed: unversioned, no schema tests |
| [internal/storage](../../internal/storage/storage.go) | Storage ports + MinIO/local adapters | standard candidate |
| [pkg/message](../../pkg/message/pubsub.go) | Pub/sub ports, Kafka + in-memory adapters (Watermill-style) | standard candidate |
| [pkg/middleware](../../pkg/middleware/) | HTTP/gRPC logging, recovery, CORS, request ID | standard candidate |
| `pkg/cache`, `pkg/crypto` | Redis/in-memory cache, bcrypt, RSA helpers | not assessed |
