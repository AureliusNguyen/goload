# Data flows

Known breaks are marked **BROKEN** or **RISK** with the root cause. Do not "fix" them inside unrelated tasks. See [legacy-map](../ai/legacy-map.md) (Phase 4).

## 1. Register and log in
1. `POST /api/v1/auth/create` → gateway → `auth.CreateAccount` (gRPC).
2. auth hashes the password, then inserts `accounts` + `account_passwords` in one tx. A duplicate name hits the unique key and returns `ALREADY_EXISTS` (409).
3. `POST /api/v1/auth/session` → `auth.CreateSession` → bcrypt verify → RS512 JWT (kid = row in `token_public_keys`).
- Code: [internal/auth/service.go](../../internal/auth/service.go)
- RISK: login returns 404 for an unknown account and 401 for a wrong password, which enables account enumeration.

## 2. Authenticated task request
1. `addTokenToContext` extracts the Bearer token. A missing token returns 401 JSON.
2. `NewAuthMiddleware` → `auth.VerifySession` (gRPC) → account ID in context.
3. `RequireTaskOwnerMiddleware` → `task.GetTask` (gRPC) → 403 if the owner differs.
4. The real task RPC runs (a third gRPC call).
- Code: [internal/apigateway/endpoint.go](../../internal/apigateway/endpoint.go), [owner_middleware.go](../../internal/apigateway/owner_middleware.go)
- RISK: auth and task gRPC servers apply a 1 req/s (burst 100) limiter to every endpoint; once exhausted, requests fail with HTTP 500 (not 429).

## 3. Create task → download → complete
1. `POST /api/v1/tasks/create` → `task.CreateTask`: insert row `PENDING`, then publish `task.created` **inside the DB tx**.
2. download consumes `task.created` → `ExecuteTask` claims a semaphore slot (default 5), starts a goroutine, **acks immediately**.
3. Goroutine: `GetFileInfo` → `Download`. Only connection start is retried (`MaxRetries`, default 3, `2^n`s + jitter); a mid-stream failure fails the task → publish `task.progress.updated` every ~3s → `Store` to MinIO → publish `task.completed` (storage key, checksum, size).
4. task consumes `task.completed` → `COMPLETED` + storage info; `task.failed` → `FAILED` + message.
- Code: [internal/task/service.go](../../internal/task/service.go), [internal/download/service.go](../../internal/download/service.go), [internal/task/transport/event_consumer.go](../../internal/task/transport/event_consumer.go)
- RISK: publish-then-commit with no outbox. A commit failure after a successful publish leaves a phantom download.
- RISK: ack-before-completion. A crash mid-download loses the task, which stays `PENDING` forever.
- RISK: consumers have no state guard. A redelivered `task.failed` can overwrite `COMPLETED`.
- BROKEN: download publishes `task.status.updated` (DOWNLOADING/STORING) but task does not consume it. Status jumps `PENDING → COMPLETED`.
- BROKEN: checksum is published but not persisted (`mysql.taskRepo.Update` ignores it).

## 4. Pause / resume / cancel
1. Gateway → `task.PauseTask` → guard `status == DOWNLOADING` → set `PAUSED` + publish `task.paused` in tx.
2. download consumes → `PausableProgressReader.Pause()` on the in-memory execution. Resume and cancel work the same way (cancel = context cancel).
- BROKEN: tasks never reach `DOWNLOADING` (see #3), so pause returns `INVALID_INPUT` (400; semantically should be `INVALID_STATE`) and resume is unreachable. `TODO: confirm` end-to-end.
- RISK: control state is in-memory per download replica. With >1 replica or after a restart, the event is acked as NOT_FOUND and ignored.
- `DeleteTask` publishes `task.cancelled` and deletes the row even if the publish fails, which can orphan a running download.

## 5. Fetch a completed file
1. `POST /api/v1/tasks/download-url` → `task.GenerateDownloadURL`.
   - Presigner set and `one_time=false` → presigned MinIO URL (`direct=true`).
   - Otherwise → UUID token stored in Redis (HMAC key, TTL) → `/download?token=…`.
2. `GET /download?token=` → gateway consumes the token from Redis → streams from MinIO (supports `Range`).
- Code: [internal/task/service.go](../../internal/task/service.go) `GenerateDownloadURL`, [internal/apigateway/transport.go](../../internal/apigateway/transport.go) `NewHTTPHandlerWithDownload`
- Note: `cmd/task/main.go` passes `WithPresigner(nil)`, so microservice mode always uses the token path. `TODO: confirm intent`.
- Coupling: gateway reads task-owned Redis keys and download-owned MinIO objects directly.
