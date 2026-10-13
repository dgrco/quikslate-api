-- +goose Up
ALTER TABLE locations
  ADD CONSTRAINT locations_id_business_id_key UNIQUE (id, business_id);

ALTER TABLE location_roles
  DROP CONSTRAINT location_roles_location_id_fkey;

ALTER TABLE location_roles
  ADD CONSTRAINT location_roles_location_business_fkey
  FOREIGN KEY (location_id, business_id)
  REFERENCES locations (id, business_id)
  ON DELETE CASCADE;

ALTER TABLE invites
  DROP CONSTRAINT invites_location_id_fkey;

ALTER TABLE invites
  ADD CONSTRAINT invites_location_business_fkey
  FOREIGN KEY (location_id, business_id)
  REFERENCES locations (id, business_id)
  ON DELETE CASCADE;

-- +goose Down
ALTER TABLE invites
  DROP CONSTRAINT invites_location_business_fkey;

ALTER TABLE invites
  ADD CONSTRAINT invites_location_id_fkey
  FOREIGN KEY (location_id) REFERENCES locations (id)
  ON DELETE CASCADE;

ALTER TABLE location_roles
  DROP CONSTRAINT location_roles_location_business_fkey;

ALTER TABLE location_roles
  ADD CONSTRAINT location_roles_location_id_fkey
  FOREIGN KEY (location_id) REFERENCES locations (id)
  ON DELETE CASCADE;

ALTER TABLE locations
  DROP CONSTRAINT locations_id_business_id_key;
