# ADR-0005: go-kit and hexagonal layout

- Status: Accepted
- Date: 2026-11-15
- Deciders: project

## Context
Services need HTTP/gRPC/event adapters, middleware (logging, metrics, circuit breaking), and testable cores. The repo already uses go-kit endpoints and a hexagonal `internal/<svc>` layout.

## Decision
Keep **go-kit** for endpoints, transport adapters, and service discovery/load-balancing helpers. Keep **hexagonal** package layout: core owns ports; `endpoint/`, `transport/`, and tech adapters depend inward ([CC-1](../standards/clean-code.md)–[CC-3](../standards/clean-code.md)). Logging stays on **go-kit/log** for consistency with existing code; structured fields must still satisfy [OBS-1](../standards/observability.md)–[OBS-5](../standards/observability.md).

## Consequences
- Boilerplate in endpoint/transport packages is expected.
- Avoid importing go-kit into core.
- Future slog/OTel bridges wrap kit logger rather than a big-bang rewrite.

## Alternatives considered
- Replace go-kit with plain net/http + grpc-go only — more custom middleware; deferred.
- Switch logging to slog-only now — rejected until observability stack lands; kit log is fine.
