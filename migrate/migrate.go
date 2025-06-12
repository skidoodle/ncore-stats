package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ProfileData struct to match the structure in data.json
type ProfileData struct {
	Owner           string    `json:"owner"`
	Timestamp       time.Time `json:"timestamp"`
	Rank            int       `json:"rank"`
	Upload          string    `json:"upload"`
	CurrentUpload   string    `json:"current_upload"`
	CurrentDownload string    `json:"current_download"`
	Points          int       `json:"points"`
	SeedingCount    int       `json:"seeding_count"`
}

var (
	db           *sql.DB
	dbFile       = "../data/ncore_stats.db"
	jsonFile     = "../data/data.json"
	profilesFile = "../data/profiles.json"
)

// initDB is the same function from the main app to set up the database.
func initDB() {
	var err error
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	db, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	// Create users table
	usersTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"display_name" TEXT NOT NULL UNIQUE,
		"profile_id" TEXT NOT NULL
	);`
	if _, err = db.Exec(usersTableSQL); err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}
	// Create profile_history table
	profileHistoryTableSQL := `
	CREATE TABLE IF NOT EXISTS profile_history (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"user_id" INTEGER NOT NULL,
		"timestamp" DATETIME NOT NULL,
		"rank" INTEGER,
		"upload" TEXT,
		"current_upload" TEXT,
		"current_download" TEXT,
		"points" INTEGER,
		"seeding_count" INTEGER,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`
	if _, err = db.Exec(profileHistoryTableSQL); err != nil {
		log.Fatalf("Failed to create profile_history table: %v", err)
	}
	log.Println("Database initialized successfully.")
}

func main() {
	log.Println("Starting data migration...")

	// 1. Initialize the database and tables
	initDB()
	defer db.Close()

	// 2. Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	// Defer a rollback. If the transaction is committed, this is a no-op.
	// If something fails, this will undo all changes.
	defer tx.Rollback()

	// --- Migrate Users ---
	log.Println("Reading profiles.json...")
	profFile, err := os.Open(profilesFile)
	if err != nil {
		log.Fatalf("Failed to open profiles file: %v", err)
	}
	defer profFile.Close()

	var profiles map[string]string
	if err := json.NewDecoder(profFile).Decode(&profiles); err != nil {
		log.Fatalf("Failed to decode profiles.json: %v", err)
	}

	// This map will hold the new database ID for each user display name
	displayNameToID := make(map[string]int64)

	userStmt, err := tx.Prepare("INSERT INTO users(display_name, profile_id) VALUES(?, ?)")
	if err != nil {
		log.Fatalf("Failed to prepare user insert statement: %v", err)
	}
	defer userStmt.Close()

	log.Println("Migrating users to the database...")
	for name, id := range profiles {
		res, err := userStmt.Exec(name, id)
		if err != nil {
			log.Fatalf("Failed to insert user %s: %v", name, err)
		}
		newID, err := res.LastInsertId()
		if err != nil {
			log.Fatalf("Failed to get last insert ID for user %s: %v", name, err)
		}
		displayNameToID[name] = newID
		log.Printf("  > Migrated user: %s (New DB ID: %d)", name, newID)
	}
	log.Println("User migration complete.")

	// --- Migrate History ---
	log.Println("Reading data.json...")
	histFile, err := os.Open(jsonFile)
	if err != nil {
		log.Fatalf("Failed to open data file: %v", err)
	}
	defer histFile.Close()

	byteValue, _ := io.ReadAll(histFile)
	var history []ProfileData
	if err := json.Unmarshal(byteValue, &history); err != nil {
		log.Fatalf("Failed to decode data.json: %v", err)
	}

	historyStmt, err := tx.Prepare(`
		INSERT INTO profile_history(user_id, timestamp, rank, upload, current_upload, current_download, points, seeding_count)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		log.Fatalf("Failed to prepare history insert statement: %v", err)
	}
	defer historyStmt.Close()

	log.Println("Migrating profile history to the database...")
	for i, record := range history {
		userID, ok := displayNameToID[record.Owner]
		if !ok {
			log.Printf("WARNING: Skipping history record for '%s' as they were not found in profiles.json.", record.Owner)
			continue
		}

		_, err := historyStmt.Exec(userID, record.Timestamp, record.Rank, record.Upload, record.CurrentUpload, record.CurrentDownload, record.Points, record.SeedingCount)
		if err != nil {
			log.Fatalf("Failed to insert history record for %s: %v", record.Owner, err)
		}
		if (i+1)%100 == 0 { // Log progress every 100 records
			log.Printf("  > Migrated %d history records...", i+1)
		}
	}
	log.Println("History migration complete.")

	// 3. Commit the transaction
	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	fmt.Println("\n✅ MIGRATION COMPLETED SUCCESSFULLY!")
}
