package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

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

type User struct {
	ID          int
	DisplayName string
	ProfileID   string
}

var (
	db      *sql.DB
	dbFile  = "./data/ncore_stats.db"
	baseUrl = "https://ncore.pro/profile.php?id="
	nick    string
	pass    string
	client  *http.Client
)

func profilesHandler(w http.ResponseWriter, r *http.Request) {
	query := `
	SELECT u.display_name, ph.timestamp, ph.rank, ph.upload, ph.current_upload, ph.current_download, ph.points, ph.seeding_count
	FROM profile_history ph
	INNER JOIN (
		SELECT user_id, MAX(timestamp) as max_ts
		FROM profile_history
		GROUP BY user_id
	) latest ON ph.user_id = latest.user_id AND ph.timestamp = latest.max_ts
	JOIN users u ON ph.user_id = u.id;
	`
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, "Could not read latest profiles from database", http.StatusInternalServerError)
		log.Printf("Error querying latest profiles: %v", err)
		return
	}
	defer rows.Close()

	var latestProfiles []ProfileData
	for rows.Next() {
		var p ProfileData
		if err := rows.Scan(&p.Owner, &p.Timestamp, &p.Rank, &p.Upload, &p.CurrentUpload, &p.CurrentDownload, &p.Points, &p.SeedingCount); err != nil {
			http.Error(w, "Could not process profile data", http.StatusInternalServerError)
			log.Printf("Error scanning latest profile row: %v", err)
			return
		}
		latestProfiles = append(latestProfiles, p)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(latestProfiles); err != nil {
		log.Printf("Error encoding latest profiles to JSON: %v", err)
	}
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	if owner == "" {
		http.Error(w, "Missing 'owner' query parameter", http.StatusBadRequest)
		return
	}

	query := `
		SELECT u.display_name, ph.timestamp, ph.rank, ph.upload, ph.current_upload, ph.current_download, ph.points, ph.seeding_count
		FROM profile_history ph
		JOIN users u ON ph.user_id = u.id
		WHERE u.display_name = ?
		ORDER BY ph.timestamp ASC
	`
	rows, err := db.Query(query, owner)
	if err != nil {
		http.Error(w, "Could not read history from database", http.StatusInternalServerError)
		log.Printf("Error querying history for %s: %v", owner, err)
		return
	}
	defer rows.Close()

	var userHistory []ProfileData
	for rows.Next() {
		var p ProfileData
		if err := rows.Scan(&p.Owner, &p.Timestamp, &p.Rank, &p.Upload, &p.CurrentUpload, &p.CurrentDownload, &p.Points, &p.SeedingCount); err != nil {
			http.Error(w, "Could not process history data", http.StatusInternalServerError)
			log.Printf("Error scanning history row for %s: %v", owner, err)
			return
		}
		userHistory = append(userHistory, p)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(userHistory); err != nil {
		log.Printf("Error encoding history for %s to JSON: %v", owner, err)
	}
}

func main() {
	// --- Environment and Initialization ---
	_ = godotenv.Load(".env.local")
	godotenv.Load()
	nick = os.Getenv("NICK")
	pass = os.Getenv("PASS")

	// --- Ensure critical env vars are set ---
	if nick == "" || pass == "" {
		log.Fatal("FATAL: Critical environment variables NICK and/or PASS are not set. Please create a .env or .env.local file with these values.")
	}

	client = &http.Client{}

	initDB()
	defer db.Close()

	// --- Command-line flags for user management ---
	addUserFlag := flag.String("add-user", "", "Add a new user. Provide as 'DisplayName,ProfileID'")
	flag.Parse()

	if *addUserFlag != "" {
		parts := strings.Split(*addUserFlag, ",")
		if len(parts) != 2 {
			log.Fatal("Invalid format for --add-user. Use 'DisplayName,ProfileID'")
		}
		addUser(parts[0], parts[1])
		return // Exit after adding user
	}

	// --- Background task and Web Server ---
	go func() {
		fetchAndLogProfiles()
		ticker := time.NewTicker(time.Hour * 24)
		defer ticker.Stop()
		for range ticker.C {
			fetchAndLogProfiles()
		}
	}()

	// --- Handler Registration ---
	http.HandleFunc("/api/profiles", profilesHandler)
	http.HandleFunc("/api/history", historyHandler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		http.ServeFile(w, r, "style.css")
	})

	http.HandleFunc("/script.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "script.js")
	})

	log.Println("Server is starting on port 3000...")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func initDB() {
	var err error
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}
	db, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	usersTableSQL := `CREATE TABLE IF NOT EXISTS users ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "display_name" TEXT NOT NULL UNIQUE, "profile_id" TEXT NOT NULL);`
	_, err = db.Exec(usersTableSQL)
	if err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}
	profileHistoryTableSQL := `CREATE TABLE IF NOT EXISTS profile_history ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "user_id" INTEGER NOT NULL, "timestamp" DATETIME NOT NULL, "rank" INTEGER, "upload" TEXT, "current_upload" TEXT, "current_download" TEXT, "points" INTEGER, "seeding_count" INTEGER, FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);`
	_, err = db.Exec(profileHistoryTableSQL)
	if err != nil {
		log.Fatalf("Failed to create profile_history table: %v", err)
	}
	log.Println("Database initialized successfully.")
}

