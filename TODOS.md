# Quikslate API — To-Dos

## 2. Finish the invite feature

- [x] **One type fix before building on top of it:** `domain.Invite.AcceptedAt`
      is `time.Time`, not `*time.Time`. The DB column is nullable (`NULL` =
      pending, per the migration) and the repo's scan already handles that
      correctly with a local pointer — but since the struct field itself
      isn't a pointer, an unaccepted invite reads back as the Go zero time
      rather than an absent value. Works today only because nothing checks
      it yet; whoever writes `AcceptInvite` next needs an explicit way to
      ask "is this accepted." Change it to `*time.Time` now, before that
      code gets written against the wrong assumption.

- [x] **Rewrite `AcceptInvite` — the current stub is the wrong shape, not a
      partial version of the right one.** `service/auth.go` currently has:
      ```go
      func (s *AuthService) AcceptInvite(token, name, password string) {
          // TODO:
      }
      ```
      Replace entirely with:
      ```go
      func (s *AuthService) AcceptInvite(ctx context.Context, token string) (*AuthResponse, error) {
          userId := ctxkeys.GetUserId(ctx)

          inv, err := s.repo.GetInviteByTokenHash(ctx, hashToken(token))
          if err != nil {
              return nil, domain.ErrNotFound
          }
          if inv.AcceptedAt != nil || time.Now().After(inv.ExpiresAt) {
              return nil, domain.ErrNotFound // same response either way — don't distinguish
          }

          caller, err := s.repo.GetUserById(ctx, userId)
          if err != nil {
              return nil, err
          }
          if !strings.EqualFold(caller.Email, inv.Email) {
              return nil, domain.ErrForbidden // invite wasn't issued to this account
          }

          if err := s.repo.AddUserToBusiness(ctx, userId, inv.BusinessId, false); err != nil {
              return nil, err
          }
          if err := s.repo.AssignRole(ctx, userId, inv.BusinessId, inv.LocationId, inv.Role); err != nil {
              return nil, err
          }
          if _, err := s.repo.MarkInviteAccepted(ctx, hashToken(token)); err != nil {
              return nil, err
          }

          return s.SelectBusiness(ctx, inv.BusinessId) // mints correctly-tiered tokens, reuses existing logic
      }
      ```
      No `name`/`password` params — those belong to `Register`/`Login`,
      which the caller already went through to get here. Requires
      `AccessAuthMiddleware` (any tier, including identity-only — someone
      who just registered specifically to accept this has nothing else).

- [x] **`business_members_repo.go`: `AddUserToBusiness` needs
      `ON CONFLICT (user_id, business_id) DO NOTHING`** on its `INSERT` —
      without it, the call above throws a unique-constraint error for
      someone accepting a second invite to a business they're already in,
      instead of silently succeeding.

- [ ] **Write `CreateInvite`** (new file, e.g. `service/invite.go`):
      admin-only (`validateIsAdmin`); `businessId` from
      `ctxkeys.GetBusinessId(ctx)` — **not a request parameter**, an admin
      can only invite into their own business; validate email
      (`domain.ValidateEmail`, then `domain.NormalizeEmail`) and role
      (`domain.ValidateLocationRole` — note it accepts `EmptyRole` for other
      callers, so reject that case specifically here, an invite must carry
      Manager or Employee); confirm `locationId` belongs to the caller's
      business via the existing `getAndValidateLocation`; if
      `GetUserByEmail` finds an existing user who already has a
      `GetBusinessMember` row in this business, return `ErrAlreadyExists`;
      generate a token (`generateSecureToken()`), hash it (`hashToken()`),
      call `repo.CreateInvite`; send the email with the raw token in the
      link (raw token is never stored — this is the only place it exists).

- [ ] **Write `GetInvitePreview(ctx, token) (email, businessName, role,
      expiresAt, error)`** — public, no auth. `GetInviteByTokenHash`, then
      `GetBusinessById(inv.BusinessId)` for the name. 404 for
      missing/expired/already-accepted, same as `AcceptInvite`.

- [ ] **New `handler/invite.go`:**
      `POST /invites` (admin, `AccessAuthMiddleware`) → `CreateInvite`
      `GET /invites/{token}` (public, no middleware) → `GetInvitePreview`
      `POST /invites/{token}/accept` (`AccessAuthMiddleware`, no body) → `AcceptInvite`
      Wire all three into `cmd/server/main.go`.

---

## 3. `CreateBusiness` — small, standalone, currently missing entirely

