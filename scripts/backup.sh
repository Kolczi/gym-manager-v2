#!/usr/bin/env bash
set -euo pipefail

# Gym Manager backup: SQLite → timestamped .db.gz
# Uses file copy (safe for WAL mode — Go app holds the only connection)
# Usage: ./scripts/backup.sh [output_dir]

DB_PATH="${DATABASE_PATH:-data/gym.db}"
BACKUP_DIR="${1:-${GYM_BACKUP_DIR:-$HOME/gym-backups}}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
FILENAME="gym_manager_${TIMESTAMP}.db.gz"
FILEPATH="${BACKUP_DIR}/${FILENAME}"

mkdir -p "$BACKUP_DIR"

if [ ! -f "$DB_PATH" ]; then
    echo "Database not found: $DB_PATH" >&2
    exit 1
fi

# Copy + gzip (WAL is auto-checkpointed by SQLite on close/threshold)
gzip -c "$DB_PATH" > "$FILEPATH"

SIZE=$(du -h "$FILEPATH" | cut -f1)
echo "${FILEPATH}|${SIZE}|${TIMESTAMP}"

# Keep only last 30 backups
cd "$BACKUP_DIR"
ls -t gym_manager_*.db.gz 2>/dev/null | tail -n +31 | xargs -r rm
