# Lessons learned

## Retro

At the end of each task, add 'Proposed context updates' to the report:
- Anything in docs wrong or outdated? Propose a specific diff.
- Did the user correct or redirect you? Propose one rule to prevent repeating it.
- New legacy pattern found? Propose a legacy-map.md entry.
- Architectural decision made? Propose an ADR.
Only write to these files after the user approves. Max 2 lines per lesson. No duplicates, no task-specific lessons. Monthly: promote stable lessons into standards/, delete stale ones.

## Approved lessons

None yet.

## Discovery notes

Newest at the top. These are review findings, not approved lessons. Promote one only after it stays true and the user agrees.

### 2026-11-15 — Status-update events are published but never consumed
- Context: Pause/resume assumed task status becomes `DOWNLOADING` after download starts.
- Mistake: Treating `task.status.updated` as part of the live pipeline without a consumer.
- Rule: Every published topic needs a consumer or must be deleted ([MS-10](../standards/microservices.md)).
- Prevention: Topic inventory in [data-flows](../architecture/data-flows.md); refuse new publishes without a subscriber.

### 2026-11-15 — Ack-before-work loses pause/cancel under load
- Context: Download consumer Ack'd after starting `ExecuteTask` in a goroutine.
- Mistake: Broker offset advanced while work was still running; pause/cancel raced a gone message.
- Rule: Ack only after successful processing for long jobs ([MS-6](../standards/microservices.md)); Option A chosen.
- Prevention: Consumer tests must cover "message still held until done" ([TEST-8](../standards/testing.md)).

### 2026-11-15 — Publish-inside-DB-transaction is not atomic
- Context: Task create/pause/cancel call Kafka `Publish` before/during DB commit.
- Mistake: Assuming "same function = same fate" for DB and broker.
- Rule: Outbox in the same tx, relay after commit ([MS-5](../standards/microservices.md)).
- Prevention: Ban `Publisher.Publish` from service methods that hold a SQL tx.

### 2026-11-15 — Shared MySQL hides service boundaries
- Context: Local docker-compose uses one MySQL for auth and task.
- Mistake: Treating shared DB as temporary without documenting the production rule.
- Rule: DB-per-service in non-local ([MS-2](../standards/microservices.md)); local shared DB is ADR-documented only.
- Prevention: Separate migration directories and connection URLs per service.

### 2026-11-15 — Domain error package must not import gRPC
- Context: `internal/errors.EncodeGRPCError` pulled transport into domain.
- Mistake: Convenience mapping living next to core error types.
- Rule: Encoding stays in adapters ([CC-2](../standards/clean-code.md), [ERR-8](../standards/error-handling.md)).
- Prevention: Exemplars + lint import rules when feasible.

### 2026-11-15 — Completing and failing without state guards
- Context: Task completion/failure consumers update status without checking current state.
- Mistake: Idempotency treated as "always write the new status".
- Rule: Transition tables + ack for redundant events ([MS-7](../standards/microservices.md)).
- Prevention: Unit tests for late `failed` after `COMPLETED`.

### 2026-11-15 — Masking all store errors as NOT_FOUND
- Context: `GetTask` / `DeleteTask` mapped every repo error to not found.
- Mistake: Hiding DB failures from operators and clients.
- Rule: Only `ErrNotFound` → NOT_FOUND; else INTERNAL ([ERR-7](../standards/error-handling.md)).
- Prevention: Repository contracts return typed not-found; assert in service tests.

### 2026-11-15 — SSRF on user-supplied download URLs
- Context: HTTP downloader fetched arbitrary URLs.
- Mistake: Trusting URL host without checking resolved IPs.
- Rule: Block private/link-local/loopback after DNS; re-check on redirect.
- Prevention: Keep dialer `Control` checks; regression tests for private ranges.
