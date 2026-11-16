# ADR-0002: Async download pipeline via Kafka

- Status: Accepted
- Date: 2026-11-15
- Deciders: project

## Context
Downloads are long-running and must not block the API or task service request path. Pause/resume/cancel need to interrupt in-flight work.

## Decision
Task lifecycle mutations that start or control downloads publish Kafka events. The download service consumes them and performs I/O against storage. Clients observe progress via task APIs (and later optional push), not by holding an HTTP connection for the whole download.

## Consequences
- At-least-once delivery; consumers must be idempotent ([MS-7](../standards/microservices.md)).
- Requires outbox ([ADR-0003](0003-transactional-outbox.md)) and correct Ack policy ([ADR-0004](0004-consumer-ack-after-completion.md)).
- Operational dependency on Kafka (or Redpanda).

## Alternatives considered
- Synchronous download in apigateway — rejected; timeouts and scaling.
- Direct gRPC from task to download for start — rejected for long work; keep sync only for short queries if ever needed.
