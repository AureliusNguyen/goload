# ADR-0003: Transactional outbox for domain events

- Status: Accepted (target; not yet implemented)
- Date: 2026-11-15
- Deciders: project

## Context
Task service publishes Kafka messages inside or beside DB transactions. That is not atomic: commit without publish or publish without commit both occur under failure.

## Decision
Persist events in an `outbox` table in the same DB transaction as the domain write. A relay process publishes to Kafka and marks rows sent ([MS-5](../standards/microservices.md)). First implementation target: **task** service.

## Consequences
- Slightly more schema and a relay (poller or CDC).
- Removes dual-write bugs for create/pause/cancel/delete.
- Download and auth adopt the same pattern when they gain durable events.

## Alternatives considered
- Dual-write and hope — current; rejected for production correctness.
- Listen to MySQL binlog (Debezium) — deferred until scale justifies it.
