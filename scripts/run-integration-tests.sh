#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

export ROOTCAUSEWAY_TEST_DB_URL="${ROOTCAUSEWAY_TEST_DB_URL:-postgres://rootcauseway:rootcauseway_dev_password@localhost:5432/test_rootcauseway?sslmode=disable}"

echo "=== Setting up test database ==="
createdb test_rootcauseway 2>/dev/null || echo "Database test_rootcauseway already exists"

# Apply all migrations in order
MIGRATIONS_DIR="$PROJECT_DIR/backend/migrations"
for migration in $(ls "$MIGRATIONS_DIR"/*.up.sql 2>/dev/null | sort); do
    echo "  Applying $(basename "$migration")..."
    psql -d test_rootcauseway -f "$migration" -q 2>/dev/null || true
done

echo ""
echo "=== Running Go integration tests ==="
cd "$PROJECT_DIR/backend"
ROOTCAUSEWAY_TEST_DB_URL="$ROOTCAUSEWAY_TEST_DB_URL" go test -tags=integration ./... -v -count=1
GO_EXIT=$?

echo ""
echo "=== Running Python integration tests ==="
cd "$PROJECT_DIR/agent-service"
python -m pytest tests/ -v -m integration --tb=short
PY_EXIT=$?

echo ""
echo "=== Cleanup ==="
dropdb test_rootcauseway 2>/dev/null || echo "Could not drop test_rootcauseway (may not exist)"

echo ""
if [ $GO_EXIT -ne 0 ] || [ $PY_EXIT -ne 0 ]; then
    echo "FAILED: Go exit=$GO_EXIT, Python exit=$PY_EXIT"
    exit 1
fi
echo "All integration tests passed."
