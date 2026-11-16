# ADR-0006: Deploy on Kubernetes

- Status: Accepted (target platform)
- Date: 2026-11-15
- Deciders: project

## Context
Services need horizontal scale, health probes, config/secret injection, and a place for sidecars (mesh optional). Local docker-compose remains for development.

## Decision
Production (and staging) deploy to **Kubernetes**. Each service is a Deployment + Service; probes use existing `/health` (liveness) and a readiness check that verifies critical deps when available. Service discovery: prefer Kubernetes DNS; Consul via go-kit `sd` remains optional for non-k8s or hybrid environments ([MS-17](../standards/microservices.md)).

## Consequences
- Need manifests/Helm/Kustomize (not in repo yet).
- Config via env + Secrets; no baked credentials.
- Local compose stays the fast path for contributors.

## Alternatives considered
- Compose-only forever — rejected for production ops.
- Nomad/VM systemd — out of scope.