func addUser(displayName, profileID string) {
	stmt, err := db.Prepare("INSERT INTO users(display_name, profile_id) VALUES(?, ?)")
	if err != nil {
		log.Fatalf("Failed to prepare statement for adding user: %v", err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(displayName, profileID)
	if err != nil {
		log.Fatalf("Failed to add user %s: %v", displayName, err)
	}
	log.Printf("User '%s' with profile ID '%s' added successfully.", displayName, profileID)
}

func getUsers() ([]User, error) {
	rows, err := db.Query("SELECT id, display_name, profile_id FROM users")
	if err != nil {
		return nil, fmt.Errorf("error querying users: %v", err)
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.DisplayName, &u.ProfileID); err != nil {
			return nil, fmt.Errorf("error scanning user row: %v", err)
		}
		users = append(users, u)
	}
	return users, nil
}

func logToDB(profile *ProfileData, userID int) error {
	stmt, err := db.Prepare(`INSERT INTO profile_history(user_id, timestamp, rank, upload, current_upload, current_download, points, seeding_count) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("error preparing insert statement: %v", err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(userID, profile.Timestamp, profile.Rank, profile.Upload, profile.CurrentUpload, profile.CurrentDownload, profile.Points, profile.SeedingCount)
	if err != nil {
		return fmt.Errorf("error executing insert for %s: %v", profile.Owner, err)
	}
	log.Printf("Profile for %s logged successfully to database.", profile.Owner)
	return nil
}

func fetchAndLogProfiles() {
	users, err := getUsers()
	if err != nil {
		log.Printf("Could not get users to fetch: %v", err)
		return
	}
	log.Printf("Starting profile fetch for %d user(s).", len(users))
	for _, user := range users {
		profileURL := baseUrl + user.ProfileID
		profile, err := fetchProfile(profileURL, user.DisplayName)
		if err != nil {
			log.Printf("Error fetching profile for %s: %v", user.DisplayName, err)
			continue
		}
		if err := logToDB(profile, user.ID); err != nil {
			log.Printf("Error logging profile to DB for %s: %v", user.DisplayName, err)
		}
		time.Sleep(2 * time.Second)
	}
	log.Println("Profile fetch cycle complete.")
}

func fetchProfile(url string, displayName string) (*ProfileData, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s: %v", displayName, err)
	}
	req.Header.Set("Cookie", fmt.Sprintf("nick=%s; pass=%s", nick, pass))
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching profile for %s: %v", displayName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch profile %s: received status %d", displayName, resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing profile document for %s: %v", displayName, err)
	}
	profile := &ProfileData{Owner: displayName, Timestamp: time.Now()}
	doc.Find(".userbox_tartalom_mini .profil_jobb_elso2").Each(func(i int, s *goquery.Selection) {
		label, value := s.Text(), s.Next().Text()
		switch label {
		case "Helyezés:":
			profile.Rank, _ = strconv.Atoi(strings.TrimSuffix(value, "."))
		case "Feltöltés:":
			profile.Upload = value
		case "Aktuális feltöltés:":
			profile.CurrentUpload = value
		case "Aktuális letöltés:":
			profile.CurrentDownload = value
		case "Pontok száma:":
			profile.Points, _ = strconv.Atoi(strings.ReplaceAll(value, " ", ""))
		}
	})
	doc.Find(".lista_mini_fej").Each(func(i int, s *goquery.Selection) {
		if matches := regexp.MustCompile(`\((\d+)\)`).FindStringSubmatch(s.Text()); len(matches) > 1 {
			fmt.Sscanf(matches[1], "%d", &profile.SeedingCount)
		}
	})
	return profile, nil
}
