#!/usr/bin/env bash
set -euo pipefail

# RootCauseway Database Migration Runner
# Usage:
#   ./scripts/migrate.sh up       # Apply pending migrations
#   ./scripts/migrate.sh down [N] # Rollback last N migrations (default 1)
#   ./scripts/migrate.sh status   # Show applied migrations

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-rootcauseway}"
DB_NAME="${DB_NAME:-rootcauseway}"
DB_PASS="${DB_PASS:-${POSTGRES_PASSWORD:-rootcauseway_dev_password}}"

MIGRATIONS_DIR="${MIGRATIONS_DIR:-$(dirname "$0")/../backend/migrations}"
MIGRATIONS_DIR="$(cd "$MIGRATIONS_DIR" && pwd)"

export PGPASSWORD="$DB_PASS"

psql_cmd() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -A "$@"
}

ensure_migrations_table() {
    psql_cmd -c "
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version TEXT PRIMARY KEY,
            applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );
    " > /dev/null
}

is_applied() {
    local count
    count=$(psql_cmd -c "SELECT COUNT(*) FROM schema_migrations WHERE version = '$1';")
    [ "$count" -gt 0 ]
}

migrate_up() {
    ensure_migrations_table
    local applied=0

    for f in "$MIGRATIONS_DIR"/*.up.sql; do
        [ -f "$f" ] || continue
        version="$(basename "$f" .up.sql)"

        if is_applied "$version"; then
            echo "[skip] $version (already applied)"
            continue
        fi

        echo "[apply] $version ..."
        psql_cmd -f "$f" > /dev/null
        psql_cmd -c "INSERT INTO schema_migrations (version) VALUES ('$version');" > /dev/null
        applied=$((applied + 1))
    done

    if [ "$applied" -eq 0 ]; then
        echo "No pending migrations."
    else
        echo "Applied $applied migration(s)."
    fi
}

migrate_down() {
    ensure_migrations_table
    local count="${1:-1}"
    local rolled=0

    local versions
    versions=$(psql_cmd -c "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT $count;")

    for version in $versions; do
        local down_file="$MIGRATIONS_DIR/${version}.down.sql"
        if [ ! -f "$down_file" ]; then
            echo "[error] No down migration for $version ($down_file not found)"
            exit 1
        fi
        echo "[rollback] $version ..."
        psql_cmd -f "$down_file" > /dev/null
        psql_cmd -c "DELETE FROM schema_migrations WHERE version = '$version';" > /dev/null
        rolled=$((rolled + 1))
    done

    echo "Rolled back $rolled migration(s)."
}

migrate_status() {
    ensure_migrations_table
    echo "Applied migrations:"
    psql_cmd -c "SELECT version, applied_at FROM schema_migrations ORDER BY version;" | column -t -s '|'

    echo ""
    echo "Pending:"
    local pending=0
    for f in "$MIGRATIONS_DIR"/*.up.sql; do
        [ -f "$f" ] || continue
        version="$(basename "$f" .up.sql)"
        if ! is_applied "$version"; then
            echo "  $version"
            pending=$((pending + 1))
        fi
    done
    [ "$pending" -eq 0 ] && echo "  (none)"
}

case "${1:-}" in
    up)     migrate_up ;;
    down)   migrate_down "${2:-1}" ;;
    status) migrate_status ;;
    *)
        echo "Usage: $0 {up|down [N]|status}"
        exit 1
        ;;
esac
