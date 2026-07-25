-- +goose Up
CREATE TYPE shift_status AS ENUM ('draft', 'assigned', 'uncovered', 'covered', 'cancelled');

CREATE TABLE shifts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
  position_id UUID NOT NULL REFERENCES positions(id),
  status shift_status NOT NULL,
  start_time TIMESTAMPTZ NOT NULL,
  end_time TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE shifts;
DROP TYPE shift_status;
