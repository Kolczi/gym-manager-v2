package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wailsapp/wails/v3/pkg/application"

	"gym-manager-v2/internal/auth"
	"gym-manager-v2/internal/dbinit"
	"gym-manager-v2/internal/seed"
	"gym-manager-v2/internal/server"
	"gym-manager-v2/internal/store"
)

//go:embed all:frontend/dist
var staticFS embed.FS

//go:embed all:internal/templates
var templateFS embed.FS

func main() {
	_ = godotenv.Load()
	ensureEnvFile()

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "data/gym.db"
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Cannot create data directory: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("Unable to open database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	// Auto-migrate
	if err := dbinit.AutoMigrate(db); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	queries := store.New(db)

	// Create default admin user if none exist
	seed.EnsureAdmin(context.Background(), db, queries)

	authSvc := auth.NewService(queries)
	srv := server.New(queries, authSvc, templateFS, staticFS)

	// Dev HTTP mode — for testing without GTK/WebView
	if os.Getenv("GYM_DEV_HTTP") == "1" {
		addr := ":8080"
		fmt.Printf("Dev HTTP server on http://localhost%s\n", addr)
		fmt.Printf("Database: %s\n", dbPath)
		log.Fatal(http.ListenAndServe(addr, srv.Router))
		return
	}

	app := application.New(application.Options{
		Name:        "Gym Manager",
		Description: "System zarządzania siłownią",
		Assets: application.AssetOptions{
			Handler: srv.Router,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Gym Manager",
		Width:            1280,
		Height:           800,
		BackgroundColour: application.NewRGB(15, 23, 42),
		URL:              "/",
	})

	app.OnShutdown(func() {
		db.Close()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
