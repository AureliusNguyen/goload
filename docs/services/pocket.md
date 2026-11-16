# pocket / pocketsrv (deployment modes)

Not services. Single-process composition roots that wire `internal/*` with local adapters. Business logic must stay in `internal/*`; nothing here may diverge from microservice behavior except adapters.

| | [cmd/pocket](../../cmd/pocket/main.go) | [cmd/pocketsrv](../../cmd/pocketsrv/main.go) |
|---|---|---|
| Auth | `NewNoAuthMiddleware` with fixed `pocket` account | Real auth service, JWT RS512, in-memory public-key store |
| Task/auth persistence | SQLite (`POCKET_DB_PATH`) | SQLite |
| Events | `pkg/message/inmem` broker | same |
| File storage | `storage.Local` under `POCKET_DATA_DIR` (24h expiry, sidecar `.meta.json`) | same |
| Download tokens | `task.NewInmemTokenStore` (raw token keys, TTL not enforced on write) | `task.NewTokenStore` (HMAC keys) over in-memory cache |
| Extra route | `POST /api/v1/pocket/tasks/reveal?id=` (opens OS file manager) | none |
| Frontend | static files from `POCKET_WEB_DIR` | same |

## Gotchas
- SQLite schema is created by `runMigrations` inside each `main.go`, duplicated between the two binaries and separate from [migrations/mysql](../../migrations/mysql/). Keep both in sync with MySQL schema changes.
- Local presigned URLs are `file://`, so pocket uses `/download?token=` for browser downloads.
- `reveal` runs OS commands (`explorer.exe`, `open`, `xdg-open`). Paths inside Docker won't match the host.
- In-memory broker redelivers on Nack, but nothing survives a restart.

## Config (env)
`HTTP_ADDRESS`, `POCKET_DB_PATH`, `POCKET_DATA_DIR`, `POCKET_WEB_DIR`, `CORS_*`, `LOG_LEVEL`; pocketsrv adds `TOKEN_HMAC_SECRET`, `AUTH_TOKEN_RSA_BITS`, `AUTH_TOKEN_EXPIRES_IN`, `AUTH_HASH_BCRYPT_COST`.

## Commands
- Run: `go run ./cmd/pocket` or `make run-pocket` (Docker)
- Release zip: `make build-pocket`
