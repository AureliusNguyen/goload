# Microservices

## Boundaries and data
- **MS-1** Each table, bucket, Redis key prefix, and topic has exactly one owning service ([service-catalog](../architecture/service-catalog.md)). Others access it only through the owner's API or events.
- **MS-2** Database per service: separate schema and credentials (`goload_auth`, `goload_task`); migrations in `migrations/<svc>/`.
- **MS-3** No distributed transactions. Multi-service workflows are choreographed sagas; every step that can fail has a documented compensation.
- **MS-4** Services are stateless. Any in-memory state needed after a restart (e.g. active downloads) is reconstructible from durable state.

## Messaging (Kafka via `pkg/message`)
- **MS-5** Transactional outbox: write the state change and an `outbox` row in the same DB tx; a relay publishes and marks rows sent. Never call `Publish` inside or right after a DB tx.
- **MS-6** At-least-once: ack only after the handler's effects are durable. Long work: the worker goroutine owns the message and acks/nacks on finish; concurrency bounded by a semaphore; in-flight work drained on shutdown.
- **MS-7** Idempotent consumers: redelivery is a no-op. Use a state guard (allowed-from states per transition) or a `processed_messages(message_id)` table, checked in the same tx as the effect.
- **MS-8** Envelope: `event_id`, `event_type`, `version`, `occurred_at`, `correlation_id`, `payload`. Topic names `task.<past-tense-verb>`. Changes are additive; breaking change = new version.
- **MS-9** Partition key = aggregate ID (`task_id`) so events per task stay ordered.
- **MS-10** Every published topic has a consumer and is declared in deployment config; otherwise delete it.
- **MS-11** Poison messages: permanent failures and messages past 5 deliveries go to `<topic>.dlq` with the error; DLQ depth is alerted.

## Synchronous calls (gRPC)
- **MS-12** Every outbound call has a deadline derived from the caller's ctx: default 2s reads, 5s writes.
- **MS-13** Retry only idempotent calls, only on `Unavailable`/`DeadlineExceeded`, max 3 attempts, exponential backoff with jitter (`go-kit/sd/lb.Retry` or gRPC retry policy).
- **MS-14** Circuit breaker per downstream client (`go-kit/circuitbreaker.Gobreaker`).
- **MS-15** ≤ 2 sync hops per user request. Pass authorization context with the call (e.g. `GetTask(id, accountID)`) instead of extra round trips.
- **MS-16** Rate limiting at the edge, per client (gateway). No global limiters inside services. Limit errors map to `TOO_MANY_REQUESTS`.

## Deployment (Kubernetes)
- **MS-17** One image per service; one k8s `Deployment` + `Service`; resource requests/limits set; ≥ 2 replicas for stateless services.
- **MS-18** Probes: `/livez` (process alive) and `/readyz` (dependencies reachable) on an admin port.
- **MS-19** Discovery via k8s Service DNS (`task.<ns>.svc.cluster.local:8082`). Use Consul (`go-kit/sd/consul`) only for deployments outside k8s.
- **MS-20** Config from env (`envconfig`); secrets from k8s `Secret`; no secret defaults in code; fail fast on missing required config.
- **MS-21** Graceful shutdown on SIGTERM: stop intake, drain in-flight (HTTP `Shutdown`, gRPC `GracefulStop`, consumer drain) within `terminationGracePeriodSeconds`.
- **MS-22** Service-to-service traffic uses TLS (mTLS via mesh or gRPC credentials) outside local dev.
