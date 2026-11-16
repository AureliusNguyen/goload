# auth

Status: **mixed** (closest to standard). Catalog: [service-catalog](../architecture/service-catalog.md#auth-mixed-closest-to-standard).

## Responsibility
Accounts, password hashing, JWT session issue/verify. Does NOT authorize access to tasks.

## Owned data
- MySQL: `accounts` (unique `account_name`), `account_passwords`, `token_public_keys`. Schema: [internal/auth/mysql/sqlc/schema.sql](../../internal/auth/mysql/sqlc/schema.sql), migrations in [migrations/mysql](../../migrations/mysql/) (shared with task).
- Redis: `auth:account_name:*` (name-taken set), `auth:token_public_key:*` (public-key cache).

## Communication
- In: gRPC `auth.v1.AuthService` on `AUTH_SERVICE_GRPC_ADDRESS` (`:8081`): `CreateAccount`, `CreateSession`, `VerifySession`. Contract: [api/auth.proto](../../api/auth.proto).
- Out: none (no events).

## Folder structure
| Path | Role | Status |
|---|---|---|
| [internal/auth/service.go](../../internal/auth/service.go) | Use cases + ports (`AccountStore`, `TxManager`, `PasswordHasher`) | standard candidate |
| `internal/auth/account*.go`, `token_public_key.go` | Domain types | mixed: `sql` tags on domain struct |
| `internal/auth/token_manager.go`, `noop_token_manager.go` | JWT RS512 / pocket no-op tokens | mixed: uses `context.Background()` |
| `internal/auth/endpoint` | go-kit endpoints + rate limiting | mixed: global 1 req/s limiter |
| `internal/auth/transport` | gRPC server/client, error encoding | standard candidate |
| `internal/auth/mysql` | sqlc-backed stores, tx manager | mixed |
| `internal/auth/sqlite` | Pocket stores | mixed |
| `internal/auth/cache` | Redis cache decorators over stores | mixed |
| `internal/auth/mysql/sqlc`, `internal/auth/pb` | Generated | do not edit |

## Gotchas
- RSA key pair is generated at every startup; public keys accumulate in `token_public_keys`. Old tokens still verify by `kid`.
- `CreateSession` returns NOT_FOUND for unknown accounts and INVALID_PASSWORD for wrong passwords (enumeration risk).
- Each endpoint has its own `rate.NewLimiter(1, 100)`. `VerifySession` runs on every gateway request.
- `cache.accountStoreCache.CreateAccount` short-circuits with ALREADY_EXISTS if the name is in the Redis set. The DB unique key is the real guard; cache errors are only logged.
- Custom codes `INVALID_PASSWORD`, `INVALID_TOKEN` live in [internal/auth/errors.go](../../internal/auth/errors.go) and are mapped to `Unauthenticated` in transport.

## Config (env)
`MYSQL_*`, `REDIS_*`, `AUTH_HASH_BCRYPT_COST`, `AUTH_TOKEN_RSA_BITS`, `AUTH_TOKEN_EXPIRES_IN`, `AUTH_TOKEN_REGENERATE_BEFORE_EXPIRY` (parsed but unused), `AUTH_SERVICE_GRPC_ADDRESS`, `LOG_LEVEL`.

## Commands
- Build: `go build ./cmd/auth/svc`
- Test: `go test ./internal/auth/...`
- Regenerate sqlc/proto: `make generate`
