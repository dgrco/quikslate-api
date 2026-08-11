-- +goose Up
-- IANA timezone name (e.g. 'America/New_York'). Shifts are stored as
-- TIMESTAMPTZ (absolute instants), so this is purely how a location's
-- schedule should be *rendered* — a manager in one zone scheduling a store in
-- another must see the store's local hours, not their own.
-- Defaults to UTC so existing rows stay valid; the app validates any value it
-- writes against Go's tzdata (see domain.ValidateTimezone).
ALTER TABLE locations ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';

-- +goose Down
ALTER TABLE locations DROP COLUMN timezone;
