---
name: refactor-legacy
description: Refactor a goload legacy area in small test-first steps. Use when the user asks to clean up task, the gateway, consumers, or a legacy-map row. Do not start a drive-by refactor.
---

# Refactor legacy

1. Only when the user asked for the refactor, or they approved a proposal from the last task.
2. Find the row in [legacy-map](../../../docs/ai/legacy-map.md). One row per change. Do not clear the table in one PR.
3. Read [exemplars](../../../docs/ai/exemplars.md). If the concern says "no exemplar", make this change the golden path and update the exemplar table after approval.
4. Write a failing test that locks current correct behavior and the bug ([TEST-4](../../../docs/standards/testing.md)). Copy [internal/auth/service_test.go](../../../internal/auth/service_test.go).
5. Smallest edit that meets the standard cited in the row. No unrelated renames, no file splits unless the task is the split.
6. Keep pocket and microservice adapters working: behavior differences stay in adapters ([CC-17](../../../docs/standards/clean-code.md)).
7. Run `go test` for the packages touched, with `-race` when the change starts goroutines or touches shared fields.
8. Do not "fix" lint nits outside errcheck, govet, staticcheck, gosec, errorlint, and bodyclose unless the user asked (CC-16).
9. After the change, propose a legacy-map edit (delete the row or narrow it). Do not edit the map until the user agrees ([Retro](../../../docs/ai/lessons.md)).
10. Order, smallest risk first, is in [enforcement](../../../docs/ai/enforcement.md#refactor-order).
