# Clean code & architecture

Standards beat existing code. If code conflicts with a rule, follow the rule and report the conflict (see [legacy-map](../ai/legacy-map.md)).

## Service layout (hexagonal)
```
cmd/<svc>/main.go         composition root: config, wiring, servers. No business logic.
internal/<svc>/           core: domain types, ports (interfaces), use cases (Service)
internal/<svc>/endpoint/  go-kit endpoints, request/response types
internal/<svc>/transport/ driving adapters: gRPC/HTTP servers+clients, event consumers, pb mapping
internal/<svc>/<tech>/    driven adapters: mysql/, sqlite/, cache/, messaging/, storage adapters
internal/<svc>/{pb,mysql/sqlc}, internal/apigateway/gen   generated: never hand-edit
```

## Dependencies
- **CC-1** Direction: `transport → endpoint → core ← adapters`. Core never imports `transport`, `endpoint`, or adapter packages.
- **CC-2** Core imports only stdlib, `internal/errors`, `internal/events` (contract types), and framework-free `pkg/*`. Never `google.golang.org/grpc`, `go-kit`, `database/sql`, `sarama`, `go-redis`, `minio-go`, `net/http`.
- **CC-3** No service imports another service's `internal/<svc>` package. Cross-service contracts live only in `api/*.proto`, `api/openapi.yaml`, `internal/events`.
- **CC-4** `pkg/*` never imports `internal/*`.

## Types and interfaces
- **CC-5** Domain structs carry no `json`/`sql`/`protobuf` tags; map in adapters. Event and DTO structs may.
- **CC-6** Ports are declared in the consuming package, ≤ 7 methods; split fat interfaces by role (e.g. reader/writer, client-facing vs worker-facing).
- **CC-7** Constructors take interfaces, return concrete types; optional deps via `WithX` functional options.
- **CC-8** Zero values are safe, or the constructor is the only way to build the type.

## Functions and files
- **CC-9** `ctx context.Context` is the first parameter of every function that does I/O or may block. Never store ctx in structs. `context.Background()` only in `main` and tests.
- **CC-10** Guideline, not a gate: aim for functions ≤ 60 lines and files ≤ 500 lines (generated code exempt). Split when a function does more than one job.
- **CC-11** No package-level mutable state, no `init()`.
- **CC-12** Every goroutine has an owner, a stop signal (ctx), and is awaited on shutdown (`sync.WaitGroup`/`errgroup`).

## Naming and style
- **CC-13** Go naming: MixedCaps (no `ALL_CAPS`), initialisms upper (`ID`, `URL`), no stutter (`task.TaskService` → `task.Service`).
- **CC-14** Comments state *why* or a constraint; exported identifiers have doc comments.
- **CC-15** Delete dead code: unused RPCs, config fields, types, topics.
- **CC-16** `gofmt`/`goimports` formatting is required. Lint findings that block: `errcheck`, `govet`, `staticcheck`, `gosec`, `errorlint`, `bodyclose`. All other `.golangci.yml` linters are advisory (the config is a copied template, not a project decision).

## Deployment modes
- **CC-17** `cmd/pocket*` contain wiring only. Behavior differences between modes live in adapters, never in core.
- **CC-18** One schema source of truth per store; pocket SQLite DDL is generated or kept beside MySQL migrations, not inlined in `main.go`.
