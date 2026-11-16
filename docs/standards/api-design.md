# API design

## REST (apigateway)
- **API-1** Contract-first: edit [api/openapi.yaml](../../api/openapi.yaml), run `make generate` and `make docs`, then implement using `internal/apigateway/gen` types.
- **API-2** Versioned prefix `/api/v{n}`. Breaking change = new version. `/api/v1` is **deprecated and frozen**: bug fixes only, no new endpoints; responses carry `Deprecation: true` and a `Sunset` header once a date is set. New work goes to `/api/v2`.
- **API-3** Keep one path style across versions for consistency with the existing API: `/api/v{n}/tasks/create`, `/api/v{n}/tasks/get?id=`, `/api/v{n}/tasks/pause?id=`. Do not introduce resource-style paths (`/tasks/{id}/pause`) alongside them.
- **API-4** Status codes: 201 + `Location` on create, 200 with body, 204 without, 400 malformed/invalid input, 401, 403, 404, 409 conflict or invalid state, 429, 500, 503 dependency unavailable, 504 upstream timeout.
- **API-5** Errors:
  - **v2:** RFC 7807 `application/problem+json` with `type`, `title`, `status`, `detail`, plus `code` (from [error-handling](error-handling.md#codes)), `request_id`, and `errors[]` (`field`, `message`) for validation.
  - **v1 (deprecated):** `{"error":"<message>","code":"<CODE>","request_id":"<id>"}`. Add `code` if missing; no new v1 formats.
- **API-6** Validate at the edge: required fields, enums, ranges; body size capped with `http.MaxBytesReader`; unknown JSON fields rejected.
- **API-7** Lists are paginated: `limit` (default 20, max 100) + `offset` or cursor; stable ordering; response includes `total` or `next_cursor`.
- **API-8** Create endpoints accept an `Idempotency-Key` header; repeated keys per account within 24h return the original response.
- **API-9** JSON fields `snake_case`; timestamps RFC 3339 UTC; enums `UPPER_SNAKE` strings; 64-bit IDs serialized as strings in v2.
- **API-10** Never return internal details: storage keys, source credentials, SQL, file paths, stack traces.
- **API-11** Every endpoint has an `operationId`, request/response schemas, and all error responses documented.

## gRPC (auth, task)
- **API-12** Proto packages versioned `<svc>.v1`; files in `api/`; generated into `internal/<svc>/pb`.
- **API-13** Never reuse or renumber fields; removed fields and names go in `reserved`.
- **API-14** One request and one response message per RPC, named `<Rpc>Request`/`<Rpc>Response`.
- **API-15** Breaking changes detected by `buf breaking` against `main` before merge.
- **API-16** Only expose RPCs with a caller. Worker-only operations go in a separate service definition or don't exist.
- **API-17** Servers return status codes only via the canonical mapping; status messages are client-safe.
- **API-18** Clients always set deadlines ([MS-12](microservices.md)).
