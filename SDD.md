# Quikslate API: System Design Document

## Overview

Quikslate is a scheduling SaaS API built in Go. It allows businesses to manage
locations, employees, positions, and shifts. A user can belong to multiple
businesses, and within each business can work at one or more locations with a
specific role (manager or employee).

A user identity, a business, and a business membership are three independent
concepts. A person can exist with no business at all, can own more than one
business, and can be a member of a business without ever having created it.
Every membership — however it was created — requires that person's explicit
consent before it's active.

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

- **Handler**: parses HTTP requests, performs structural validation (required
  fields present, non-empty), calls service, writes responses
- **Service**: business-rule validation (format, length, ordering, uniqueness),
  authorization checks
- **Repository**: SQL queries, no business logic

The `domain` package defines all types and repository interfaces. The `infra/repo`
package implements those interfaces against Postgres. Services depend only on
`domain.Repo` (the interface), never on the concrete implementation — this is
what makes the service layer testable without a database.

---

## Database Schema

### `users`
```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
email       TEXT NOT NULL UNIQUE
name        TEXT NOT NULL
password    TEXT NOT NULL     -- bcrypt hash
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

A `users` row is only ever created one way: `Register`, with a real name and
password supplied by the person themselves. Nothing else writes to this
table — in particular, **creating an invite never touches `users`**. See
[Invite & Consent Model](#invite--consent-model). This table has no reference
to any business; a user existing with zero memberships is a normal, valid
state.

*(Earlier revisions of this schema made `name`/`password` nullable to
represent an "unclaimed identity" created as a side effect of an invite. That
design is gone — see the rationale below — but the currently-applied migration
still has these columns as nullable. Nothing writes `NULL` there anymore, so
it's inert, but a follow-up migration to restore `NOT NULL` is worth doing
once any leftover unclaimed rows from the old design are cleaned out of
whichever database this has been tested against.)*

### `businesses`
```sql
id         UUID PRIMARY KEY DEFAULT gen_random_uuid()
name       TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

No owner column. Ownership is expressed entirely through `business_members`.
Any authenticated user can create a business at any time — it isn't tied to
registration.

### `business_members`
```sql
user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
is_admin    BOOLEAN NOT NULL DEFAULT FALSE,
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
PRIMARY KEY (user_id, business_id)
```

*(Note the trailing commas on every column line above — a real migration
draft of this table shipped without them and failed to run. Called out
explicitly because it's exactly the kind of thing that's invisible on a quick
read.)*

Answers: "Does this user belong to this business? Are they an admin?" Queried
at auth time (login, refresh, select-business).

