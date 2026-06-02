#!/usr/bin/env bash
set -euo pipefail

# Gym Manager sync: local → VPS
# Usage: ./scripts/sync.sh [user@host]
#
# Dumps local DB, rsyncs binary + templates + DB dump to VPS, restores DB there.
# Requires: pg_dump, rsync, ssh access to VPS

VPS_HOST="${1:-${GYM_VPS_HOST:-}}"
VPS_DIR="${GYM_VPS_DIR:-/opt/gym-manager}"
LOCAL_DB="${GYM_DB_NAME:-gym_manager}"
REMOTE_DB="${GYM_REMOTE_DB:-gym_manager}"
DUMP_FILE="/tmp/gym_manager_sync.sql"

if [[ -z "$VPS_HOST" ]]; then
    echo "Usage: $0 user@host"
    echo "  or set GYM_VPS_HOST env var"
    exit 1
fi

echo "=== Gym Manager Sync: local → $VPS_HOST ==="
echo ""

# 1. Dump local database
echo "[1/4] Dumping local database '$LOCAL_DB'..."
pg_dump -Fc --no-owner --no-acl "$LOCAL_DB" > "$DUMP_FILE"
echo "  → $(du -h "$DUMP_FILE" | cut -f1) dump created"

# 2. Build headless binary
echo "[2/4] Building headless binary..."
SCRIPT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$SCRIPT_DIR"
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
go build -o bin/gym-web ./cmd/web/
echo "  → bin/gym-web built"

# 3. Rsync files to VPS
echo "[3/4] Syncing files to $VPS_HOST:$VPS_DIR ..."
ssh "$VPS_HOST" "mkdir -p $VPS_DIR/{bin,internal/templates,frontend/dist}"

rsync -avz --progress \
    bin/gym-web \
    "$VPS_HOST:$VPS_DIR/bin/"

rsync -avz --delete \
    internal/templates/ \
    "$VPS_HOST:$VPS_DIR/internal/templates/"

rsync -avz --delete \
    frontend/dist/ \
    "$VPS_HOST:$VPS_DIR/frontend/dist/"

rsync -avz \
    "$DUMP_FILE" \
    "$VPS_HOST:/tmp/gym_manager_sync.sql"

# Sync .env if it exists and VPS doesn't have one
if [[ -f .env ]]; then
    ssh "$VPS_HOST" "test -f $VPS_DIR/.env || true" && \
    rsync -avz .env "$VPS_HOST:$VPS_DIR/.env" 2>/dev/null || true
fi

echo "  → files synced"

# 4. Restore database on VPS
echo "[4/4] Restoring database on VPS..."
ssh "$VPS_HOST" "pg_restore --clean --if-exists --no-owner --no-acl -d $REMOTE_DB /tmp/gym_manager_sync.sql 2>/dev/null; rm -f /tmp/gym_manager_sync.sql"
echo "  → database restored"

# Restart service if systemd unit exists
ssh "$VPS_HOST" "systemctl is-active gym-manager >/dev/null 2>&1 && sudo systemctl restart gym-manager && echo '  → service restarted' || echo '  → no systemd service found, start manually'"

echo ""
echo "=== Sync complete! ==="
echo "VPS: $VPS_HOST:$VPS_DIR"
echo "Start: cd $VPS_DIR && ./bin/gym-web"
