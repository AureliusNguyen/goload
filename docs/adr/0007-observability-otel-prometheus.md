# ADR-0007: Observability with OpenTelemetry and Prometheus

- Status: Accepted (target; not yet implemented)
- Date: 2026-11-15
- Deciders: project

## Context
Today the stack is log-centric. Production debugging of multi-service download flows needs traces, RED metrics, and correlation IDs across HTTP, gRPC, and Kafka.

## Decision
- **Traces + metrics instrumentation:** OpenTelemetry SDK; export OTLP to a collector.
- **Metrics scrape:** Prometheus exposition (`/metrics`) and/or OTLP metrics — Prometheus for k8s scrape is the default ops path.
- **Logs:** keep go-kit/log; include `request_id` / `trace_id` ([OBS-8](../standards/observability.md)).
- Follow [docs/standards/observability.md](../standards/observability.md).

## Consequences
- Composition roots gain tracer/meter providers and graceful flush.
- Exemplars and CI should eventually assert key spans/metrics exist for critical paths.

## Alternatives considered
- Vendor agent-only (Datadog native) — deferred; OTel keeps optionality.
- Metrics-only without traces — insufficient for Kafka + gRPC chains.
