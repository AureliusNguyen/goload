# Error handling

## Creating and propagating
- **ERR-1** Check every returned error. `_ =` only with a comment saying why it is safe.
- **ERR-2** Wrap when adding context: `fmt.Errorf("load task %d: %w", id, err)`. Use `%w`, never `%v`, for errors callers may inspect.
- **ERR-3** Inspect with `errors.Is`/`errors.As`/`errors.IsError`; never compare `err.Error()` strings.
- **ERR-4** Check or join (`errors.Join`) commit, rollback, close, and flush errors where they affect correctness.

## Classification boundaries
- **ERR-5** Driven adapters translate driver errors before returning to core: `sql.ErrNoRows` → `errors.ErrNotFound`, MySQL 1062 / SQLite UNIQUE → `ALREADY_EXISTS`. No driver error types reach core.
- **ERR-6** Core returns `*errors.Error{Code, Message, Cause}`. `Message` is client-safe, low-cardinality, and contains no IDs, paths, or SQL. `Cause` keeps internals.
- **ERR-7** Map only "not found" to `NOT_FOUND`. Any other repository failure is `INTERNAL`.
- **ERR-8** Transport alone translates codes: gRPC via one encoder, HTTP via `encodeError`. Code that maps to gRPC/HTTP lives in transport packages; `internal/errors` itself imports neither.
- **ERR-9** Responses with status ≥ 500 never include `Message` or `Cause`; 4xx include `Message` only.
- **ERR-10** Decode and validation failures are `INVALID_INPUT` with per-field details, never 500.

## Codes
| Code | gRPC | HTTP |
|---|---|---|
| `INVALID_INPUT` | InvalidArgument | 400 |
| `UNAUTHENTICATED`, `INVALID_TOKEN`, `INVALID_PASSWORD` | Unauthenticated | 401 |
| `PERMISSION_DENIED` | PermissionDenied | 403 |
| `NOT_FOUND` | NotFound | 404 |
| `ALREADY_EXISTS`, `CONFLICT`, `INVALID_STATE` | AlreadyExists / Aborted / FailedPrecondition | 409 |
| `TOO_MANY_REQUESTS` | ResourceExhausted | 429 |
| `INTERNAL`, `UNKNOWN` | Internal / Unknown | 500 |
| dependency down (`Unavailable`) | Unavailable | 503 |
| upstream timeout (`DeadlineExceeded`) | DeadlineExceeded | 504 |

- **ERR-11** `context.Canceled` from a client disconnect is logged at debug and not counted as a server error.
- **ERR-12** Auth failures don't reveal account existence: unknown account and wrong password both return `UNAUTHENTICATED` "invalid credentials".

## Logging and panics
- **ERR-13** Log an error once, at the boundary that handles it (transport error handler or consumer loop), with `request_id`/`trace_id`. Inner layers wrap and return.
- **ERR-14** No panics for expected conditions. Every request handler and every spawned goroutine has `recover` that logs a stack trace and converts to `INTERNAL`.

## Consumers
- **ERR-15** Decode failures and `INVALID_INPUT` are permanent: send to DLQ, then ack. `NOT_FOUND` for a deleted aggregate: ack. Everything else: nack for redelivery ([MS-11](microservices.md)).
