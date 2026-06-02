#!/usr/bin/env bash
set -euo pipefail

# Post-install script for gym-manager .deb

if ! id gym-manager &>/dev/null; then
    useradd --system --no-create-home --shell /usr/sbin/nologin gym-manager
fi

mkdir -p /opt/gym-manager/data
mkdir -p /var/backups/gym-manager
chown -R gym-manager:gym-manager /opt/gym-manager
chown gym-manager:gym-manager /var/backups/gym-manager

# Install systemd service
cp /opt/gym-manager/deploy/gym-manager.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable gym-manager

# Run migrations if goose is available
if command -v goose &>/dev/null; then
    cd /opt/gym-manager
    goose -dir sql/migrations sqlite3 data/gym.db up
    chown gym-manager:gym-manager data/gym.db
fi

# Create default .env if it doesn't exist
if [ ! -f /opt/gym-manager/.env ]; then
    cat > /opt/gym-manager/.env <<EOF
DATABASE_PATH=/opt/gym-manager/data/gym.db
LISTEN_ADDR=:8080
GYM_BACKUP_DIR=/var/backups/gym-manager
EOF
    chown gym-manager:gym-manager /opt/gym-manager/.env
fi

echo ""
echo "=== Gym Manager installed ==="
echo ""
echo "Database: /opt/gym-manager/data/gym.db (SQLite)"
echo "Config:   /opt/gym-manager/.env"
echo ""
echo "Start: sudo systemctl start gym-manager"
echo "Logs:  journalctl -u gym-manager -f"