Neither the service method nor the route exists yet (`service/business.go`
has Get/Rename/Delete only).

- [ ] `service/business.go`: `CreateBusiness(ctx, userId, name)` — any
      authenticated session, any tier, any number of existing businesses;
      `domain.ValidateBusinessName`; insert `businesses` +
      `business_members(is_admin: true)` — no acceptance step needed.
- [ ] `handler/business.go`: `POST /businesses`, `AccessAuthMiddleware`.

---

## 4. Remaining resource handlers

- [ ] `handler/location.go` — `CreateLocation` is written but not wired to
      a route (add `r.Post("/", h.CreateLocation)`); `DeleteLocation`
      handler doesn't exist yet (service method does — `DeleteLocation` in
      `service/location.go` is ready to call). Get/GetAll/Update are done
      and correctly wired.
- [ ] `handler/position.go` (new) — create, get, get all, rename, delete
- [ ] `handler/shift.go` (new) — create, get, get by location (`GET
      /shifts`, location implicit from session), update, assign, unassign,
      cancel, delete
- [ ] `handler/employee.go` (new) — add position, remove position, get all
      positions by user
- [ ] `cmd/server/main.go` — remove the placeholder `r.Get("/", ...)` route;
      wire each handler above as it's built

---

## 5. Small validation/consistency gaps

Batch these together whenever convenient — each is a one-line fix.

- [ ] `service/business.go` — `RenameBusiness` doesn't call
      `domain.ValidateBusinessName` on the new name
- [ ] `service/location.go` — `UpdateLocation` doesn't validate `update.Name`
      when present
- [ ] `domain` — add `ValidatePositionName`; call it from `CreatePosition`
      and `RenamePosition` in `service/position.go`
- [ ] `infra/repo/positions_repo.go` — `ChangePositionName` doesn't map a
      unique-constraint violation to `ErrAlreadyExists` the way
      `CreatePosition` does one line above it
- [ ] `service/employee.go` — copy-paste error strings: the
      target-location-check block inside `RemovePosition` and
      `GetAllPositionsByUser` both say `"failed to add position to
      employee"` instead of matching their own function name

---

## 6. Later (production readiness)

- [ ] Pagination on list endpoints (`GetAllLocations`, `GetShiftsByLocation`, etc.)
- [ ] Structured logging (`slog`/`zap` instead of `log.Println`)
- [ ] `sslmode=require` in the database URL outside local dev
- [ ] CORS configuration for the frontend origin
- [ ] Email sending integration (Resend/Postmark/SES) — needed before
      `CreateInvite` sends real emails; stub/log the link until then

---

## Design Notes

**Admin + Location access**
- Admin-only session (`locationId = ""`) → business, locations, positions,
  employees, employee-position assignments. Not shifts.
- Admin + location session → adds shifts at that location.
- Frontend should call `POST /auth/select-location` silently when an admin
  opens a specific location's schedule — swap the token, no visible step.

**Refresh token nullable location/business**
- Both `location_id` and `business_id` are nullable in `refresh_tokens`.
  Domain/service layer uses `""` as the sentinel; conversion to/from SQL
  `NULL` happens only at the repo boundary. Never scan a nullable column
  directly into a plain (non-pointer) struct field — see bug #1 above for
  what happens when that's missed on the write side too.

**Invites are keyed by email, not `user_id`**
- `CreateInvite` never touches `users` or `business_members` — only
  `invites`. Both real rows get created together, at accept time, for
  whichever `user_id` the *authenticated caller* turns out to have. See
  `SDD.md`, "Invite & Consent Model," for the full reasoning.
- The accept step's one real security check: caller's own email must match
  the invite's email (case-insensitive). A token proves an email was
  reachable, not that the caller owns it.

**Two authorization helpers, not one**
- `requireLocationRole` — resources tied to one location (shifts). Rejects
  admin-only sessions.
- `requireAdminOrManagerAtOwnLocation` — business-wide resources a Manager
  is delegated authority over (employee positions). Allows admin-only.
- Picking the wrong one for a new endpoint reintroduces either the
  admin-only-sessions-touching-shifts bug or unnecessary admin friction.

**No separate token type for multi-step auth**
- `Login`/`SelectBusiness` mint an ordinary access token even when
  returning `RequiresBusinessSelection`/`RequiresLocationSelection` — same
  `AccessTokenClaims` shape, just no refresh token minted alongside it. See
  `SDD.md`, "Why There's Only One Access Token Type."
