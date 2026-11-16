---
name: debug-cross-service
description: Debug a goload request that crosses the gateway, auth, task, download, Kafka, or storage. Use for failed downloads, stuck tasks, pause/resume, 500s, or missing events.
---

# Debug across services

1. Pick the flow in [data-flows](../../../docs/architecture/data-flows.md). Name the hop that failed before reading unrelated packages.
2. Correlation: `X-Request-ID` exists at the HTTP edge ([pkg/middleware/requestid.go](../../../pkg/middleware/requestid.go)). It is not yet on gRPC or Kafka ([legacy-map](../../../docs/ai/legacy-map.md)). Today, join logs on `task_id` and timestamps.
3. Auth failures: unknown account is NOT_FOUND and bad password is INVALID_PASSWORD. That split is a known bug, not two different features ([ERR-12](../../../docs/standards/error-handling.md)).
4. HTTP 500 on a bad body or missing `id` is a known decoder bug ([ERR-10](../../../docs/standards/error-handling.md)). Check the gateway encoder before blaming the downstream service.
5. Task stuck in `PENDING`, or pause/resume rejected: `task.status.updated` has no consumer, so status never becomes `DOWNLOADING` ([legacy-map](../../../docs/ai/legacy-map.md)).
6. Download "succeeded" in Kafka but the file is missing: the download consumer acks when work starts ([MS-6](../../../docs/standards/microservices.md)). Check the consumer lag and the process logs, not only the offset.
7. Event published and the DB row missing, or the reverse: publish-inside-transaction ([MS-5](../../../docs/standards/microservices.md)). Compare outbox-less `Publish` call sites in `internal/task/service.go`.
8. Status flipped from `COMPLETED` to `FAILED`: no transition guard ([MS-7](../../../docs/standards/microservices.md)).
9. Rate-limit symptoms: auth and task use a 1 req/s limiter and map it to 500, not 429 ([MS-16](../../../docs/standards/microservices.md)).
10. Report the hop, the rule it breaks, and whether a code change is a fix or a legacy-map follow-up. Retro: [docs/ai/lessons.md](../../../docs/ai/lessons.md).
