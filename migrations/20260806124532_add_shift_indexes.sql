-- +goose Up
-- The schedule UI fetches one week at a time via
-- GetShiftDetailsByLocationId, which filters on location_id plus a
-- start_time/end_time range — without this the query is a seq scan over every
-- shift the location has ever had.
CREATE INDEX shifts_location_start_idx ON shifts (location_id, start_time);

-- Serves the double-booking check (GetOverlappingShiftsForUser), which looks
-- up one user's shifts in a time window. Partial: unassigned shifts have a
-- NULL user_id and are never a booking conflict for anyone.
CREATE INDEX shifts_user_start_idx ON shifts (user_id, start_time) WHERE user_id IS NOT NULL;

-- +goose Down
DROP INDEX shifts_user_start_idx;
DROP INDEX shifts_location_start_idx;
