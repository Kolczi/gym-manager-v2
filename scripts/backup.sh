#!/usr/bin/env bash
set -euo pipefail

# Gym Manager backup: pg_dump → timestamped file
# Usage: ./scripts/backup.sh [output_dir]

DB_NAME="${GYM_DB_NAME:-gym_manager}"
BACKUP_DIR="${1:-${GYM_BACKUP_DIR:-$HOME/gym-backups}}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
FILENAME="gym_manager_${TIMESTAMP}.sql.gz"
FILEPATH="${BACKUP_DIR}/${FILENAME}"

mkdir -p "$BACKUP_DIR"

pg_dump -Fc --no-owner --no-acl --dbname="${DATABASE_URL:-postgresql:///$DB_NAME}" | gzip > "$FILEPATH"

SIZE=$(du -h "$FILEPATH" | cut -f1)
echo "${FILEPATH}|${SIZE}|${TIMESTAMP}"

# Keep only last 30 backups
cd "$BACKUP_DIR"
ls -t gym_manager_*.sql.gz 2>/dev/null | tail -n +31 | xargs -r rm
