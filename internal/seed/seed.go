package seed

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"

	"gym-manager-v2/internal/store"
)

// EnsureAdmin creates a default admin user if no users exist.
// Default credentials: admin / admin
func EnsureAdmin(ctx context.Context, db *sql.DB, queries *store.Queries) {
	count, err := queries.CountUsers(ctx)
	if err != nil {
		log.Printf("seed: cannot count users: %v", err)
		return
	}
	if count > 0 {
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_, err = queries.CreateUser(ctx, store.CreateUserParams{
		Login:        "admin",
		Name:         "Administrator",
		Surname:      "",
		Role:         "admin",
		PasswordHash: string(hash),
	})
	if err != nil {
		log.Printf("seed: cannot create admin user: %v", err)
		return
	}
	fmt.Println("Created default admin user (login: admin, password: admin)")
}
