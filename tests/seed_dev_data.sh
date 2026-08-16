#!/usr/bin/env bash
set -euo pipefail
# Seeds one demo business with two locations in different timezones, six
# positions, one account per role at each location, and a full week of
# shifts (including a couple of uncovered gaps), all driven through the
# HTTP API so registration validation and password hashing actually run
# (see NOTES.md Phase 3).
#
# Requires API_PORT and DATABASE_URL in the environment, same convention as
# tests/smoke_auth_flow_with_invite.sh. Safe to re-run: deletes its own
# previous output first (cascades to locations/positions/shifts via FK).
BASE="http://localhost:$API_PORT/v1"
PASSWORD="seedpass123"
SEED_EMAILS="owner@quikslate.dev,lead@quikslate.dev,manager@quikslate.dev,employee@quikslate.dev,employee2@quikslate.dev,employee3@quikslate.dev,westside.manager@quikslate.dev,westside.employee1@quikslate.dev,westside.employee2@quikslate.dev"

psql_c() {
  docker compose exec -T postgres psql -U postgres -d "$DATABASE_URL" -t -A -c "$1"
}

echo "== cleanup from any previous run =="
psql_c "DELETE FROM businesses WHERE name = 'QuikSlate Demo Co';" > /dev/null
psql_c "DELETE FROM users WHERE email IN ($(echo "$SEED_EMAILS" | sed "s/\([^,]*\)/'\1'/g"));" > /dev/null

register() {
  local email="$1" name="$2"
  curl -s -X POST "$BASE/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\",\"name\":\"$name\",\"password\":\"$PASSWORD\"}" | jq -r .access_token
}

user_id_of() {
  psql_c "SELECT id FROM users WHERE email = '$1';" | tr -d '[:space:]'
}

# local_to_utc TZ 'YYYY-MM-DD HH:MM:SS' -> UTC ISO8601.
# Deliberately two steps: parsing with TZ set (no -u) gets the local-time
# epoch right; -u only on the render step. Doing both in one `date -u -d`
# call silently drops the offset instead of converting it (see NOTES.md
# Phase 3's TZ gotcha).
local_to_utc() {
  local tz="$1" dt="$2" epoch
  epoch=$(TZ="$tz" date -d "$dt" +%s)
  date -u -d "@$epoch" +"%Y-%m-%dT%H:%M:%SZ"
}

echo "== register business admin =="
OWNER_ACCESS=$(register owner@quikslate.dev "Demo Owner")
[ "$OWNER_ACCESS" != "null" ] || { echo "FAILED: register owner"; exit 1; }

echo "== create business =="
BUSINESS_ID=$(curl -s -X POST "$BASE/businesses" \
  -H "Authorization: Bearer $OWNER_ACCESS" -H "Content-Type: application/json" \
  -d '{"business_name":"QuikSlate Demo Co"}' | jq -r .business_id)
[ "$BUSINESS_ID" != "null" ] || { echo "FAILED: create business"; exit 1; }
echo "business_id=$BUSINESS_ID"

echo "== create locations in two timezones =="
LOCATION_A_ID=$(curl -s -X POST "$BASE/businesses/$BUSINESS_ID/locations" \
  -H "Authorization: Bearer $OWNER_ACCESS" -H "Content-Type: application/json" \
  -d '{"name":"Downtown","address":"100 Main St, New York, NY","timezone":"America/New_York"}' \
  | jq -r .location.id)
[ "$LOCATION_A_ID" != "null" ] || { echo "FAILED: create location A"; exit 1; }
echo "location_a_id (Downtown, America/New_York)=$LOCATION_A_ID"

LOCATION_B_ID=$(curl -s -X POST "$BASE/businesses/$BUSINESS_ID/locations" \
  -H "Authorization: Bearer $OWNER_ACCESS" -H "Content-Type: application/json" \
  -d '{"name":"Westside","address":"200 Ocean Ave, Los Angeles, CA","timezone":"America/Los_Angeles"}' \
  | jq -r .location.id)
[ "$LOCATION_B_ID" != "null" ] || { echo "FAILED: create location B"; exit 1; }
echo "location_b_id (Westside, America/Los_Angeles)=$LOCATION_B_ID"

