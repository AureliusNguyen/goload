# Testing

Pyramid: many unit → fewer adapter integration → contract tests per boundary → one e2e smoke.

| Kind | Tests | Speed | Needs |
|---|---|---|---|
| Unit | one function/use case with fakes | ms | nothing |
| Integration | one adapter against a real DB/Kafka/MinIO | seconds | Docker |
| Contract | a boundary (proto, OpenAPI, event JSON) stays compatible | ms | nothing |
| E2E smoke | the whole system, one happy path | minutes | docker-compose |

## Unit
- **TEST-1** Core use cases are tested with hand-written fakes for ports; no network, DB, or sleeps. Pattern: [internal/auth/service_test.go](../../internal/auth/service_test.go).
- **TEST-2** Table-driven with named `t.Run` cases; `t.Parallel()` when no shared state; testify `require` for preconditions, `assert` for checks.
- **TEST-3** Assert the contract, not just `err != nil`: error code (`errors.IsError`), HTTP status and body, gRPC code, published events.
- **TEST-4** Every bug fix starts with a failing test that reproduces it.
- **TEST-5** No `time.Sleep` for synchronization; use channels, `require.Eventually`, or injected clocks.

## Integration (adapters)
- **TEST-6** Adapters (`mysql/`, Kafka, MinIO, Redis) are tested against real dependencies via `testcontainers-go`.
- **TEST-7** Integration tests use the `//go:build integration` tag (not `testing.Short()`), run with `go test -tags=integration ./...`.
- **TEST-8** Consumers are tested for redelivery (same message twice → one effect) and ack/nack per error class.

## Contract
- **TEST-9** gRPC: `buf breaking` against `main` on every change to `api/*.proto`.
- **TEST-10** REST: handler tests validate responses against `api/openapi.yaml` (`kin-openapi/openapi3filter`) for success and each documented error.
- **TEST-11** Events: golden JSON fixtures per event type and version in `internal/events/testdata/`; producer tests marshal to match them and consumer tests unmarshal from them.

## End-to-end
- **TEST-12** One smoke flow (register → login → create task → task COMPLETED → download URL works) against docker-compose, via `make smoke`.

## Hygiene
- **TEST-13** CI runs `go test -race ./...`; packages that start goroutines use `goleak.VerifyTestMain`.
- **TEST-14** Changed core packages keep ≥ 80% statement coverage (`go test -cover`).
- **TEST-15** Test names: `TestType_Method_Scenario`. Fakes live in the test file or `<pkg>test` package, never in production code.

## Commands
```
go test -short ./...                       # unit (until build tags land)
go test -race ./...                        # all incl. testcontainers (needs Docker)
go test -tags=integration ./...            # target after TEST-7
```
