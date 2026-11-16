# Observability

## Logging
- **OBS-1** Structured key-value logs only, through the injected `go-kit/log` `log.Logger` with `go-kit/log/level`; no `fmt.Print`, stdlib `log`, or `slog`.
- **OBS-2** Standard keys (snake_case): `msg`, `err`, `service`, `request_id`, `trace_id`, `span_id`, `task_id`, `account_id`, `method`, `status`, `duration_ms`.
- **OBS-3** Levels: `error` needs action; `warn` degraded but handled; `info` lifecycle and one line per request; `debug` details. 4xx at debug, 5xx at error.
- **OBS-4** Never log secrets or PII: passwords, JWTs, download tokens, source credentials, presigned URLs, source URL query strings.
- **OBS-5** One log line per failure ([ERR-13](error-handling.md)).

## Correlation and tracing (OpenTelemetry)
- **OBS-6** Every service initializes the OTel SDK with an OTLP exporter (`OTEL_EXPORTER_OTLP_ENDPOINT`), `service.name` resource, and W3C `traceparent` propagation.
- **OBS-7** Instrument boundaries: `otelhttp` on the gateway, `otelgrpc` stats handlers on every gRPC server and client, a span per message publish and consume.
- **OBS-8** Trace context and `request_id` travel in `pkg/message` metadata (Kafka headers) and gRPC metadata (`x-request-id`). Consumers continue the producer's trace.
- **OBS-9** Spans cover DB queries, outbound calls, and each download execution; failures call `span.RecordError` and set error status.
- **OBS-10** `request_id` is set at the edge ([pkg/middleware/requestid.go](../../pkg/middleware/requestid.go)), logged on every line, and returned in error bodies and the `X-Request-Id` header.

## Metrics (Prometheus)
- **OBS-11** Each service exposes `/metrics` on the admin port via `prometheus/client_golang`, namespace `goload_<svc>_`, with Go runtime and process collectors.
- **OBS-12** RED per HTTP route and gRPC method: `requests_total{method,code}`, `request_duration_seconds` histogram.
- **OBS-13** Messaging: `messages_published_total{topic}`, `messages_consumed_total{topic,result}`, `handler_duration_seconds{topic}`, `dlq_messages_total{topic}`; consumer lag via Kafka exporter.
- **OBS-14** Domain: `downloads_active`, `download_bytes_total{source_type}`, `download_duration_seconds{source_type,result}`.
- **OBS-15** Labels are low cardinality: never IDs, URLs, account names, or error messages.

## Health and alerting
- **OBS-16** `/livez` and `/readyz` on every service ([MS-18](microservices.md)); `/readyz` checks DB, Kafka, Redis, and MinIO as used.
- **OBS-17** Alerts: 5xx ratio and p99 latency per service (SLO burn), consumer lag, DLQ > 0, restart loops, outbox backlog age.
