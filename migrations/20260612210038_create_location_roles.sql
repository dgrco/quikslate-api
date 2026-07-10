-- +goose Up
CREATE TYPE location_role AS ENUM ('location_lead', 'manager', 'employee');

CREATE TABLE location_roles (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  business_id UUID NOT NULL REFERENCES businesses(id) ON DELETE CASCADE,
  location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
  role location_role NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, location_id)
);

-- +goose Down
DROP TABLE location_roles;
DROP TYPE location_role;
