package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"gym-manager-v2/internal/store"
)

// MongoDB document shapes
type MongoClient struct {
	ID      bson.ObjectID `bson:"_id"`
	Name    string        `bson:"name"`
	Surname string        `bson:"surname"`
	Phone   string        `bson:"phone"`
	Mail    string        `bson:"mail"`
	Comment string        `bson:"comment"`
	RfidTag string        `bson:"rfidTag"`
	Carnet  *MongoCarnetAssignment `bson:"carnet"`
	Entries []MongoEntry  `bson:"entries"`
}

type MongoCarnetAssignment struct {
	CarnetID  bson.ObjectID `bson:"carnetId"`
	StartDate time.Time     `bson:"startDate"`
	EndDate   time.Time     `bson:"endDate"`
	Paid      bool          `bson:"paid"`
}

type MongoEntry struct {
	Date   time.Time `bson:"date"`
	Method string    `bson:"method"`
}

type MongoCarnet struct {
	ID          bson.ObjectID `bson:"_id"`
	Name        string        `bson:"name"`
	Duration    string        `bson:"duration"`
	Price       float64       `bson:"price"`
	Description string        `bson:"description"`
}

type MongoUser struct {
	ID       bson.ObjectID `bson:"_id"`
	Login    string        `bson:"login"`
	Name     string        `bson:"name"`
	Surname  string        `bson:"surname"`
	Role     string        `bson:"role"`
	Password string        `bson:"password"`
}

func main() {
	_ = godotenv.Load()

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://127.0.0.1:27017"
	}
	mongoDB := os.Getenv("MONGO_DB")
	if mongoDB == "" {
		mongoDB = "gymManager"
	}

	pgURL := os.Getenv("DATABASE_URL")
	if pgURL == "" {
		log.Fatal("DATABASE_URL required")
	}

	ctx := context.Background()

	// Connect MongoDB
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("MongoDB connect: %v", err)
	}
	defer mongoClient.Disconnect(ctx)
	db := mongoClient.Database(mongoDB)
	fmt.Printf("Connected to MongoDB: %s/%s\n", mongoURI, mongoDB)

	// Connect PostgreSQL
	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		log.Fatalf("PostgreSQL connect: %v", err)
	}
	defer pool.Close()
	queries := store.New(pool)
	fmt.Println("Connected to PostgreSQL")

	// --- Migrate Users ---
	fmt.Println("\n=== Migrating Users ===")
	userCur, _ := db.Collection("users").Find(ctx, bson.D{})
	var mongoUsers []MongoUser
	userCur.All(ctx, &mongoUsers)

	userCount := 0
	for _, mu := range mongoUsers {
		// SHA256 passwords stay as-is — dual-check in auth handles them
		_, err := queries.CreateUser(ctx, store.CreateUserParams{
			Login:        strings.ToLower(mu.Login),
			PasswordHash: mu.Password,
			Name:         mu.Name,
			Surname:      pgtype.Text{String: mu.Surname, Valid: mu.Surname != ""},
			Role:         mu.Role,
		})
		if err != nil {
			fmt.Printf("  SKIP user %s: %v\n", mu.Login, err)
			continue
		}
		userCount++
		fmt.Printf("  + user: %s (%s)\n", mu.Login, mu.Role)
	}
	fmt.Printf("  → %d users migrated\n", userCount)

	// --- Migrate Carnets → membership_types ---
	fmt.Println("\n=== Migrating Carnets → membership_types ===")
	carnetCur, _ := db.Collection("carnets").Find(ctx, bson.D{})
	var mongoCarnets []MongoCarnet
	carnetCur.All(ctx, &mongoCarnets)

	carnetMap := map[string]int32{} // mongo _id hex → pg id
	carnetCount := 0
	for _, mc := range mongoCarnets {
		durationDays := parseDuration(mc.Duration)
		mt, err := queries.CreateMembershipType(ctx, store.CreateMembershipTypeParams{
			Name:         mc.Name,
			DurationDays: int32(durationDays),
			Price:        pgtype.Numeric{Valid: mc.Price > 0},
			Description:  pgtype.Text{String: mc.Description, Valid: mc.Description != ""},
			IsActive:     pgtype.Bool{Bool: true, Valid: true},
		})
		if err != nil {
			fmt.Printf("  SKIP carnet %s: %v\n", mc.Name, err)
			continue
		}
		carnetMap[mc.ID.Hex()] = mt.ID
		carnetCount++
		fmt.Printf("  + type: %s (%dd)\n", mc.Name, durationDays)
	}
	fmt.Printf("  → %d membership types migrated\n", carnetCount)

	// --- Migrate Clients + Entries + Memberships ---
	fmt.Println("\n=== Migrating Clients ===")
	clientCur, _ := db.Collection("clients").Find(ctx, bson.D{})
	var mongoClients []MongoClient
	clientCur.All(ctx, &mongoClients)

	clientCount, entryCount, membershipCount := 0, 0, 0
	for _, mc := range mongoClients {
		client, err := queries.CreateClient(ctx, store.CreateClientParams{
			Name:    mc.Name,
			Surname: mc.Surname,
			Email:   textOrNull(mc.Mail),
			Phone:   textOrNull(mc.Phone),
			Comment: textOrNull(mc.Comment),
			RfidTag: textOrNull(mc.RfidTag),
		})
		if err != nil {
			fmt.Printf("  SKIP client %s %s: %v\n", mc.Name, mc.Surname, err)
			continue
		}
		clientCount++

		// Migrate entries
		for _, e := range mc.Entries {
			method := e.Method
			if method == "" {
				method = "manual"
			}
			_, err := queries.CreateEntry(ctx, store.CreateEntryParams{
				ClientID:   client.ID,
				RecordedBy: pgtype.Int4{},
				Method:     pgtype.Text{String: method, Valid: true},
			})
			if err == nil {
				entryCount++
			}
		}

		// Migrate active carnet → membership
		if mc.Carnet != nil {
			typeID, ok := carnetMap[mc.Carnet.CarnetID.Hex()]
			if ok {
				_, err := queries.CreateMembership(ctx, store.CreateMembershipParams{
					ClientID:         client.ID,
					MembershipTypeID: typeID,
					StartsAt:         pgtype.Date{Time: mc.Carnet.StartDate, Valid: true},
					EndsAt:           pgtype.Date{Time: mc.Carnet.EndDate, Valid: true},
					IsActive:         pgtype.Bool{Bool: mc.Carnet.EndDate.After(time.Now()), Valid: true},
				})
				if err == nil {
					membershipCount++
				}
			}
		}

		fmt.Printf("  + client: %s %s (%d entries)\n", mc.Name, mc.Surname, len(mc.Entries))
	}

	fmt.Printf("\n=== Migration Summary ===\n")
	fmt.Printf("Users:       %d\n", userCount)
	fmt.Printf("Types:       %d\n", carnetCount)
	fmt.Printf("Clients:     %d\n", clientCount)
	fmt.Printf("Entries:     %d\n", entryCount)
	fmt.Printf("Memberships: %d\n", membershipCount)
}

func parseDuration(s string) int {
	// "30d" → 30, "1m" → 30, "3m" → 90, etc.
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		n := 0
		fmt.Sscanf(s, "%dd", &n)
		return n
	}
	if strings.HasSuffix(s, "m") {
		n := 0
		fmt.Sscanf(s, "%dm", &n)
		return n * 30
	}
	return 30
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// sha256Hash for reference/verification
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
