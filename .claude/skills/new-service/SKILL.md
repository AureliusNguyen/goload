---
name: new-service
description: Scaffold a new goload microservice from the hexagonal layout. Use when adding a service, cmd entrypoint, or bounded context. Ask the user before starting.
---

# New service

Ask before creating a service. Prefer extending the owner of the data.

1. Read [clean-code](../../../docs/standards/clean-code.md) layout and [microservices](../../../docs/standards/microservices.md) MS-1–MS-4. No shared tables with auth or task.
2. There is no golden-path service yet ([exemplars](../../../docs/ai/exemplars.md)). Scaffold the layout; do not clone `internal/task`.
3. Layout:
   - `cmd/<svc>/main.go` — config, wiring, servers only
   - `internal/<svc>/` — types, ports, use cases
   - `internal/<svc>/endpoint/` — go-kit
   - `internal/<svc>/transport/` — gRPC or HTTP and consumers
   - `internal/<svc>/<tech>/` — mysql, cache, messaging
4. Contracts: `api/<svc>.proto` and/or OpenAPI. Events in `internal/events`. No imports of another service's `internal/` ([CC-3](../../../docs/standards/clean-code.md)).
5. Own database `goload_<svc>` and `migrations/<svc>/`. Local compose may share a MySQL server, not a schema ([ADR-0001](../../../docs/adr/0001-database-per-service.md), [ADR-0009](../../../docs/adr/0009-local-shared-mysql.md)).
6. Logging: go-kit/log with `service`, `request_id` ([ADR-0005](../../../docs/adr/0005-go-kit-and-hexagonal.md)). Health: `/health` now; `/livez` and `/readyz` when adding k8s ([MS-18](../../../docs/standards/microservices.md)).
7. gRPC server: panic recovery from [pkg/middleware/recovery.go](../../../pkg/middleware/recovery.go), deadlines on clients ([MS-12](../../../docs/standards/microservices.md)).
8. Docs: add a catalog row, `docs/services/<svc>.md`, and `internal/<svc>/CLAUDE.md` pointing at it. Update [overview](../../../docs/architecture/overview.md) only for a real new hop.
9. Tests: one use-case test with a fake port before any adapter ([TEST-1](../../../docs/standards/testing.md)).
10. Retro: [docs/ai/lessons.md](../../../docs/ai/lessons.md).
