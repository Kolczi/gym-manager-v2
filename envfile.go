package main

import (
	"fmt"
	"os"
)

const envTemplate = `# Gym Manager — konfiguracja
# Skopiuj ten plik lub uzupełnij wartości poniżej.

# Ścieżka do bazy SQLite (domyślnie: data/gym.db)
DATABASE_PATH=

# Katalog backupów (domyślnie: data/backups/)
GYM_BACKUP_DIR=

# Tryb deweloperski — serwer HTTP bez Wails (1 = włączony)
# GYM_DEV_HTTP=1
`

// ensureEnvFile creates a .env file with empty defaults if it doesn't exist yet.
func ensureEnvFile() {
	if _, err := os.Stat(".env"); err == nil {
		return // already exists
	}
	if err := os.WriteFile(".env", []byte(envTemplate), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create .env: %v\n", err)
	}
}
