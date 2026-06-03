package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"gym-manager-v2/internal/store"
)

// MongoDB document shapes
type MongoClient struct {
	ID      bson.ObjectID          `bson:"_id"`
	Name    string                 `bson:"name"`
	Surname string                 `bson:"surname"`
	Phone   string                 `bson:"phone"`
	Mail    string                 `bson:"mail"`
	Comment string                 `bson:"comment"`
	RfidTag string                 `bson:"rfidTag"`
	Carnet  *MongoCarnetAssignment `bson:"carnet"`
	Entries []MongoEntry           `bson:"entries"`
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

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		log.Fatal("DATABASE_PATH required (e.g. data/gym.db)")
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

	// Connect SQLite
	sqliteDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		log.Fatalf("SQLite open: %v", err)
	}
	defer sqliteDB.Close()
	queries := store.New(sqliteDB)
	fmt.Printf("Connected to SQLite: %s\n", dbPath)

	// --- Migrate Users ---
	fmt.Println("\n=== Migrating Users ===")
	userCur, _ := db.Collection("users").Find(ctx, bson.D{})
	var mongoUsers []MongoUser
	userCur.All(ctx, &mongoUsers)

	userCount := 0
	for _, mu := range mongoUsers {
		_, err := queries.CreateUser(ctx, store.CreateUserParams{
			Login:        strings.ToLower(mu.Login),
			PasswordHash: mu.Password,
			Name:         mu.Name,
			Surname:      mu.Surname,
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

	carnetMap := map[string]int64{} // mongo _id hex → sqlite id
	carnetCount := 0
	for _, mc := range mongoCarnets {
		durationValue, durationUnit := parseDuration(mc.Duration)
		mt, err := queries.CreateMembershipType(ctx, store.CreateMembershipTypeParams{
			Name:          mc.Name,
			DurationValue: durationValue,
			DurationUnit:  durationUnit,
			Price:         mc.Price,
			Description:   nullString(mc.Description),
			IsActive:      sql.NullInt64{Int64: 1, Valid: true},
		})
		if err != nil {
			fmt.Printf("  SKIP carnet %s: %v\n", mc.Name, err)
			continue
		}
		carnetMap[mc.ID.Hex()] = mt.ID
		carnetCount++
		fmt.Printf("  + type: %s (%d %s)\n", mc.Name, durationValue, durationUnit)
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
			Email:   nullString(mc.Mail),
			Phone:   nullString(mc.Phone),
			Comment: nullString(mc.Comment),
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
				RecordedBy: sql.NullInt64{},
				Method:     nullString(method),
			})
			if err == nil {
				entryCount++
			}
		}

		// Migrate active carnet → membership
		if mc.Carnet != nil {
			typeID, ok := carnetMap[mc.Carnet.CarnetID.Hex()]
			if ok {
				isActive := int64(0)
				if mc.Carnet.EndDate.After(time.Now()) {
					isActive = 1
				}
				_, err := queries.CreateMembership(ctx, store.CreateMembershipParams{
					ClientID: client.ID,
					TypeID:   typeID,
					StartsAt: mc.Carnet.StartDate.Format("2006-01-02"),
					EndsAt:   mc.Carnet.EndDate.Format("2006-01-02"),
					IsActive: sql.NullInt64{Int64: isActive, Valid: true},
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

func parseDuration(s string) (int64, string) {
	// "30d" → (30, "days"), "1m" → (1, "months"), "3m" → (3, "months")
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		var n int
		fmt.Sscanf(s, "%dd", &n)
		if n == 0 {
			n = 30
		}
		return int64(n), "days"
	}
	if strings.HasSuffix(s, "m") {
		var n int
		fmt.Sscanf(s, "%dm", &n)
		if n == 0 {
			n = 1
		}
		return int64(n), "months"
	}
	return 30, "days"
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// sha256Hash for reference/verification
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
