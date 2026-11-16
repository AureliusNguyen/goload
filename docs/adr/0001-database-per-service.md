# ADR-0001: Database per service

- Status: Accepted
- Date: 2026-11-15
- Deciders: project

## Context
Auth and task currently share one MySQL instance and schema in local docker-compose. That couples migrations, backup, and access control, and invites cross-service SQL.

## Decision
Each service owns its data store and schema. Auth and task use separate databases (or schemas) and separate migration trees. Cross-service data access is only via API or events ([MS-1](../standards/microservices.md), [MS-2](../standards/microservices.md)).

**Exception:** local docker-compose may run a single MySQL *server* with separate databases, or temporarily a shared database for developer convenience — never as a production pattern. See [ADR-0009](0009-local-shared-mysql.md).

## Consequences
- Migrations, credentials, and backups are per service.
- No joins across auth and task data.
- Local setup must document DB names and migration order.

## Alternatives considered
- Shared database with table prefixes — rejected; blurs ownership.
- Shared schema forever — rejected for production.
