package dbinit

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// AutoMigrate runs all SQL migrations embedded in the binary.
func AutoMigrate(db *sql.DB) error {
	// Create goose-compatible version table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL DEFAULT 1,
		tstamp TEXT DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("create version table: %w", err)
	}

	// Get current version
	var currentVersion int64
	row := db.QueryRow("SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied = 1")
	row.Scan(&currentVersion)

	// Read migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Parse version from filename (e.g., "001_initial.sql" -> 1)
		var version int64
		fmt.Sscanf(entry.Name(), "%d_", &version)
		if version <= currentVersion {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		// Extract Up section (everything between "-- +goose Up" and "-- +goose Down")
		sql := extractUp(string(content))
		if sql == "" {
			continue
		}

		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("run migration %s: %w", entry.Name(), err)
		}

		if _, err := db.Exec("INSERT INTO goose_db_version (version_id) VALUES (?)", version); err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}

		log.Printf("Migration applied: %s", entry.Name())
	}

	return nil
}

func extractUp(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inUp := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose Up") {
			inUp = true
			continue
		}
		if strings.HasPrefix(trimmed, "-- +goose Down") {
			break
		}
		if inUp {
			result = append(result, line)
		}
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}
