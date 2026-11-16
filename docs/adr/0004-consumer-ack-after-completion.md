# ADR-0004: Consumer Ack after completion (Option A)

- Status: Accepted
- Date: 2026-11-15
- Deciders: project

## Context
Download consumer previously Ack'd messages when work *started*, then ran `ExecuteTask` in a detached goroutine. Offsets advanced while downloads were still running, so pause/cancel and redelivery semantics were wrong under crash or lag.

## Decision
For long-running download (and similar) consumers: process the message to completion (success or terminal failure handling), then Ack. Nack (or equivalent) on retryable failure. The processing goroutine owns the message until then; shutdown drains in-flight work ([MS-6](../standards/microservices.md)).

## Consequences
- Consumer lag reflects real work, not just accept rate.
- Need bounded concurrency and careful shutdown timeouts.
- At-least-once still applies; combine with idempotent handlers ([MS-7](../standards/microservices.md)).

## Alternatives considered
- Option B: Ack on start + durable in-memory/DB work ledger — more moving parts; deferred.
- Sync processing in the subscribe loop without goroutines — simpler but hurts throughput; allowed for short handlers (task completion updates).
