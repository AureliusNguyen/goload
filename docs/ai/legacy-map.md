# Legacy map

**When touching legacy code: do not refactor opportunistically.** Finish the requested change with the smallest safe edit, then report the legacy entry and propose a separate refactor PR.

| Location | Problem | Violates | Correct pattern |
|---|---|---|---|
| `deployments/docker-compose.yaml` + `migrations/mysql/` (auth+task same DB) | Shared MySQL database/schema | MS-2 | Separate DBs/schemas `goload_auth`, `goload_task`; migrations per service |
| `internal/task/service.go` Create/Pause/Resume/Cancel/Retry | Kafka `Publish` inside DB tx | MS-5 | Transactional outbox + relay |
| `internal/download/service.go` `ExecuteTask` + `transport/event_consumer.go` | Ack when download *starts* | MS-6 | Goroutine owns message; Ack/Nack when finished; drain on shutdown |
| `internal/task/transport/event_consumer.go` | No consumer for `task.status.updated`; status never `DOWNLOADING` | MS-10, data-flows #3/#4 | Consume status events or stop publishing; make pause/resume reachable |
| `internal/task/transport/event_consumer.go` + `CompleteTask`/`UpdateTaskError` | No state guard; late `failed` can overwrite `COMPLETED` | MS-7 | Allowed-from transitions; ack redundant events |
| `internal/task/service.go` `GetTask`/`DeleteTask` | Any repo error → NOT_FOUND | ERR-7 | Only map `ErrNotFound`; else INTERNAL |
| `internal/task/mysql/task_repo.go` `Update` | Ignores checksum/metadata | correctness | Persist all fields service writes, or remove dead APIs |
| `internal/task/service.go` `DeleteTask` | Continues delete if cancel publish fails | MS-5 | Outbox or fail the delete |
| `internal/task/transport/grpc.go` Update*/CompleteTask RPCs | No callers (download uses Kafka) | API-16, CC-15 | Delete unused RPCs or document internal-only service |
| `internal/apigateway` imports `internal/task`, `internal/auth` | Cross-service domain imports | CC-3 | Gateway DTOs + clients only; no domain packages |
| Gateway reads Redis tokens + MinIO | Crosses data ownership | MS-1 | Task issues URL only; download/task owns storage access |
| `internal/errors/errors.go` | Domain package imports gRPC | CC-2, ERR-8 | Move `EncodeGRPCError` to transport helpers |
| `internal/auth/account.go` | `sql` tags on domain | CC-5 | Tags on sqlc/DB models only |
| `cmd/apigateway` gRPC clients / all `http.Server` | No deadlines or ReadHeaderTimeout | MS-12 | Deadlines from ctx; set server timeouts |
| Whole stack | No retries/circuit breaker on sync calls | MS-13, MS-14 | go-kit `lb.Retry` + `circuitbreaker.Gobreaker` |
| `pkg/middleware/requestid.go` vs services | Request ID not in gRPC/Kafka | OBS-8 | Propagate metadata/headers |
| No OTel / Prometheus | Logs only | OBS-6–15 | OTLP + `/metrics` |
| `internal/auth/endpoint`, `internal/task/endpoint` | Global 1 req/s limiters; ErrLimited → 500 | MS-16, ERR codes | Rate limit at gateway; map to TOO_MANY_REQUESTS |
| `internal/auth/service.go` CreateSession | NOT_FOUND vs INVALID_PASSWORD | ERR-12 | Same UNAUTHENTICATED message |
| `internal/apigateway/transport.go` decoders | `http.ErrMissingFile` / parse errors → 500 | ERR-10, API-4 | `INVALID_INPUT` → 400 |
| `internal/apigateway/transport.go` `/download` | Plain-text `http.Error` | API-5 | Same JSON/problem error contract |
| `internal/task/service.go` PauseTask | Uses `INVALID_INPUT` for bad state | ERR codes | `INVALID_STATE` → 409 |
| `cmd/task/main.go` | `WithPresigner(nil)` despite building MinIO presigner | CC-15 / clarity | Wire or remove dead code; document intent |
| `cmd/auth/svc` | `AUTH_TOKEN_REGENERATE_BEFORE_EXPIRY` unused | CC-15 | Use or delete |
| `cmd/pocket`, `cmd/pocketsrv` | SQLite DDL duplicated in `main` | CC-18 | Shared migration source |
| `internal/events` | Unversioned events; `task.retried` unpublished topic | MS-8, MS-10 | Envelope + version; create topic or delete publisher |
| `internal/task/mysql/tx_manager.go` | `db.Begin()` ignores ctx | CC-9 | `BeginTx(ctx, …)` |
| `internal/task/service.go`, `endpoint.go` | 800+ line files mixing concerns | CC-10 | Split use cases / mapping / wiring |
| No CI, no `make test` | Untested merges | TEST-13 | CI: lint + `go test -race` + smoke |

## Instruction for agents
1. Match the table before editing a listed file.
2. Prefer the smallest change that preserves behavior unless the task is the refactor.
3. After the task, propose a legacy-map update or a dedicated refactor if you found a new pattern.
