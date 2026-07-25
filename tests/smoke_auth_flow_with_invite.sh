#!/usr/bin/env bash
set -euo pipefail
BASE="http://localhost:$API_PORT"

# --- cleanup from any previous run, so this is safe to re-run ---
docker compose exec postgres psql -U postgres -d "$DATABASE_URL" -c "DELETE FROM businesses WHERE name = 'Test Co';" > /dev/null
docker compose exec postgres psql -U postgres -d "$DATABASE_URL" -c "DELETE FROM users WHERE email IN ('owner@test.com','employee@test.com');" > /dev/null
# businesses cascades away locations/positions/shifts/business_members/
# location_roles/employee_positions/invites tied to it; users cascades away
# refresh_tokens. Order matters: business first, then users.

echo "== register admin =="
REGISTER_RESP=$(curl -s -X POST "$BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"owner@test.com","name":"Owner","password":"testpass123"}')
echo "$REGISTER_RESP" | jq .
ACCESS=$(echo "$REGISTER_RESP" | jq -r .access_token)
[ "$ACCESS" != "null" ] || { echo "FAILED: register"; exit 1; }

echo "== create business =="
CREATE_RESP=$(curl -s -X POST "$BASE/businesses" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"business_name":"Test Co"}')
echo "$CREATE_RESP" | jq .
ACCESS=$(echo "$CREATE_RESP" | jq -r .access_token)
[ "$ACCESS" != "null" ] || { echo "FAILED: create business"; exit 1; }

echo "== create location =="
LOCATION_RESP=$(curl -s -X POST "$BASE/locations" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"name":"Test Location","address":"123 Main St"}')
echo "$LOCATION_RESP" | jq .
LOCATION_ID=$(echo "$LOCATION_RESP" | jq -r .location.id)
[ "$LOCATION_ID" != "null" ] || { echo "FAILED: create location"; exit 1; }

echo "== create invite =="
INVITE_RESP=$(curl -s -X POST "$BASE/invites" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d "{\"email\":\"employee@test.com\",\"location_id\":\"$LOCATION_ID\",\"target_role\":\"employee\"}")
echo "$INVITE_RESP" | jq .
TOKEN=$(echo "$INVITE_RESP" | jq -r .invite_token)
[ "$TOKEN" != "null" ] || { echo "FAILED: create invite"; exit 1; }

echo "== preview invite (no auth) =="
curl -s "$BASE/invites/$TOKEN" | jq .

echo "== register invitee, accept invite =="
INVITEE_ACCESS=$(curl -s -X POST "$BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"employee@test.com","name":"Employee","password":"testpass123"}' | jq -r .access_token)

curl -s -X POST "$BASE/invites/$TOKEN/accept" \
  -H "Authorization: Bearer $INVITEE_ACCESS" | jq .

echo "== ALL PASSED =="
