#!/usr/bin/env bash
set -euo pipefail

: "${POSTGRES_SW_API_USER:=sw_api}"
: "${POSTGRES_SW_API_PASS:=sw_api}"
: "${POSTGRES_SW_HM_USER:=sw_hallmonitor}"
: "${POSTGRES_SW_HM_PASS:=sw_hallmonitor}"

export PGUSER="${POSTGRES_USER:-swearjarbot}"
export PGPASSWORD="${POSTGRES_PASSWORD:-swearjar}"
export PGDATABASE="${POSTGRES_DB:-swearjar}"

psql <<SQL
DO \$\$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${POSTGRES_SW_API_USER}') THEN
    CREATE ROLE ${POSTGRES_SW_API_USER} LOGIN PASSWORD '${POSTGRES_SW_API_PASS}';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${POSTGRES_SW_HM_USER}') THEN
    CREATE ROLE ${POSTGRES_SW_HM_USER} LOGIN PASSWORD '${POSTGRES_SW_HM_PASS}';
  END IF;
END
\$\$;
SQL
