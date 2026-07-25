-- +goose Up
CREATE TABLE invites (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash TEXT NOT NULL UNIQUE, -- SHA256 hash of the actual token (same as refresh tokens)
  email TEXT NOT NULL, -- ensure lowercased before invite
  business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
  role location_role NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ, -- NULL = still pending
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_invite_pending_unique
ON invites (email, business_id)
WHERE accepted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_invite_pending_unique;
DROP TABLE invites;
