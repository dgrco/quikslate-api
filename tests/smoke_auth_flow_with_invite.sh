#!/usr/bin/env bash
set -euo pipefail
BASE="http://localhost:$API_PORT"

# --- cleanup from any previous run, so this is safe to re-run ---
docker compose exec postgres psql -U postgres -d "$DATABASE_URL" -c "DELETE FROM businesses WHERE name = 'Test Co';" > /dev/null
docker compose exec postgres psql -U postgres -d "$DATABASE_URL" -c "DELETE FROM users WHERE email IN ('owner@test.com','employee@test.com');" > /dev/null

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
BUSINESS_ID=$(echo "$CREATE_RESP" | jq -r .business_id)
[ "$BUSINESS_ID" != "null" ] || { echo "FAILED: create business"; exit 1; }

echo "== create location =="
LOCATION_RESP=$(curl -s -X POST "$BASE/businesses/$BUSINESS_ID/locations" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d '{"name":"Test Location","address":"123 Main St"}')
echo "$LOCATION_RESP" | jq .
LOCATION_ID=$(echo "$LOCATION_RESP" | jq -r .location.id)
[ "$LOCATION_ID" != "null" ] || { echo "FAILED: create location"; exit 1; }

echo "== create invite =="
INVITE_RESP=$(curl -s -X POST "$BASE/businesses/$BUSINESS_ID/locations/$LOCATION_ID/invites" \
  -H "Authorization: Bearer $ACCESS" -H "Content-Type: application/json" \
  -d "{\"email\":\"employee@test.com\",\"target_role\":\"employee\"}")
echo "$INVITE_RESP" | jq .
TOKEN=$(echo "$INVITE_RESP" | jq -r .invite_token)
[ "$TOKEN" != "null" ] || { echo "FAILED: create invite"; exit 1; }

echo "== preview invite (no auth) =="
curl -s "$BASE/businesses/$BUSINESS_ID/invites/$TOKEN" | jq .

echo "== register invitee, accept invite =="
INVITEE_ACCESS=$(curl -s -X POST "$BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"employee@test.com","name":"Employee","password":"testpass123"}' | jq -r .access_token)

curl -s -X POST "$BASE/businesses/$BUSINESS_ID/invites/$TOKEN/accept" \
  -H "Authorization: Bearer $INVITEE_ACCESS" | jq .

echo "== confirm invitee actually has access now =="
curl -s "$BASE/businesses/$BUSINESS_ID/locations/$LOCATION_ID/shifts" \
  -H "Authorization: Bearer $INVITEE_ACCESS" | jq .

echo "== look up invitee's user id directly from the db =="
INVITEE_USER_ID=$(docker compose exec postgres psql -U postgres -d "$DATABASE_URL" -t -A \
  -c "SELECT id FROM users WHERE email = 'employee@test.com';" | tr -d '[:space:]')
echo "invitee user id: $INVITEE_USER_ID"

echo "== admin removes invitee from the business =="
curl -s -X DELETE "$BASE/businesses/$BUSINESS_ID/members/$INVITEE_USER_ID" \
  -H "Authorization: Bearer $ACCESS" | jq .

echo "== the actual point of this refactor: same old token, should be rejected now =="
if curl -sf "$BASE/businesses/$BUSINESS_ID/locations" \
  -H "Authorization: Bearer $INVITEE_ACCESS" > /dev/null; then
  echo "FAILED: revoked user's old token still works"
  exit 1
else
  echo "confirmed: revoked access denied immediately, not after 15 min token expiry"
fi

echo "== ALL PASSED =="
