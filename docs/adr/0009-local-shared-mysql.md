# ADR-0009: Local shared MySQL convenience

- Status: Accepted
- Date: 2026-11-15
- Deciders: project

## Context
Developers want a one-command local stack. Strictly separate MySQL servers per service is heavy for laptops. [ADR-0001](0001-database-per-service.md) still requires clear ownership in non-local environments.

## Decision
**Local docker-compose only** may use a single MySQL server. Prefer separate databases (`goload_auth`, `goload_task`) on that server. A single shared database is tolerated only as a temporary local shortcut and must not be copied into staging/production compose or k8s manifests.

## Consequences
- Document connection URLs per service even when host is shared.
- Production/staging always use isolated databases (and preferably isolated instances).

## Alternatives considered
- Force multiple MySQL containers locally — clearer isolation, higher RAM; optional for contributors who want it.