There is deliberately no "pending" state here anymore. A `business_members`
row's mere existence *is* active membership — every row, whether created by
`CreateBusiness` (self-service, `is_admin: true`) or by accepting an invite
(`is_admin: false`), is fully active the instant it's written. Nothing ever
inserts a row here on someone's behalf before they've consented — see
[Invite & Consent Model](#invite--consent-model) for why that's now
structurally true rather than something each write path has to remember to
enforce.

### `invites`
```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
token_hash  TEXT NOT NULL UNIQUE,  -- SHA256 hash; raw token never stored, same treatment as refresh_tokens
email       TEXT NOT NULL,         -- lowercased before insert; NOT a foreign key to users
business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
role        location_role NOT NULL, -- reuses the ENUM defined for location_roles
expires_at  TIMESTAMPTZ NOT NULL,
accepted_at TIMESTAMPTZ,           -- NULL = still pending
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

An invite is identified by **email, not `user_id`** — deliberately, since at
invite-creation time there may be no `users` row at all, and creating one
just to hang an invite off of is exactly the design this table replaces (see
[Why invites live in their own table](#why-invites-live-in-their-own-table-not-on-business_members-or-users)
below). An unaccepted invite is nothing more than a token, an email, and an
expiry — never a placeholder identity.

No uniqueness constraint ties `(email, business_id)` together: re-inviting the
same person creates a new row with its own token/expiry, and old rows simply
go stale on their own via `expires_at`. `token_hash` is the only thing that
must be unique. Index on `email` if/when a "list my pending invites" lookup
is added (see the note in [Invite & Consent Model](#invite--consent-model)) —
not required for the core flow.

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

Answers: "Does this user work at this location? What is their role?" Queried
only at auth time (select-business, select-location, refresh) and — as a
narrow, deliberate exception — when a Manager acts on a specific employee (see
[Two Authorization Patterns](#two-authorization-patterns) below, that lookup
is about the *target*, not the caller, and isn't the kind of per-request DB
call this design avoids). Does not carry its own consent state — its validity
is inherited from the corresponding `business_members` row existing at all
(both are now written together, at accept time — see
[Invite & Consent Model](#invite--consent-model)).

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
of location. This is also why assigning a position to an employee is, by
default, an Admin action available at any session tier — see the
Authorization Matrix.

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

Unlike positions, a shift genuinely belongs to one specific location — this is
why shift operations, unlike position assignment, require the caller to have
an active location in their session even if they're an admin. See
[Two Authorization Patterns](#two-authorization-patterns).

### `refresh_tokens`
```sql
id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
business_id UUID REFERENCES businesses(id) ON DELETE CASCADE  -- nullable: identity-only sessions
location_id UUID REFERENCES locations(id) ON DELETE CASCADE   -- nullable: admin-only sessions
token       TEXT NOT NULL UNIQUE  -- SHA256 hash of the actual token
expires_at  TIMESTAMPTZ NOT NULL
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
```

The raw token is never stored, only the SHA256 hash. Both `business_id` and
`location_id` are nullable to represent the two "less than fully scoped"
session tiers below. Conversion between the domain layer's `""` sentinel and
SQL `NULL` happens only at the repository boundary — nowhere else in the
codebase needs to know these columns are nullable. (`business_id` needs the
same nullable treatment `location_id` already has — an identity-only session's
refresh token has neither.)

---

## Authentication & Authorization

### Identity Model

A user session is scoped to **at most** one business and one location at a
time — but it doesn't have to be scoped to either. There are three tiers:

| Tier | `business_id` | `location_id` | Can do |
|---|---|---|---|
| Identity-only | empty | empty | Create a business, accept an invite |
| Admin-only | set | empty | Manage business, locations, positions, employees, employee positions — not shifts |
| Full (admin+location, or manager/employee) | set | set | Everything in-scope, including shifts |

Switching business or location requires a re-auth (issues a new token pair).

A Manager or Employee session is, by construction, always in the Full tier —
there's no "manager with no location" state, since a location role is what
makes someone a manager or employee in the first place.

Every response above is a fully **resolved** session — the caller knows
exactly what they can do next. Two more responses exist that are
deliberately *not* resolved: `Login` finding several businesses, or
`select-business` finding several locations within the chosen one. Both
still need to hand the client something to act on before the flow can
continue. See [JWT Claims](#jwt-claims) for why that "something" is just an
ordinary access token minted without its usual refresh token, rather than a
separate token type.

### JWT Claims

There is exactly one JWT shape in this system.

```go
// Claims — accepted by AccessAuthMiddleware everywhere it's required.
type Claims struct {
    UserId     string       `json:"user_id"`
    Purpose    string       `json:"purpose"`      // always "access"
    BusinessId string       `json:"business_id"`  // empty for identity-only sessions
    IsAdmin    bool         `json:"is_admin"`
    LocationId string       `json:"location_id"`  // empty for identity-only or admin-only sessions
    Role       domain.LRole `json:"role"`          // empty unless a location role applies
    jwt.RegisteredClaims
}
```

`purpose` is kept even though only one value (`"access"`) currently exists —
it costs one field and means a future second JWT-based mechanism, if one is
ever needed, doesn't require retrofitting a purpose check onto every
existing `AccessAuthMiddleware` call site.

What varies between a fully resolved session and a caller still mid-login
(more than one business, or more than one location within the chosen
business) is **not** the token's shape — it's whether a refresh token is
minted to go with it:

- **Resolved** (identity-only with zero businesses, admin-only, or fully
  location-scoped): access token + a persisted, cookied refresh token, same
  as always.
- **Mid-selection** (`Login` found several businesses; `select-business`
  found several locations): an access token only — `business_id`/
  `location_id` both empty, functionally identical to an identity-only
  session — with **no** refresh token minted or persisted.

An access token minted this way is, deliberately, just as capable as any
other identity-only access token — it can call `CreateBusiness` or accept an
invite, not only `select-business`. That's not a gap: the caller already
proved their password to `Login` moments earlier, so this isn't a new
identity being trusted, just the same one a moment earlier in the flow.
What actually matters — bounding how long an unresolved, "please pick one"
moment stays exploitable if the token leaks — is handled entirely by there
being no refresh token behind it: once the 15-minute access token expires,
that's the end of it, full stop, no way to silently extend it. See
[Why There's Only One Access Token Type](#why-theres-only-one-access-token-type)
for how this replaced an earlier design that used a second, parallel token
type to express the same thing.

Access token expiry: **15 minutes**, uniformly — whether or not a refresh
token accompanies it
Refresh token expiry: **30 days**

JWTs are signed (HS256), not encrypted. The payload is readable but not
modifiable without invalidating the signature. Do not store PII (email, name,
etc.) in the claims.

### Token Storage

| Token | Storage | Why |
|---|---|---|
| Access token | JS memory (client) | Short-lived, never persisted; the mid-selection flavor is handled identically — it's the same token shape |
| Refresh token | HttpOnly cookie (`/auth` path) | Inaccessible to JS, CSRF-safe with SameSite=Lax; simply absent from the response during mid-selection, not stored anywhere client-side in that case |

### Registration and Business Creation (decoupled)

```
POST /auth/register  { email, password, name }
  create a users row
  issue an identity-only token pair
  (no business created)

POST /businesses  { name }              -- requires any valid session
  insert businesses row
  insert business_members row (is_admin: true)
  -- no acceptance step needed — you don't consent to your own business,
  -- and a business_members row is active the instant it exists
  -- callable at any time, by any authenticated user, regardless of
  -- how many businesses they already own
```

The common cold-signup case (someone starting their first business) is simply
both calls made back to back by the frontend — one user-facing action, two API
calls. Someone who arrives via an invite instead just never calls the second
one unless they choose to start their own business later.

### Auth Flow

```
POST /auth/login
  verify password
  get business memberships (business_members WHERE user_id = ...)
    -- every row here is, by construction, an active membership; there is no
    -- pending state left to filter out (see Invite & Consent Model)
    ├── 0 businesses → issue identity-only token pair → done
    ├── 1 business   → get location roles
    │     ├── 0 locations (admin, no location) → issue admin-only token pair → done
    │     ├── 1 location  → issue full token pair → done
    │     └── N locations → issue an access token only (no refresh token),
    │                       return location list (RequiresLocationSelection: true)
    └── N businesses → issue an access token only (no refresh token),
                        return business list (RequiresBusinessSelection: true)

  a refresh token is only ever minted alongside the access token on the
  branches that resolve in one step ("→ done") — the two branches that
  return a list instead mint an access token with no business/location
  scope and nothing persisted server-side; there is no distinct token type
  for this, see JWT Claims

POST /auth/select-business  { business_id }
  Authorization: Bearer <access token>
POST /auth/select-location  { business_id, location_id }
  Authorization: Bearer <access token>

  Both routes sit behind the same AccessAuthMiddleware as every other
  protected route — no separate middleware exists for this. The service
  functions read ctxkeys.GetUserId(ctx), same as any other protected
  service; there is no user_id field in either request body. Because these
  are ordinary AccessAuthMiddleware routes, they also work for a caller
  who's already fully logged into a different business or location — this
  is what lets someone switch their active business/location without
  logging out, which the earlier selection-token design couldn't do.

  select-business: same branching as the 1-business case above, now for the
  chosen business (may itself return a fresh RequiresLocationSelection — the
  client reuses the access token it already holds for the follow-up
  select-location call; nothing needs to re-issue or echo a new one)

  select-location: fetch the location; require location.business_id ==
  business_id — this is REQUIRED and unconditional, admin or not (see 1.2 in
  the audit — this was previously skipped for admins entirely, and
  unfiltered by business for non-admins, which let a token be minted with a
  business_id/location_id pair that didn't actually belong together); then
  fetch business_member + (if not admin) location_role at that location, and
  mint the full token pair
```

### Why select-business/select-location Still Require a Real Access Token, Not a Bare User ID

The user's own `user_id` is not a secret once they're part of a business —
teammates legitimately see each other's `user_id` in ordinary application data
(a shift roster, a membership list). An endpoint that mints real, signed
access and refresh tokens off nothing but a client-supplied `user_id` +
`business_id` is authenticating on "an ID I looked up," not on anything that
proves the caller is the account holder. `select-business`/`select-location`
require a real, signed access token — the same kind `Login` or `Register`
would issue — proving the caller already passed password verification (or
holds an otherwise-valid session); that's the actual proof of identity for
this step, and it's the only thing these two endpoints trust for `userId`.

### Why There's Only One Access Token Type

An earlier revision of this design gave the "please pick a business/location"
moment its own token type (`SelectionClaims`, a separate `purpose: "select"`,
a 5-minute expiry, a dedicated `SelectionAuthMiddleware`) — reasoning that if
this moment reused the ordinary `Claims` shape, it would parse as a valid
identity-only access token and grant identity-only-tier capabilities
(`CreateBusiness`, accepting an invite) to something that was only supposed
to prove "I can proceed to the next login step."

That reasoning doesn't hold up: by this point in the flow the caller has
already presented a correct password to `Login`. Letting that same moment
also be capable of creating a business or accepting an invite isn't
privilege escalation — they're already a verified identity, and both of
those are already available to any identity-only session. The separate type
was solving a problem the design created for itself by giving this moment a
different shape than it needed, not a problem that had to exist.

The thing actually worth protecting — how long a leaked "please pick a
business" token stays useful — was never really about the token's shape.
`generateTokens` unconditionally mints and persists a 30-day refresh token
every time it runs, including for `Register`'s identity-only case. If this
moment reused that same path, a mid-login, not-yet-resolved state would walk
away with a 30-day-refreshable session, which is a real problem — a fleeting
"pick one" prompt shouldn't have a 30-day tail. **Not minting a refresh
token** is what actually bounds this: once the access token's own 15 minutes
run out, that's the end of it, with no `/auth/refresh` call able to extend
it. That property doesn't require a second token type — it only requires
`generateTokens` to skip its refresh-token half in exactly these two
branches, using the same access token everywhere else.

Two things fall out of this for free rather than needing separate design
work: the class of bug where one branch remembers to mint the interim token
and another forgets (this happened — `SelectBusiness`'s "many locations"
branch shipped without ever minting anything, a dead end) becomes
structurally harder to reintroduce once both branches share the exact same
"access token, no refresh" path instead of each hand-rolling it. And because
`select-business`/`select-location` now accept any valid access token rather
than a single-purpose one, a caller who's already fully logged into one
business can call either endpoint to switch — a capability the old design
had no way to express.

### Invite & Consent Model

Every membership requires explicit consent — nothing is ever silently
granted, whether the invitee is brand new or already has an active account
elsewhere. Unlike the identity/business/membership split above, invites don't
introduce a third acceptance path — they route through the *same* two
already-existing entry points to an account (`Register`, `Login`),
unmodified, plus exactly one new check.

```
POST /invites  { email, location_id, role }   -- admin only, business from JWT
  validate email format, normalize to lowercase
  validate role is a legal location_role
  confirm locationId belongs to the caller's own business (getAndValidateLocation)
  if a user with this email already has an accepted business_members row for
  this business → ErrAlreadyExists ("already a member")
  generate a random token; store only its SHA256 hash
  insert an invites row: email, business_id (from JWT), location_id, role,
    expires_at: +7 days, accepted_at: NULL
  email the invite link containing the RAW token (never stored, never
  returned by this endpoint again)

GET /invites/{token}   -- public, unauthenticated
  look up by hashed token → 404 if missing, expired, or already accepted
  return { email, business_name, role, expires_at } only — enough for the
  frontend to render "you've been invited to join <business>" and prefill
  the email field on whichever of register/login it shows next. Knowledge of
  the token is already equivalent to mailbox access, so this leaks nothing
  beyond what the invite itself already disclosed.

POST /invites/{token}/accept   -- authenticated (any tier, including identity-only)
  look up by hashed token → 404 if missing, expired, or already accepted
  load the caller's own user row (ctxkeys.GetUserId(ctx) → GetUserById)
  if caller.email != invite.email (case-insensitive) → 403 Forbidden
  AddUserToBusiness(callerId, invite.business_id, is_admin: false)
    -- idempotent: ON CONFLICT (user_id, business_id) DO NOTHING
  AssignRole(callerId, invite.business_id, invite.location_id, invite.role)
    -- already idempotent today: ON CONFLICT (user_id, location_id) DO UPDATE
  mark the invites row accepted_at = now
  delegate into SelectBusiness(ctx, invite.business_id) and return that
    AuthResponse — the caller gets fresh tokens scoped to the business they
    just joined, without a second round trip
```

**Nothing is created until the moment of acceptance.** `CreateInvite` never
touches `users`, and never creates a `business_members` row — it only ever
writes to the standalone `invites` table, keyed by email. This is the
structural fix for two real problems the previous design had: a "dummy user"
created purely because an admin typed an email into a form (a privacy
concern, since that person never chose to have an account), and an orphaned
pending-membership row left behind forever if the invite is never accepted
(an unaccepted `invites` row is just a token+email+expiry — mundane, and it
naturally goes stale). See
[Why invites live in their own table](#why-invites-live-in-their-own-table-not-on-business_members-or-users).

**How the three real-world situations resolve, without three separate code
paths:**
- **The invitee doesn't have an account yet.** They click the link, the
  frontend shows a registration form (pre-filled from `GET /invites/{token}`),
  they call the ordinary `POST /auth/register` — completely unmodified,
  unaware an invite is even involved — and are now authenticated. The
  frontend then calls `POST /invites/{token}/accept` with their brand-new
  access token.
- **The invitee has an account and is already signed in** (a session from
  using the product for a different business, or one they never logged out
  of). The frontend calls `POST /invites/{token}/accept` immediately with the
  access token it already has. No branching needed — the endpoint only cares
  that *some* valid, currently-authenticated caller is presenting the token,
  regardless of which business their token happens to be scoped to right now.
- **The invitee has an account but isn't currently signed in.** They click
  the link, the frontend shows a login form (again pre-filled from
  `GET /invites/{token}`), they call the ordinary `POST /auth/login` —
  unmodified — and are now authenticated. Same accept call as above follows.

**The email-match check is the only thing standing between this design and a
real vulnerability.** Without it, anyone holding a valid invite token — a
forwarded email, a leaked link — could authenticate as *themselves* and
redeem someone else's invite, joining the business under their own account
with the role meant for someone else. The token proves "this reached an
inbox," not "the caller is who the invite was for" — proving *that* is what
`Register`/`Login` already do, which is exactly why accept always demands a
real, already-authenticated session rather than ever trusting the token
alone to establish identity.

*(Optional, not required for the core flow: a `GET /invites` — "my pending
invites by email" — for an already-signed-in user who hasn't clicked the
emailed link yet, e.g. to show an in-app notification. This is a trivial
addition later — `email` is already a plain column on `invites` — and is left
out of the initial build to keep the surface area minimal.)*

### Authorization Checks (Service Layer)

Every protected operation validates that the resource being acted on belongs
to the business/location in the caller's JWT — never trusts a business/location
ID passed in the request body or URL for scoping. This applies to **both**
`business_id` and `location_id`, without exception: there is no service
function that accepts a `locationId` parameter from a caller and uses it to
decide what the caller is allowed to touch. It is always read from
`ctxkeys.GetLocationId(ctx)`.

The only place a foreign ID legitimately appears in a request is when it
identifies some *other* resource being referenced — `position_id` in a shift
body, the `user_id` being assigned a shift, the `user_id` whose positions are
being changed. Every one of those must be independently validated against the
caller's own business/location before use. Accepting a foreign-business ID and
silently succeeding on it is exactly the class of bug this rule exists to
prevent.

### Two Authorization Patterns

Not every location-adjacent action is location-*scoped* in the same way, and
using one authorization helper for both produced real bugs (an admin-only
session being allowed to touch shifts; a manager being able to reach into
another location's roster). Two helpers, used for two different kinds of
resource:

**`requireLocationRole(ctx, roles...)`** — for resources that genuinely belong
to one specific location (shifts). Zero DB calls: role and location are
already-verified JWT claims.

```go
// requireLocationRole enforces a location-scoped session — rejecting
// identity-only AND admin-only sessions, since neither has an active
// location — and that the caller's role is one of `roles` unless they're an
// admin. Use for resources that are themselves tied to one location.
func requireLocationRole(ctx context.Context, roles ...domain.LRole) (locationId string, err error) {
    locationId = ctxkeys.GetLocationId(ctx)
    if locationId == "" {
        return "", domain.ErrUnauthorized
    }
    if ctxkeys.GetIsAdmin(ctx) {
        return locationId, nil
    }
    if slices.Contains(roles, ctxkeys.GetRole(ctx)) {
        return locationId, nil
    }
    return "", domain.ErrUnauthorized
}
```

A shift genuinely belongs to a location, so an admin needs an active location
in their own session to touch one — same as anyone else. This is what makes
`CreateShift` no longer need a caller-supplied `locationId` *or* a DB call to
validate one: the location is `requireLocationRole`'s return value, already
guaranteed to belong to the caller's business because it came from their own
token.

**`requireAdminOrManagerAtOwnLocation(ctx)`** — for resources that are
business-wide in nature, but where a Manager is delegated authority over their
own slice of them (employee position assignment). Also zero DB calls for the
caller's own standing; admins pass regardless of location tier, because the
resource itself (a position) isn't location-scoped.

```go
// requireAdminOrManagerAtOwnLocation enforces business-wide admin access, or
// a Manager acting within their own location. Unlike requireLocationRole, an
// admin-only session (no location) is allowed here — positions are a
// business-wide concept, only a Manager's authority over them is
// location-limited. Returns the caller's own locationId when the caller is a
// non-admin Manager (empty string for admins), for use in a follow-up check
// against the *target* resource.
func requireAdminOrManagerAtOwnLocation(ctx context.Context) (callerLocationId string, err error) {
    if ctxkeys.GetIsAdmin(ctx) {
        return "", nil
    }
    if ctxkeys.GetRole(ctx) != domain.Manager {
        return "", domain.ErrUnauthorized
    }
    locationId := ctxkeys.GetLocationId(ctx)
    if locationId == "" {
        return "", domain.ErrUnauthorized // a Manager session always has one; fail closed if not
    }
    return locationId, nil
}
```

When this returns a non-empty `callerLocationId` (the caller is a Manager, not
an admin), the service layer still needs one more check before proceeding:
does the *target* employee actually work at that location? That's a fact
about someone else's data, not a re-derivation of the caller's own
already-verified claims — so, unlike the role checks this section replaces, it
is a legitimate DB call (`GetLocationRole(ctx, targetUserId, callerLocationId)`),
not the wasteful kind this redesign removes.

### Why JWT Claims Instead of Per-Request DB Lookups

Business and location membership are queried once, at auth time (login,
refresh, select-business, select-location), to build the token. Every
subsequent request within that token's lifetime trusts `business_id`,
`location_id`, `is_admin`, and `role` from the claims rather than re-querying
`business_members`/`location_roles` to find out the *caller's own* standing —
that's what `requireLocationRole`/`requireAdminOrManagerAtOwnLocation` do, in
one line, with zero DB round trips.

Two categories of DB call remain in an authorization path, and both are
legitimate — neither is what this section is about:
1. Fetching the target resource itself (a shift, a location) to confirm it
   belongs to the caller's business/location — unavoidable, since you need the
   row's data anyway to act on it.
2. Fetching a fact about a *different* entity than the caller — e.g. does the
   employee a Manager is trying to reassign actually work at the Manager's own
   location. This is data the caller's own token can't possibly contain.

This keeps authorization fast at the cost of a 15-minute staleness window on
role changes, which is acceptable for a scheduling SaaS — see
[Why There Is No Per-Request Role Revocation](#why-there-is-no-per-request-role-revocation-no-blocklist).

### What Each Table Is Actually Used For

| Table | Used for |
|---|---|
| `business_members` | Auth-time: which businesses, admin status |
| `location_roles` | Auth-time: which locations, role. Also: verifying a *target* employee's location when a Manager acts on them (not the caller's own role) |
| `invites` | Invite-flow only: create/preview/accept by token. Never queried at normal auth time — `business_members`/`location_roles` are the only source of truth once an invite has been accepted |
| Everything else | Normal request-time reads/writes, scoped by JWT claims |

---

## API Design & Routing Conventions

Two independent questions determine how a piece of identifying information is
carried in a request. Conflating them is what produced the "not RESTful"
problem this section fixes.

1. **Scope** — which business/location am I allowed to act within? Always
   from the JWT (`ctxkeys`). Never from the URL, query string, or body — no
   exceptions, including `location_id`.
2. **Resource identity** — which specific location/position/shift am I
   fetching or changing? Always a URL path segment. Never a body field. This
   isn't a security rule (a resource's own ID isn't sensitive the way scope
   is) — it's what makes a route self-describing, and it's what a plain `GET`
   with no body actually needs.

`POST /invites` is a direct application of rule 1: an admin can only ever
invite someone into *their own* business, so `business_id` has no business
being a client-supplied field at all — it comes from the JWT like every other
admin-only write. `location_id` stays in the body, though, because it's rule
2's kind of value here: which *target* location (among possibly several in
the caller's business) the invite is for, not a claim about the caller's own
scope. `POST /invites/{token}/accept` puts the token in the URL for the same
reason any other resource id is a path segment — `{token}` identifies which
invite is being acted on, exactly like `{id}` does for a location or shift.

```
GET    /business                          singleton: caller's own business (JWT)
PATCH  /business
DELETE /business

GET    /locations                         list, business from JWT
POST   /locations
GET    /locations/{id}
PATCH  /locations/{id}
DELETE /locations/{id}

GET    /positions
POST   /positions
GET    /positions/{id}
PATCH  /positions/{id}
DELETE /positions/{id}

GET    /shifts                            location implicit: "my session's location"
POST   /shifts
GET    /shifts/{id}
PATCH  /shifts/{id}
DELETE /shifts/{id}
POST   /shifts/{id}/assign      { user_id }
POST   /shifts/{id}/unassign

GET    /employees                         business members + location roles, business from JWT
POST   /employees/{user_id}/positions     { position_id }
DELETE /employees/{user_id}/positions/{position_id}
GET    /employees/{user_id}/positions

POST   /invites                           { email, location_id, role } — business from JWT, not the body
GET    /invites/{token}                   public preview, no auth required
POST   /invites/{token}/accept            authenticated (any tier); no body — token is the resource id

POST   /auth/register
POST   /auth/login
POST   /auth/select-business
POST   /auth/select-location
POST   /auth/refresh
POST   /auth/logout

POST   /businesses                        { name } — create an additional business
```

`chi.URLParam(r, "id")` is where a target resource's own ID comes from —
never a JSON body field.

---

## Authorization Matrix

| Action | Admin (any tier) | Manager | Employee |
|---|---|---|---|
| Create business | ✓ (any session) | ✓ (any session) | ✓ (any session) |
| Manage business / rename / delete | ✓ | ✗ | ✗ |
| Create/edit/delete locations | ✓ | ✗ | ✗ |
| Create/rename/delete positions | ✓ | ✗ | ✗ |
| Add/remove employees from business, send invites | ✓ | ✗ | ✗ |
| Assign/remove an employee's positions | ✓ (any tier, business-wide) | ✓ (own location's employees only) | ✗ |
| Create/Update/Cancel shift | ✓ (location-scoped session) | ✓ | ✗ |
| Assign/Unassign shift | ✓ (location-scoped session) | ✓ | ✗ |
| Delete shift (hard) | ✓ (location-scoped session) | ✗ | ✗ |
| Get shift / Get shifts by location | ✓ (location-scoped session) | ✓ | ✓ |

Admin-only sessions (no location selected) can do everything above the shift
rows, **including** assigning employee positions — that's a business-wide
administrative action, not a location-scoped one (see
[Two Authorization Patterns](#two-authorization-patterns)). Shifts are
different: a shift belongs to one specific location, so every shift operation
requires the caller to have an active location in their session, admin or
not. This is enforced structurally by `requireLocationRole` rejecting an empty
`ctxkeys.GetLocationId(ctx)` before any role comparison runs, for shift
operations only — not the same check used for position assignment.

The Manager row for position assignment means: the service layer verifies the
*target* employee has a `location_roles` row at the Manager's own session
location, not merely that they belong to the same business. A Manager at
Location X cannot assign positions to someone who only works at Location Y.

---

## Data Validation Rules

| Entity | Rule |
|---|---|
| User email | Must match RFC 5322 pattern; normalized to lowercase before storage/comparison (applies to Register, Login, and invite email matching alike) |
| User password | Minimum 8 characters; required at Register time |
| User name | Cannot be empty or whitespace; required at Register time |
| Business name | Cannot be empty or whitespace |
| Location name | Cannot be empty or whitespace |
| Position name | Cannot be empty or whitespace; unique per business |
| Shift times | end_time must be after start_time (enforce in service) |
| Invite email | Must match RFC 5322 pattern; normalized to lowercase |
| Invite role | Must be a legal `location_role` value |
| Invite token | Single-use; expires after 7 days; stored as a SHA256 hash (raw token never persisted, same treatment as refresh tokens); scoped to one `invites` row, independent of any `users`/`business_members` row |

A partial update (`RenameBusiness`, `UpdateLocation`, `RenamePosition`, ...)
validates any field it changes with the exact same rule the corresponding
`Create` uses. "It's optional" is not the same as "it's exempt from
validation" — a PATCH that sets a name to `"   "` should fail exactly like a
POST would.

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
- Last-admin removal is guarded against, using row-level locking that doesn't
  hit Postgres's restriction on combining `FOR UPDATE` with an aggregate (see
  implementation note below)
- No membership is ever created in an already-active state on someone else's
  say-so, and — since `CreateInvite` doesn't write to `business_members` or
  `users` at all — there is no pending-but-unconsented row of any kind
  sitting in either table for an admin's invite to have silently created.
  The only row an invite creates before acceptance lives in `invites`,
  keyed by email, with no reference to any identity
- Accepting an invite requires the authenticated caller's own email to match
  the invite's email (case-insensitive) — a valid invite token proves an
  email was reachable, not that the bearer is the account it belongs to;
  identity is established the normal way, by `Register`/`Login`, before
  `accept` ever runs
- Invite tokens are stored as a SHA256 hash, never the raw value — same
  treatment as refresh tokens
- Multi-step login (`select-business`/`select-location`) requires a signed
  access token proving the caller already passed password verification — a
  bare `user_id` is never sufficient to obtain tokens. The access token
  minted mid-selection carries no refresh token, so a leaked one is bounded
  by its own 15-minute expiry with no way to extend it — see
  [Why There's Only One Access Token Type](#why-theres-only-one-access-token-type)
- `select-location` independently verifies the target location belongs to the
  requested business before minting a token — `business_id` and `location_id`
  inside a token are never allowed to be inconsistent with each other

**Implementation gotcha worth documenting where it'll be seen**: Postgres
rejects `SELECT COUNT(*) ... FOR UPDATE` outright — `FOR UPDATE` cannot be
combined with an aggregate function. The last-admin-removal guard needs to
lock rows directly and count in application code, or wrap the aggregate in a
subquery over an already-locked row set:

```sql
SELECT COUNT(*) FROM (
  SELECT 1 FROM business_members
  WHERE business_id = $1 AND is_admin = TRUE
  FOR UPDATE
) locked_admins
```

---

## Key Decisions & Rationale

### Why identity, business ownership, and membership are separate operations
Registration previously created a user and a business atomically, which meant
there was no way to create a second business, or to onboard someone who was
invited before they'd ever signed up, without special-casing each combination.
Splitting these into `Register` (identity only), `CreateBusiness` (available
to any session, any time), and `CreateInvite`/accept (a membership grant) means
every real-world ordering — register-then-invite, invite-then-register,
already-active-user-gets-a-second-business — is the same small set of
operations composed differently, not a new code path each time.

### Why invites live in their own table, not on `business_members` or `users`
An earlier revision of this design scoped the invite token to a
`business_members` row, created up front at invite time with `accepted_at:
NULL`. That required a `users` row to attach the membership to — so inviting
someone with no account yet meant creating one on their behalf: a "dummy"
identity with `name`/`password` left `NULL`, existing purely because an admin
typed an email into a form. Two real problems followed directly from that:

1. **Privacy.** A row in the primary identity table for a person who never
   chose to have an account, queryable by anything that queries `users`,
   created without their knowledge or consent.
2. **Orphaned rows.** If the invite was never accepted, that user row (and
   its pending `business_members` row) simply stayed in the database forever
   — nothing about the design gave either row a natural lifecycle or expiry.

Keying the invite by **email alone**, in a table with no reference to
`users`, removes both problems structurally instead of mitigating them:
nothing is ever written to `users` until someone completes `Register`
themselves, and an unaccepted invite is just a token+email+expiry that goes
stale on its own — not a phantom account.

The original concern this design was solving for — concurrent invites from
different businesses shouldn't be able to touch each other — still holds,
just for a simpler reason. There's no per-user identity row for two invites
to contend over in the first place; each is an independent `invites` row
with its own token, and accepting one only ever touches that one row.

This also happens to be why the "brand new / already have an account /
signed in vs. not" distinction that originally motivated two separate accept
endpoints stops being the backend's problem at all: identity is entirely
handled by `Register`/`Login`, unmodified, before an accept call ever runs.
See [Invite & Consent Model](#invite--consent-model).

### Why `location_roles` doesn't need its own consent state
Consent matters at the boundary of "are you part of this business at all."
Once that's been accepted, an admin reassigning someone's location or role
within the *same* business is ordinary internal management, not a new trust
boundary — the same norm most workspace-style products use (a Slack admin can
move you between channels without re-asking permission to be in the
workspace).

### Why a `users` row is always fully formed
An earlier design allowed `password IS NULL`/`name IS NULL` to represent an
identity created as a side effect of being invited, before the person had
ever provided either. That state no longer exists: `CreateInvite` never
writes to `users`, so the only path that creates a row there is `Register`,
which always has both. "Has this person claimed their account" is no longer
a fact to check — every `users` row already answers yes, by construction.

### Why location context is in the JWT
A user session is always scoped to one location at a time (when it's scoped
at all). Storing location in the token means zero DB calls for the caller's
own authorization standing on every request. Switching location requires a
re-auth, which is natural UX for a scheduling app (select your work location
when you start).

### Why `business_members` and `location_roles` are only queried at auth time (plus one narrow exception)
They exist to build and refresh the token, not to gate individual requests
based on the *caller's own* role — the token is the materialized result of
those table queries, and re-querying them to re-derive the caller's own
standing would be redundant and slower. The one exception —
`location_roles` looked up for a *target* employee when a Manager assigns
positions — isn't re-deriving the caller's authorization; it's reading a fact
about someone else that the caller's own token can't contain by definition.

### Why positions are business-scoped not location-scoped
A position like "Cashier" or "Supervisor" is a business concept. The same
position name applies across all locations. This is also why an admin-only
session (no location) can still assign positions — the resource itself has no
location to be scoped to.

### Why refresh tokens store nullable `business_id` and `location_id`
When rotating a refresh token, the new token must be issued with the same
scope it started with — identity-only, admin-only, or fully scoped. Storing
both IDs (nullable, matching the JWT's own sentinel pattern) lets the refresh
endpoint reconstruct exactly the right tier without requiring the user to
re-select anything.

### Why select-business/select-location require a signed access token, not a bare user ID
Covered above in
[Why select-business/select-location Still Require a Real Access Token, Not a Bare User ID](#why-select-businessselect-location-still-require-a-real-access-token-not-a-bare-user-id) —
repeated here only to keep this section a complete index of decisions. The
short version: a user ID is unique but not secret, and an endpoint that mints
real tokens off nothing but a client-supplied ID is authenticating on a
lookup, not on proof of identity.

### Why select-business/select-location read userId from context, not a service parameter
An earlier draft of this fix had the handler parse the token itself and pass
`claims.UserId` into `SelectBusiness`/`SelectLocation` as an explicit
argument. That works, but it's the only two service functions in the
codebase that would need to know a JWT exists — every other protected
service already gets its caller's identity and scope from
`ctxkeys`/context, populated once by a middleware, and never touches a token
directly. Since these two routes now sit behind the exact same
`AccessAuthMiddleware` as everything else — see
[Why There's Only One Access Token Type](#why-theres-only-one-access-token-type)
— this is no longer even a special case to explain: `ctxkeys.GetUserId(ctx)`
is populated the same way it is everywhere. `Login`'s internal call to
`SelectBusiness` (which never touches HTTP, so there's no middleware to run)
seeds the same context key by hand: `context.WithValue(ctx, ctxkeys.UserId,
u.Id)`. One consistent mechanism for "where does identity/scope come from"
across the whole codebase, rather than
an exception carved out for these two routes.

### Why locationId is never a caller-supplied parameter, without exception
`businessId` was always handled this way; `locationId` wasn't, in the
shift/employee-position endpoints specifically, and that one inconsistency
produced three separate problems at once: it needed a DB round trip to
validate on every request (defeating the point of putting role/location in
the JWT), it let request URIs/bodies carry an ID that should have been pure
session state (the "not RESTful" symptom), and it opened a path for a caller
to name a location their own session doesn't actually match. Once the two
authorization helpers above correctly gate every location-adjacent action,
there's no remaining legitimate case where a caller needs to supply a
different location than the one already in their token — so the parameter is
removed, not defended against.

### Why resource IDs live in the URL path, not the request body
This is a distinct question from where *scope* comes from. Scope
(business/location) is session state and belongs in the JWT, never in the URL
— that's the rule above. But the identity of the *specific resource* a
GET/PATCH/DELETE targets isn't session state, isn't sensitive, and is exactly
what a URL path segment exists to carry. `GET /locations/{id}` isn't a
security downgrade from putting `id` in the body; it's what makes the route
self-describing, and it's what a plain `GET` (which conventionally carries no
body at all) actually needs.

### Why there is no per-request role revocation (no blocklist)
Access tokens are short-lived (15 minutes). Role changes take effect at the
next token refresh. This staleness window is acceptable for a scheduling
SaaS. A token blocklist (Redis-based) can be added later if instant
revocation becomes a customer requirement.