echo "== create positions =="
create_position() {
  curl -s -X POST "$BASE/businesses/$BUSINESS_ID/positions" \
    -H "Authorization: Bearer $OWNER_ACCESS" -H "Content-Type: application/json" \
    -d "{\"name\":\"$1\"}" | jq -r .position.id
}
POS_BARISTA=$(create_position "Barista")
POS_SHIFTLEAD=$(create_position "Shift Lead")
POS_CASHIER=$(create_position "Cashier")
POS_COOK=$(create_position "Cook")
POS_SERVER=$(create_position "Server")
POS_HOST=$(create_position "Host")
echo "positions: Barista=$POS_BARISTA Shift Lead=$POS_SHIFTLEAD Cashier=$POS_CASHIER Cook=$POS_COOK Server=$POS_SERVER Host=$POS_HOST"

invite_and_accept() {
  local email="$1" name="$2" role="$3" location_id="$4"
  local invite_token access
  invite_token=$(curl -s -X POST "$BASE/businesses/$BUSINESS_ID/locations/$location_id/invites" \
    -H "Authorization: Bearer $OWNER_ACCESS" -H "Content-Type: application/json" \
    -d "{\"email\":\"$email\",\"target_role\":\"$role\"}" | jq -r .invite_token)
  [ "$invite_token" != "null" ] || { echo "FAILED: invite $email"; exit 1; }

  access=$(register "$email" "$name")
  [ "$access" != "null" ] || { echo "FAILED: register $email"; exit 1; }

  curl -s -X POST "$BASE/businesses/$BUSINESS_ID/invites/$invite_token/accept" \
    -H "Authorization: Bearer $access" | jq -r '"joined business " + .business_id'
}

echo "== seed accounts: one per role at Downtown, plus a roster at Westside =="
invite_and_accept lead@quikslate.dev "Demo Lead" location_lead "$LOCATION_A_ID"
invite_and_accept manager@quikslate.dev "Demo Manager" manager "$LOCATION_A_ID"
invite_and_accept employee@quikslate.dev "Demo Employee" employee "$LOCATION_A_ID"
invite_and_accept employee2@quikslate.dev "Demo Employee Two" employee "$LOCATION_A_ID"
invite_and_accept employee3@quikslate.dev "Demo Employee Three" employee "$LOCATION_A_ID"
invite_and_accept westside.manager@quikslate.dev "Westside Manager" manager "$LOCATION_B_ID"
invite_and_accept westside.employee1@quikslate.dev "Westside Employee One" employee "$LOCATION_B_ID"
invite_and_accept westside.employee2@quikslate.dev "Westside Employee Two" employee "$LOCATION_B_ID"

LEAD_ID=$(user_id_of lead@quikslate.dev)
EMP1_ID=$(user_id_of employee@quikslate.dev)
EMP2_ID=$(user_id_of employee2@quikslate.dev)
EMP3_ID=$(user_id_of employee3@quikslate.dev)
WEMP1_ID=$(user_id_of westside.employee1@quikslate.dev)
WEMP2_ID=$(user_id_of westside.employee2@quikslate.dev)

echo "== create shifts for the current week (Mon-Fri) =="
create_shift() {
  local location_id="$1" position_id="$2" user_id="$3" status="$4" start="$5" end="$6"
  local user_json="null"
  [ -n "$user_id" ] && user_json="\"$user_id\""
  curl -s -o /dev/null -w "%{http_code} " -X POST "$BASE/businesses/$BUSINESS_ID/locations/$location_id/shifts" \
    -H "Authorization: Bearer $OWNER_ACCESS" -H "Content-Type: application/json" \
    -d "{\"position_id\":\"$position_id\",\"user_id\":$user_json,\"status\":\"$status\",\"start_time\":\"$start\",\"end_time\":\"$end\"}"
}

MONDAY=$(date -d "-$(( $(date +%u) - 1 )) days" +%F)
DOWNTOWN_EMPLOYEES=("$EMP1_ID" "$EMP2_ID" "$EMP3_ID")
WESTSIDE_EMPLOYEES=("$WEMP1_ID" "$WEMP2_ID")

