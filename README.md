# QuikSlate API

The backend for QuikSlate, employee scheduling software for shift-based businesses such as restaurants and retail stores. A business has locations, each location has positions and shifts, and staff join through invites and are given a role at each location they work.

Go, PostgreSQL, Docker. Work in progress: the core scheduling flow works end to end, and deployment hardening is ongoing.

## Running it

Requires Docker with Compose.

```bash
cp .env.example .env      # fill in the blanks; `openssl rand -hex 32` for secrets
docker compose up -d --build
```

This starts Postgres, applies migrations, then starts the API on `API_PORT` (default `8080`). Interactive API docs are served at `http://localhost:<API_PORT>/v1/swagger/index.html`.

To run the API on the host instead, start only the database and use the host `DATABASE_URL` from `.env`:

```bash
docker compose up -d postgres
go run ./cmd/migrate up
go run ./cmd/server
```

### Checks

```bash
scripts/ci.sh             # gofmt, build, vet, test: the same script CI runs
```

Tests are table-driven and use the standard library. Repository tests start a throwaway Postgres through Testcontainers, so Docker must be running. `tests/smoke_auth_flow_with_invite.sh` exercises the auth and invite flow against a running server.

## Architecture

```
cmd/server         composition root: config, wiring, router, graceful shutdown
cmd/migrate        migration runner (up / down / reset), SQL embedded in the binary
internal/domain    entities, repository interfaces, domain errors; no framework or DB imports
internal/service   business logic and authorization; depends only on domain interfaces
internal/handler   HTTP: parsing, middleware, routing, mapping errors to status codes
internal/infra     Postgres repository (pgx) and mailer implementations
internal/auth      JWT signing and password hashing
```

Dependencies point inward. Services know nothing about HTTP or Postgres, which keeps business rules testable without either, and lets the storage layer change without touching them.

## Design decisions

Each decision below lists what was chosen, why, and what it costs.

### Short-lived access tokens with rotating refresh tokens

Access tokens are JWTs that expire after 15 minutes and carry only the user's ID. The client keeps them in memory. Refresh tokens are random opaque values, valid for 30 days, sent as an `HttpOnly` cookie scoped to `/v1/auth`, and stored server-side only as SHA-256 hashes. Every refresh deletes the old token and issues a new one inside a single transaction.

- **Why:** pure JWTs cannot be revoked before they expire, and pure server sessions need a database lookup on every request. This splits the difference: most requests verify a signature, while logout and revocation take effect at the next refresh. Keeping the refresh token out of JavaScript means an XSS bug cannot steal a long-lived credential.
- **Cost:** a revoked user keeps access for up to 15 minutes. The cookie only works cross-origin when CORS allow-lists the exact frontend origin with credentials enabled, and a misconfiguration fails silently: the browser simply drops the cookie.

### Authorization is looked up on every request

The JWT proves who the caller is and nothing else. Business membership, admin status and location role are read from the database per request by middleware.

- **Why:** permissions embedded in a token are stale until it expires. Removing a manager or demoting an admin should take effect immediately, not up to 15 minutes later.
- **Cost:** one extra query per scoped request. At this scale that is cheap, and it is the first thing to cache if it ever becomes a bottleneck.

### Middleware identifies, services decide

Three middlewares build up context in increasing specificity: identity, then business membership, then location role. They establish who the caller is. Whether the caller may perform an action is decided in the service layer, through a small set of shared checks, against a ranked role hierarchy (business admin > location lead > manager > employee).

- **Why:** permission rules depend on the target as well as the caller ("a manager may edit employees at their own location, but not other managers"). That logic belongs with the business rules it protects, where it is unit-tested without HTTP.
- **Cost:** every new service method must remember to call a check. Middleware cannot enforce it on its behalf.

### One error vocabulary, one translation point

Services return domain errors (`ErrNotFound`, `ErrForbidden`, validation errors with client-safe messages). A single function in the handler layer maps them to HTTP status codes; anything unrecognized is logged and returned as a generic 500.

- **Why:** status codes are an HTTP concern, and internal error details must never leak to clients. Centralizing the mapping keeps both rules in one place.

### Least-privilege database roles and gated migrations

Postgres has two non-superuser roles: one that owns and alters the schema, used only by migrations, and one limited to reading and writing rows, used by the API. Migrations run as a one-shot container; the API starts only after it exits successfully.

- **Why:** a SQL injection bug in the API cannot drop tables or reach superuser-only features that execute shell commands. A failed migration keeps the API down, which is better than serving traffic against a half-migrated schema.
- **Cost:** more moving parts in setup. Role passwords are applied only when the database volume is first created, so changing them later requires `ALTER ROLE`.

### Everything under `/v1`

All routes live under `/v1`, so production can serve the frontend and API from one origin, with a single proxy rule sending `/v1/*` to the API.

- **Cost:** the refresh cookie's path is tied to this prefix. Changing one without the other breaks sessions silently: login succeeds and the session is lost on the next reload.

### Container hardening

Images are static binaries on Alpine, running as a non-root user with a read-only filesystem and all Linux capabilities dropped. The server sets read, write and idle timeouts and shuts down gracefully on `SIGTERM`. The IANA timezone database is compiled into the binary, because Alpine ships without one and every non-UTC location would otherwise fail validation.

## Contributing

This is a personal project and is not accepting contributions at the moment. Issues and questions are welcome.

## License

Copyright © Dante Grieco. All rights reserved. The source is public for reading; no license is granted to use, copy, or distribute it.
