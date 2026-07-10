-- +goose Up
CREATE TABLE refresh_tokens (
  id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  business_id UUID REFERENCES businesses(id) ON DELETE CASCADE,  -- nullable: identity-only sessions
  location_id UUID REFERENCES locations(id) ON DELETE CASCADE,   -- nullable: admin-only sessions
  token TEXT NOT NULL UNIQUE, -- SHA256 hash of the actual token
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE refresh_tokens;
