# ADR-0008: Deprecate API v1; v2 with RFC 7807

- Status: Accepted
- Date: 2026-11-15
- Deciders: project

## Context
Public HTTP API is `/api/v1` with ad-hoc `{"error":...}` bodies and RPC-style paths. We want clearer errors and a path to remove legacy quirks without a big-bang client break.

## Decision
- **`/api/v1` is deprecated and frozen:** bug fixes only; emit `Deprecation` (and later `Sunset`) headers.
- **New work targets `/api/v2`.**
- **v2 errors:** RFC 7807 `application/problem+json` plus stable `code` and `request_id` ([API-5](../standards/api-design.md)).
- **Path style:** keep **RPC-style** consistency with v1 (`/tasks/create`, `/tasks/get?id=`) rather than introducing resource-oriented paths in parallel ([API-3](../standards/api-design.md)).
- **IDs in v2 JSON:** 64-bit integers as strings ([API-9](../standards/api-design.md)).
- Remove v1 entirely once clients migrate (date TBD).

## Consequences
- OpenAPI must document both versions until Sunset.
- Gateway error encoder gains a version-aware branch.
- Clients can migrate incrementally.

## Alternatives considered
- Resource-oriented v2 paths only — rejected for consistency with existing clients and docs.
- RFC 7807 on v1 immediately — avoided; v1 stays frozen shape aside from additive `code` if missing.
