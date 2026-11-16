# apigateway

Status: **mixed**. Catalog entry: [service-catalog](../architecture/service-catalog.md#apigateway-mixed).

## Responsibility
Public HTTP edge. Validates Bearer tokens (via auth), checks task ownership (via task), maps domain/gRPC errors to HTTP, streams files for `/download?token=`. No business rules, no owned data.

## Communication
- In: HTTP `:8080` (behind nginx `:8081` in compose). Contract: [api/openapi.yaml](../../api/openapi.yaml), generated DTOs in `internal/apigateway/gen`.
- Out: gRPC → auth (`AUTH_SERVICE_GRPC_ADDRESS`), gRPC → task (`TASK_SERVICE_GRPC_ADDRESS`). Insecure, no deadlines.
- Direct datastore reads (coupling): Redis download tokens via `task.NewTokenStore`, MinIO objects via `storage.NewMinioBackend`.

## Folder structure
| Path | Role | Status |
|---|---|---|
| [cmd/apigateway/main.go](../../cmd/apigateway/main.go) | Composition root, HTTP server, middleware chain | mixed: no server timeouts |
| [internal/apigateway/transport.go](../../internal/apigateway/transport.go) | Routes, decoders, `encodeError`, `/download` | mixed: decoders return non-domain errors |
| [internal/apigateway/endpoint.go](../../internal/apigateway/endpoint.go) | go-kit endpoints, DTO mapping | mixed: depends on `task`/`auth` domain types |
| [internal/apigateway/middleware.go](../../internal/apigateway/middleware.go) | Auth middleware (`NewAuthMiddleware`, `NewNoAuthMiddleware`) | standard candidate |
| [internal/apigateway/owner_middleware.go](../../internal/apigateway/owner_middleware.go) | Ownership check | standard candidate |
| `internal/apigateway/gen` | oapi-codegen models | generated, do not edit |

HTTP middleware order (outer → inner): RequestID → Logging → Recovery → CORS → mux → `addTokenToContext` → endpoint auth MW → owner MW.

## Error contract
Body: `{"error": "<safe message>", "request_id": "<id>"}`. `>=500` always says `internal server error`. Mapping: `httpStatusFromCode` (domain codes) then `httpStatusFromGRPC`.

## Gotchas
- Every protected request = `VerifySession` + `GetTask` + the real call (3 gRPC hops).
- Decode errors (`http.ErrMissingFile`, `strconv` errors, bad JSON) are not domain errors and return **500**, not 400.
- `/download` uses `http.Error` (plain text), unlike the JSON error contract.
- `ErrLimited` from auth/task rate limiters surfaces as 500.
- `/docs` serves the embedded spec from [docs/openapi.yaml](../openapi.yaml) (bundled by `make docs`). `docs/docs.go` embeds `*.html *.yaml` only.

## Config (env)
`HTTP_ADDRESS`, `TOKEN_HMAC_SECRET` (must match task), `REDIS_*`, `AUTH_SERVICE_GRPC_ADDRESS`, `TASK_SERVICE_GRPC_ADDRESS`, `MINIO_*`, `CORS_*`, `LOG_LEVEL`.

## Commands
- Build: `go build ./cmd/apigateway`
- Test: `go test ./internal/apigateway/...`
- Regenerate DTOs: `make generate` (oapi-codegen via [scripts/generate.sh](../../scripts/generate.sh))
