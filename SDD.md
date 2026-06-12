# Quikslate API: System Design Document

## Overview

Quikslate is a scheduling SaaS API built in Go. It allows businesses to manage
locations, employees, positions, and shifts. A user can belong to multiple
businesses, and within each business can work at one or more locations with a
specific role (manager or employee).

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go |
| Framework | Chi (HTTP router) |
| Database | PostgreSQL (via pgx/v5) |
| Auth | JWT (golang-jwt/v5) + HttpOnly refresh token cookie |
| Migrations | Goose |
| Config | Environment variables via `.env` |

---

## Architecture

Three layer architecture:

```
Handler  →  Service  →  Repository
```

- **Handler**: parses HTTP requests, calls service, writes responses
- **Service**: business logic, authorization checks
- **Repository**: SQL queries, no business logic

The `domain` package defines all types and repository interfaces. The `infra/repo`
package implements those interfaces against Postgres. Services depend only on
`domain.Repo` (the interface), never on the concrete implementation — this is
what makes the service layer testable without a database.

---

## Database Schema

### `users`
```sql
id                UUID PRIMARY KEY DEFAULT gen_random_uuid()
email             TEXT NOT NULL UNIQUE
password          TEXT NOT NULL  -- bcrypt hash
invite_token      TEXT
invite_expires_at TIMESTAMPTZ
invite_accepted_at TIMESTAMPTZ
created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `businesses`
```sql
id         UUID PRIMARY KEY DEFAULT gen_random_uuid()
name       TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `business_members`
```sql
user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE
is_admin    BOOLEAN NOT NULL DEFAULT FALSE
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
PRIMARY KEY (user_id, business_id)
```

Answers: "Does this user belong to this business? Are they an admin?"
Queried only at auth time (login, refresh, select-business).

### `locations`
```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE
name        TEXT NOT NULL
address     TEXT
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `location_roles`
```sql
user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE
location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE
role        location_role NOT NULL  -- ENUM: 'manager' | 'employee'
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
PRIMARY KEY (user_id, location_id)
```

Answers: "Does this user work at this location? What is their role?"
Queried only at auth time (select-business, select-location, refresh).

### `positions`
```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE
name        TEXT NOT NULL
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
UNIQUE (business_id, name)
```

Positions are business-scoped, not location-scoped. Example: "Cashier" is
defined once for the business and can be assigned to any employee regardless
of location.

### `employee_positions`
```sql
user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
position_id UUID NOT NULL REFERENCES positions(id) ON DELETE CASCADE
PRIMARY KEY (user_id, position_id)
```

### `shifts`
```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
user_id     UUID REFERENCES users(id) ON DELETE SET NULL  -- nullable (unassigned)
location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE
position_id UUID NOT NULL REFERENCES positions(id)
status      shift_status NOT NULL  -- ENUM: draft|assigned|uncovered|covered|cancelled
start_time  TIMESTAMPTZ NOT NULL
end_time    TIMESTAMPTZ NOT NULL
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

### `refresh_tokens`
```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE
location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE
token       TEXT NOT NULL UNIQUE  -- SHA256 hash of the actual token
expires_at  TIMESTAMPTZ NOT NULL
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

The raw token is never stored. Only the SHA256 hash is persisted.

---

## Authentication & Authorization

### Identity Model

A user session is scoped to exactly one business and one location at a time.
Switching business or location requires a re-auth (issues a new token pair).

### JWT Claims

```go
type Claims struct {
    UserId     string       `json:"user_id"`
    BusinessId string       `json:"business_id"`
    IsAdmin    bool         `json:"is_admin"`
    LocationId string       `json:"location_id"`   // empty for pure admin sessions
    Role       domain.LRole `json:"role"`          // empty for pure admin sessions
    jwt.RegisteredClaims
}
```

Access token expiry: **15 minutes**
Refresh token expiry: **30 days**

JWTs are signed (HS256), not encrypted. The payload is readable but not
modifiable without invalidating the signature. Do not store PII (email, name,
etc.) in the token.

### Token Storage

| Token | Storage | Why |
|---|---|---|
| Access token | JS memory (client) | Short-lived, never persisted |
| Refresh token | HttpOnly cookie (`/auth` path) | Inaccessible to JS, CSRF-safe with SameSite=Lax |

### Auth Flow

```
POST /auth/login
  verify password
  get business memberships (business_members)
    ├── 0 businesses → error
    ├── 1 business   → get location roles
    │     ├── 1 location  → issue token pair → done
    │     └── N locations → return location list (RequiresLocationSelection: true)
    └── N businesses → return business list (RequiresBusinessSelection: true)