for i in 0 1 2 3 4; do
  DAY=$(date -d "$MONDAY +$i days" +%F)
  IS_FRIDAY=$([ "$i" -eq 4 ] && echo 1 || echo 0)
  MORNING_EMP="${DOWNTOWN_EMPLOYEES[$(( i % 3 ))]}"
  AFTERNOON_EMP="${DOWNTOWN_EMPLOYEES[$(( (i + 1) % 3 ))]}"
  W_MORNING_EMP="${WESTSIDE_EMPLOYEES[$(( i % 2 ))]}"
  W_AFTERNOON_EMP="${WESTSIDE_EMPLOYEES[$(( (i + 1) % 2 ))]}"

  # Downtown: lead's supervisory shift, a Barista morning, a Cashier
  # afternoon. Friday's Cashier slot is left uncovered on purpose.
  create_shift "$LOCATION_A_ID" "$POS_SHIFTLEAD" "$LEAD_ID" assigned \
    "$(local_to_utc America/New_York "$DAY 07:00:00")" "$(local_to_utc America/New_York "$DAY 15:00:00")"
  create_shift "$LOCATION_A_ID" "$POS_BARISTA" "$MORNING_EMP" assigned \
    "$(local_to_utc America/New_York "$DAY 08:00:00")" "$(local_to_utc America/New_York "$DAY 16:00:00")"
  if [ "$IS_FRIDAY" -eq 1 ]; then
    create_shift "$LOCATION_A_ID" "$POS_CASHIER" "" uncovered \
      "$(local_to_utc America/New_York "$DAY 16:00:00")" "$(local_to_utc America/New_York "$DAY 23:00:00")"
  else
    create_shift "$LOCATION_A_ID" "$POS_CASHIER" "$AFTERNOON_EMP" assigned \
      "$(local_to_utc America/New_York "$DAY 16:00:00")" "$(local_to_utc America/New_York "$DAY 23:00:00")"
  fi

  # Westside: a Cook morning, a Server afternoon. Friday's Server slot is
  # also left uncovered.
  create_shift "$LOCATION_B_ID" "$POS_COOK" "$W_MORNING_EMP" assigned \
    "$(local_to_utc America/Los_Angeles "$DAY 09:00:00")" "$(local_to_utc America/Los_Angeles "$DAY 17:00:00")"
  if [ "$IS_FRIDAY" -eq 1 ]; then
    create_shift "$LOCATION_B_ID" "$POS_SERVER" "" uncovered \
      "$(local_to_utc America/Los_Angeles "$DAY 17:00:00")" "$(local_to_utc America/Los_Angeles "$DAY 23:00:00")"
  else
    create_shift "$LOCATION_B_ID" "$POS_SERVER" "$W_AFTERNOON_EMP" assigned \
      "$(local_to_utc America/Los_Angeles "$DAY 17:00:00")" "$(local_to_utc America/Los_Angeles "$DAY 23:00:00")"
  fi
done
echo
echo "(status codes above should all read 200)"

SHIFT_COUNT=$(psql_c "SELECT count(*) FROM shifts WHERE location_id IN ('$LOCATION_A_ID','$LOCATION_B_ID');" | tr -d '[:space:]')

echo
echo "== ALL SEEDED =="
cat <<EOF
business_id:   $BUSINESS_ID
location_a_id: $LOCATION_A_ID (Downtown, America/New_York)
location_b_id: $LOCATION_B_ID (Westside, America/Los_Angeles)
positions:     Barista, Shift Lead, Cashier, Cook, Server, Host
shifts:        $SHIFT_COUNT created for the week of $MONDAY (Downtown Fri Cashier and Westside Fri Server left uncovered)

accounts (password for all: $PASSWORD):
  owner@quikslate.dev              - business admin
  lead@quikslate.dev               - location_lead @ Downtown
  manager@quikslate.dev            - manager @ Downtown
  employee@quikslate.dev           - employee @ Downtown
  employee2@quikslate.dev          - employee @ Downtown
  employee3@quikslate.dev          - employee @ Downtown
  westside.manager@quikslate.dev   - manager @ Westside
  westside.employee1@quikslate.dev - employee @ Westside
  westside.employee2@quikslate.dev - employee @ Westside
EOF
