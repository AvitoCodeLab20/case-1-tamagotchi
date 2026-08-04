#!/bin/sh
set -eu

POSTGRES_HOST="${POSTGRES_HOST:-postgres}"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_DB="${POSTGRES_DB:-tamagotchi}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-/migrations}"

export PGPASSWORD="${POSTGRES_PASSWORD}"

psql_command() {
    psql \
        --host="${POSTGRES_HOST}" \
        --port="${POSTGRES_PORT}" \
        --username="${POSTGRES_USER}" \
        --dbname="${POSTGRES_DB}" \
        --no-password \
        --set=ON_ERROR_STOP=1 \
        "$@"
}

psql_command <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

for migration_path in "${MIGRATIONS_DIR}"/*_up.sql; do
    migration_file="$(basename "${migration_path}")"
    migration_version="${migration_file%%_*}"

    case "${migration_version}" in
        ''|*[!0-9]*)
            echo "Invalid migration filename: ${migration_file}" >&2
            exit 1
            ;;
    esac

    migration_checksum="$(sha256sum "${migration_path}" | awk '{print $1}')"
    applied_checksum="$(psql_command --tuples-only --no-align --command="SELECT checksum FROM schema_migrations WHERE version = ${migration_version}")"

    if [ -n "${applied_checksum}" ]; then
        if [ "${applied_checksum}" != "${migration_checksum}" ]; then
            echo "Checksum mismatch for applied migration ${migration_file}" >&2
            exit 1
        fi

        echo "Migration ${migration_file} already applied"
        continue
    fi

    echo "Applying migration ${migration_file}"
    psql_command \
        --set="migration_version=${migration_version}" \
        --set="migration_name=${migration_file}" \
        --set="migration_checksum=${migration_checksum}" <<SQL
BEGIN;
\i ${migration_path}
INSERT INTO schema_migrations (version, name, checksum)
VALUES (:'migration_version'::BIGINT, :'migration_name', :'migration_checksum');
COMMIT;
SQL
done