POST /auth/select-business  { business_id }
  verify membership (business_members)
  get location roles for this business
    ├── 1 location  → issue token pair → done
    └── N locations → return location list (RequiresLocationSelection: true)

POST /auth/select-location  { business_id, location_id }
  verify membership (business_members)
  verify location role (location_roles) — skipped for admins
  issue token pair → done

POST /auth/refresh  (uses refresh_token cookie)
  verify token exists and not expired
  rotate: delete old token
  re-query business_members and location_roles → fresh data
  issue new token pair → done

POST /auth/logout  (uses refresh_token cookie)
  delete refresh token from DB
  clear cookie
```

### Authorization Checks (Service Layer)

All checks are context reads — zero DB calls:

```go
// Business-level operations (no location context required)
validateIsAdmin(ctx) error

// Location-level operations (admin always passes, otherwise checks role)
validateAdminOrRole(ctx, []domain.LRole{domain.Manager}) error
```

Context values set by middleware on every authenticated request:

```go
ctxkeys.UserId      // string
ctxkeys.BusinessId  // string
ctxkeys.IsAdmin     // bool
ctxkeys.LocationId  // string
ctxkeys.Role        // domain.LRole
```

### Why JWT Claims Instead of Per-Request DB Lookups

- Auth checks are zero DB calls — every request is faster
- Role staleness window is 15 minutes (access token expiry) — acceptable for
  a scheduling app
- Refresh endpoint re-queries DB fresh, so role changes take effect within
  15 minutes automatically
- Simpler service code and easier to test (no repo mock needed for auth checks)

### What Each Table Is Actually Used For

| Table | Queried at auth time | Queried at request time |
|---|---|---|
| `business_members` | login, select-business, refresh | Never (for auth) |
| `location_roles` | select-business, select-location, refresh | Never (for auth) |
| `locations` | select-location (validation) | getAndValidateLocation |
| `shifts` | Never | Every shift operation |
| `positions` | Never | Every position operation |

---

## Authorization Matrix

| Operation | Admin | Manager | Employee |
|---|---|---|---|
| Get/Rename/Delete business | ✓ | ✗ | ✗ |
| Create/Update/Delete location | ✓ | ✗ | ✗ |
| Get location | ✓ | ✗ | ✗ |
| Create/Update/Delete position | ✓ | ✗ | ✗ |
| Assign/Remove employee position | ✓ | ✓ | ✗ |
| Create/Update/Cancel shift | ✓ | ✓ | ✗ |
| Assign/Unassign shift | ✓ | ✓ | ✗ |
| Delete shift (hard) | ✓ | ✗ | ✗ |
| Get shift / Get shifts by location | ✓ | ✓ | ✓ |

---

## Data Validation Rules

| Entity | Rule |
|---|---|
| User email | Must match RFC 5322 pattern |
| User password | Minimum 8 characters |
| Business name | Cannot be empty or whitespace |
| Location name | Cannot be empty or whitespace |
| Shift times | end_time must be after start_time (enforce in service) |
| Position name | Unique per business |

---

## Security Notes

- Passwords hashed with bcrypt
- Refresh tokens stored as SHA256 hash — raw token never persisted
- JWT signed with HS256 — secret from environment, never hardcoded
- Refresh token cookie: HttpOnly, SameSite=Lax, Secure=true in production
- Ownership validated on every resource access (shift belongs to location,
  location belongs to business in JWT) — prevents IDOR attacks
- No PII stored in JWT (no email, name, or personal data in claims)
- Multiple admins per business supported — no single-admin constraint
- Guard against last-admin removal (to be implemented)

---

## Key Decisions & Rationale

### Why location context is in the JWT
A user session is always scoped to one location at a time. Every protected
operation is location-aware. Storing location in the token means zero DB calls
for auth on every request. Switching location requires a re-auth, which is
natural UX for a scheduling app (select your work location when you start).

### Why business_members and location_roles are only queried at auth time
They exist to build and refresh the token, not to gate individual requests.
The token is the materialized result of those table queries. Re-querying them
on every request would be redundant and slower.

### Why positions are business-scoped not location-scoped
A position like "Cashier" or "Supervisor" is a business concept. The same
position name applies across all locations. Assigning an employee to a position
is independent of which location they work at.

### Why refresh tokens store location_id
When rotating a refresh token, the new token must be issued with the same
business and location context. Storing location_id in the refresh_tokens table
allows the refresh endpoint to re-query fresh role data without requiring the
user to re-select their location.

### Why there is no per-request role revocation (no blocklist)
Access tokens are short-lived (15 minutes). Role changes take effect at the
next token refresh. This staleness window is acceptable for a scheduling SaaS.
A token blocklist (Redis-based) can be added later if instant revocation
becomes a customer requirement.

# To-Dos

## 🟠 Phase 3.5: Wire Up the Remaining Resources (blocking — do this next)

All five services below are fully implemented and correct, but **none of them have handlers or routes**. `internal/handler/` currently only contains `auth.go`, and `main.go` only registers `/auth/*` plus a placeholder `/protected` hello-world route. This is the actual next step, ahead of any further polish.

- [ ] **`handler/business.go`** (new file) — handlers for `BusinessService`: get, rename, delete
- [ ] **`handler/location.go`** (new file) — handlers for `LocationService`: create, get, get all, update, delete
- [ ] **`handler/position.go`** (new file) — handlers for `PositionService`: create, get, get all, rename, delete
- [ ] **`handler/shift.go`** (new file) — handlers for `ShiftService`: create, get, get by location, update, assign, unassign, cancel, delete
- [ ] **`handler/employee.go`** (new file) — handlers for `EmployeeService`: add position, remove position, get all positions by user
- [ ] **`cmd/server/main.go`** — register all new routes under `/protected`, behind `middleware.AuthMiddleware`. Remove the placeholder hello-world handler.

---

## 🔵 Phase 4: Production Readiness

- [ ] Pagination on all list endpoints (`GetAllLocations`, `GetShiftsByLocation`, etc.)
- [ ] Structured logging (replace any `log.Println` with `slog` or `zap`)
- [ ] `sslmode=require` in database URL for any non-local environment
- [ ] Environment-based config validation on startup (fail fast if `JWT_SECRET` is empty, etc.)
- [ ] CORS configuration for the frontend origin

---

## ⚪ Phase 5: Invite Flow (deferred)

- [ ] **`domain/`** — Add `Invite` entity (`id`, `email`, `business_id`, `role`, `location_id`, `expires_at`, `accepted`)
- [ ] **`infra/repo/`** — Implement `InviteRepository`
- [ ] **`service/auth.go`** — Split `Register` into two functions:
  - `RegisterAdmin(email, name, password, businessName)` — creates user + business, returns admin-only token (current `Register` behaviour)
  - `RegisterEmployee(inviteToken, name, password)` — validates invite, creates user, assigns role, returns full token
- [ ] **`handler/auth.go`** — `POST /auth/accept-invite` endpoint
- [ ] **`service/`** — `InviteService.CreateInvite` — admin sends invite to email with a signed, expiring token
- [ ] Email sending integration (Resend / Postmark / SES)

---

## Design Notes

**Admin + Location access (settled)**
- Admin-only session (`locationId = ""`) → can manage business, locations, positions, employees. Cannot touch shifts (no location context).
- Admin + location session (`locationId` set) → full access including shifts at that location.
- Frontend should call `POST /auth/select-location` silently when an admin navigates to a specific location's schedule, swap the token, then load the view. The user never sees a manual "select location" step.

**UserId in multi-step auth**
- Login response includes `UserId` when returning `RequiresBusinessSelection: true`
- `SelectBusiness` response includes `UserId` + `BusinessId` when returning `RequiresLocationSelection: true`
- Client holds these in memory (not localStorage) for the duration of the selection flow
- UserId is a UUID, not a credential. All endpoints re-validate against the DB regardless.

**Validation layering (settled)**
- Handler layer: structural validation only — is the request well-formed (required fields present, non-empty)
- Service/domain layer: business-rule validation — format, length, ordering, uniqueness rules
- Business rules stay in `domain`/`service` so they apply no matter what calls them (HTTP handler, future CLI, tests, batch scripts) — a single source of truth rather than re-implemented per entry point.
