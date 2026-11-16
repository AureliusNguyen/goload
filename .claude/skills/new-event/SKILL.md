---
name: new-event
description: Add a Kafka event producer and consumer in goload. Use when publishing a topic, consuming a topic, or changing task/download event payloads.
---

# New event

1. Read [data-flows](../../../docs/architecture/data-flows.md) and the service docs for publisher and consumer. One owner per topic ([MS-1](../../../docs/standards/microservices.md), [MS-10](../../../docs/standards/microservices.md)).
2. Payload type in `internal/events` only. Envelope: `event_id`, `event_type`, `version`, `occurred_at`, `correlation_id`, `payload`. Topic name `task.<past-tense-verb>`. Partition key is the task id ([MS-8](../../../docs/standards/microservices.md), [MS-9](../../../docs/standards/microservices.md)).
3. Do not call `Publish` inside a DB transaction. Target is an outbox row in the same tx ([MS-5](../../../docs/standards/microservices.md), [ADR-0003](../../../docs/adr/0003-transactional-outbox.md)). Until the outbox exists, say so in the task report. Do not add another in-tx publish.
4. Consumer: ack only after the effect is durable. Long work (downloads) uses Option A: the goroutine owns the message until done ([MS-6](../../../docs/standards/microservices.md), [ADR-0004](../../../docs/adr/0004-consumer-ack-after-completion.md)).
5. Idempotent handler: allowed-from states or a processed-id check in the same tx as the write ([MS-7](../../../docs/standards/microservices.md)).
6. Permanent decode/`INVALID_INPUT` → DLQ then ack. `NOT_FOUND` for a deleted task → ack. Everything else → nack ([ERR-15](../../../docs/standards/error-handling.md)).
7. Declare the topic in deployment config. If nothing consumes it, do not publish it.
8. Tests: redelivery is a no-op; ack/nack matches the error class ([TEST-8](../../../docs/standards/testing.md)). Add a golden JSON fixture under `internal/events/testdata/` ([TEST-11](../../../docs/standards/testing.md)).
9. Copy ports from [pkg/message/pubsub.go](../../../pkg/message/pubsub.go). Do not copy [download event_consumer](../../../internal/download/transport/event_consumer.go) until it matches Option A ([exemplars](../../../docs/ai/exemplars.md)).
10. Retro: [docs/ai/lessons.md](../../../docs/ai/lessons.md).
