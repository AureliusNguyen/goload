# goload

Download manager: HTTP gateway, auth and task over gRPC, downloads over Kafka. Pocket (`cmd/pocket`, `cmd/pocketsrv`) is the same cores with local adapters. `public/` is out of scope unless asked.

## How to work
1. Start at [docs/architecture/service-catalog.md](docs/architecture/service-catalog.md). Open only the matching file under [docs/services/](docs/services/).
2. Do not read the whole repo. Read the entry point, the use case, and the adapter you will change.
3. Before writing code, read [docs/ai/exemplars.md](docs/ai/exemplars.md) and [docs/ai/legacy-map.md](docs/ai/legacy-map.md). Copy exemplars. Do not spread a pattern listed as legacy.
4. [docs/standards/](docs/standards/) beat existing code. On conflict, follow the standard and report it.
5. Ask before architectural changes: new service, new datastore, new sync hop, Ack policy, or outbox.
6. Do not refactor opportunistically. Finish the task with the smallest safe edit, then propose a separate refactor.
7. At the end of every task, follow the Retro in [docs/ai/lessons.md](docs/ai/lessons.md).

## Hard rules
- No service imports another service's `internal/<svc>` (CC-3). Contracts live in `api/` and `internal/events`.
- Core does not import gRPC, go-kit, `database/sql`, sarama, redis, minio, or `net/http` (CC-2).
- One owner per table, bucket, Redis prefix, and topic (MS-1). Database per service outside local compose ([ADR-0001](docs/adr/0001-database-per-service.md), [ADR-0009](docs/adr/0009-local-shared-mysql.md)).
- Do not `Publish` inside a DB transaction (MS-5). Long-running consumers ack only after the work finishes (MS-6, [ADR-0004](docs/adr/0004-consumer-ack-after-completion.md)).
- Client-facing errors are `*errors.Error`. Never return Cause, storage keys, SQL, or paths (ERR-6, API-10).
- `/api/v1` is frozen. New HTTP work is `/api/v2` with the same RPC-style paths (API-2, API-3, [ADR-0008](docs/adr/0008-api-v1-deprecate-v2-problem-json.md)).
- Do not hand-edit generated code: `internal/*/pb`, `internal/*/mysql/sqlc`, `internal/apigateway/gen`.

## Commands
- `make generate` — protoc + sqlc
- `make build` — `go build ./...`
- `make compose-up` / `make compose-down` — local stack
- `make lint` — golangci-lint. Gate on errcheck, govet, staticcheck, gosec, errorlint, bodyclose; other linters are advisory (CC-16).
- `go test ./internal/<svc>/...` — one service
- `go test -race ./...` — race; some tests need Docker
- `make smoke` — waits for gateway `/health` only

## Map
| Need | Where |
|---|---|
| Shape | [overview](docs/architecture/overview.md) |
| Services | [catalog](docs/architecture/service-catalog.md), [services/](docs/services/) |
| Flows | [data-flows](docs/architecture/data-flows.md) |
| Rules | [standards/](docs/standards/) |
| Copy from | [exemplars](docs/ai/exemplars.md) |
| Do not copy | [legacy-map](docs/ai/legacy-map.md) |
| Decisions | [adr/](docs/adr/) |
| Background | [research](docs/research.md) |
| Checklists | [.claude/skills/](.claude/skills/) |
| Diff review | [.claude/agents/reviewer.md](.claude/agents/reviewer.md) |
| Enforcement ideas | [enforcement](docs/ai/enforcement.md) |

When adding an endpoint, event, or service, debugging across services, or touching legacy code, follow the matching skill under `.claude/skills/`.
