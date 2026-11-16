# Enforcement proposals

Not implemented. Approve a section before any config or test code lands.

## 1. Import and layer checks

Go has no ArchUnit. Use two checks that match [CC-1](../standards/clean-code.md)–[CC-4](../standards/clean-code.md):

**A. `depguard` rules** (already enabled in `.golangci.yml`, only a deprecated-package rule exists today):

| Package prefix | Deny |
|---|---|
| `internal/auth`, `internal/task`, `internal/download` (non-transport, non-endpoint) | `google.golang.org/grpc`, `github.com/go-kit/kit`, `database/sql`, `github.com/IBM/sarama`, `github.com/redis/go-redis`, `github.com/minio/minio-go`, `net/http` |
| `internal/apigateway` | `internal/task`, `internal/auth` (gateway must not import other services' domain) |
| `internal/auth` | `internal/task`, `internal/download`, `internal/apigateway` |
| `internal/task` | `internal/auth`, `internal/download`, `internal/apigateway` |
| `internal/download` | `internal/auth`, `internal/task`, `internal/apigateway` |
| `pkg/` | `github.com/yuisofull/goload/internal` |

Allow transport and endpoint packages to import go-kit and gRPC. Allow `internal/errors` to import neither gRPC nor HTTP once `EncodeGRPCError` moves (ERR-8). Until that move, a deny on `internal/errors` → grpc will fail the build; treat that move as the first PR in this section.

**B. Optional `go test` in `internal/arch`:** walk packages with `golang.org/x/tools/go/packages` and assert the same edges. Use this if depguard's file globs get awkward. Do not add a Java ArchUnit port or dependency-cruiser.

Start in report-only mode (lint job non-blocking) until the known violations in [legacy-map](legacy-map.md) are fixed or baselined with explicit `//nolint` plus a legacy-map row.

## 2. Contract tests

Already specified, not wired:

| Boundary | Check | Standard |
|---|---|---|
| `api/*.proto` | `buf breaking` against `main` in CI | TEST-9 |
| `api/openapi.yaml` | handler test + `kin-openapi` response validation for 2xx and each documented error | TEST-10 |
| `internal/events` | golden JSON in `internal/events/testdata/`; producer marshal and consumer unmarshal | TEST-11 |

`buf` is not in the repo yet. Add `buf.yaml` lint + breaking only. Do not generate a second protobuf toolchain beside `scripts/generate.sh`.

Integration tests that need Kafka, MySQL, or MinIO should move to `//go:build integration` (TEST-7). Today several use `testing.Short()` skips.

## 3. Claude / CI hooks (proposed, not installed)

Do not replace `.claude/settings.local.json`. A shared `.claude/settings.json` could run, after an agent edit:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "git diff --name-only | grep -E '\\.go$' && go test -count=1 $(git diff --name-only | grep '_test.go\\|.go$' | xargs -n1 dirname | sort -u | sed 's/^/./') || true"
          }
        ]
      }
    ]
  }
}
```

That snippet is a sketch: it is noisy, skips build tags, and can run testcontainers. Prefer a narrower hook:

1. If staged files are under `internal/<svc>` or `pkg/`, run `go test` for those packages only.
2. Skip `pb/`, `sqlc/`, and `gen/`.
3. Run `golangci-lint` only for the blocking linters (CC-16), not the full copied config.
4. Put the same commands in GitHub Actions so hooks are a convenience, not the only gate. There is no CI workflow today.

Do not add a hook that runs `make lint` (it passes `--fix` and rewrites the tree).

## 4. Refactor order

Smallest risk first. Each item is its own change, with a failing test first ([refactor-legacy](../../.claude/skills/refactor-legacy/SKILL.md)).

| # | Change | Why this order |
|---|---|---|
| 1 | `GetTask` / `DeleteTask`: only `ErrNotFound` → NOT_FOUND | Local, easy to test, no protocol change |
| 2 | Pause uses `INVALID_STATE`; decoders return `INVALID_INPUT` (400) | Client-visible but mechanical |
| 3 | Login: unknown account and bad password both `UNAUTHENTICATED` | Small auth change; stops enumeration |
| 4 | `BeginTx(ctx)` in task and auth tx managers | No behavior change on the happy path |
| 5 | Persist checksum and metadata in task `Update`, or delete the dead API | Stops silent data loss |
| 6 | Remove unused task Update* RPCs after confirming no caller | Proto breaking change; do after buf is in CI |
| 7 | Download consumer: ack after completion (ADR-0004) | Changes crash and pause behavior |
| 8 | Consume `task.status.updated` or stop publishing it; then pause/resume can be tested | Unblocks the state machine |
| 9 | Task outbox (ADR-0003) | Schema + relay; do after 7–8 so events have a real consumer |
| 10 | Separate `goload_auth` and `goload_task` databases | Ops change; local compose last |
| 11 | Propagate request id, then OTel + Prometheus (ADR-0007) | After the correctness bugs, so traces show the real flow |
| 12 | `/api/v2` problem+json; freeze v1 (ADR-0008) | Client migration, not a silent fix |

Items 1–4 are safe to start without new infrastructure. Items 7–9 are the correctness core of the download pipeline.
