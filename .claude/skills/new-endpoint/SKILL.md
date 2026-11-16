---
name: new-endpoint
description: Add an HTTP or gRPC endpoint to an existing goload service. Use when adding a route, RPC, handler, or OpenAPI/proto operation.
---

# New endpoint

1. Read [docs/services/<svc>.md](../../../docs/services/) and the row in [service-catalog](../../../docs/architecture/service-catalog.md). Confirm this service owns the data.
2. Check [legacy-map](../../../docs/ai/legacy-map.md). If the transport file is listed, do not copy its error or layering mistakes.
3. Contract first ([API-1](../../../docs/standards/api-design.md), [API-11](../../../docs/standards/api-design.md)): edit `api/openapi.yaml` or `api/<svc>.proto`, then `make generate`. Do not hand-edit `internal/*/pb` or `internal/apigateway/gen`.
4. HTTP: under `/api/v2`, same RPC-style paths as v1 (`/tasks/create`, `/tasks/get?id=`). Do not add `/api/v1` routes ([API-2](../../../docs/standards/api-design.md), [API-3](../../../docs/standards/api-design.md)).
5. gRPC: one `FooRequest` / `FooResponse`, no field renumbering, `buf` breaking check ([API-12](../../../docs/standards/api-design.md)–[API-15](../../../docs/standards/api-design.md)). Skip RPCs with no caller ([API-16](../../../docs/standards/api-design.md)).
6. Use case in `internal/<svc>` with a port. Core returns `*errors.Error`. No gRPC, go-kit, SQL, or HTTP imports in core ([CC-1](../../../docs/standards/clean-code.md), [CC-2](../../../docs/standards/clean-code.md), [ERR-6](../../../docs/standards/error-handling.md)).
7. go-kit endpoint stays thin. Map pb/JSON in `transport/`. Encode with the shared error mapper ([ERR-8](../../../docs/standards/error-handling.md)–[ERR-10](../../../docs/standards/error-handling.md)). Copy [internal/auth/service.go](../../../internal/auth/service.go) and [transport_error_test.go](../../../internal/apigateway/transport_error_test.go).
8. Outbound gRPC calls take a deadline from ctx ([MS-12](../../../docs/standards/microservices.md)). Do not add a global in-process rate limiter ([MS-16](../../../docs/standards/microservices.md)).
9. Tests: table-driven, assert code and status, not only `err != nil` ([TEST-1](../../../docs/standards/testing.md)–[TEST-3](../../../docs/standards/testing.md)).
10. Retro: [docs/ai/lessons.md](../../../docs/ai/lessons.md).
