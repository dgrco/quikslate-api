-- +goose Up
CREATE TABLE business_members (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  is_primary_admin BOOLEAN NOT NULL DEFAULT FALSE,
  is_admin BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, business_id)
);

CREATE UNIQUE INDEX idx_single_primary_admin_in_business
ON business_members (business_id)
WHERE is_primary_admin = TRUE;

-- +goose Down
DROP INDEX idx_single_primary_admin_in_business;
DROP TABLE business_members;
