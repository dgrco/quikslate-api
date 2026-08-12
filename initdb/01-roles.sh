#!/bin/sh
# Creates the two application roles on first initialization of the data dir.
# Runs once, when postgres_data is empty.
set -e

# Postgres accepts PASSWORD '' as "no password at all" and only emits a NOTICE,
# which neither set -e nor ON_ERROR_STOP catches. An unset variable here would
# otherwise create a role that exists but can never log in.
: "${QUIKSLATE_MIGRATE_PASSWORD:?not set, check .env}"
: "${QUIKSLATE_API_PASSWORD:?not set, check .env}"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
  -- Neither role gets SUPERUSER, CREATEDB, or CREATEROLE; those are off by default
  CREATE ROLE quikslate_migrate LOGIN PASSWORD '${QUIKSLATE_MIGRATE_PASSWORD}';
  CREATE ROLE quikslate_api     LOGIN PASSWORD '${QUIKSLATE_API_PASSWORD}';

  -- migrate creates and owns tables; api may only look inside the schema.
  GRANT USAGE, CREATE ON SCHEMA public TO quikslate_migrate;
  GRANT USAGE         ON SCHEMA public TO quikslate_api;

  -- Standing rule for every table a future migration creates.
  ALTER DEFAULT PRIVILEGES FOR ROLE quikslate_migrate IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO quikslate_api;

  ALTER DEFAULT PRIVILEGES FOR ROLE quikslate_migrate IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO quikslate_api;
EOSQL
