package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

// Configuration holds application settings loaded from the environment.
type Configuration struct {
	ServerPort   string
	DatabasePath string
	LogLevel     logrus.Level
	Ncore        struct {
		Nick string
		Pass string
	}
}

// State holds application runtime state and dependencies.
type State struct {
	config *Configuration
	db     *sql.DB
	client *http.Client
}

// ProfileData represents a snapshot of a user's profile statistics.
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

// User represents a user whose stats are being tracked.
type User struct {
	ID          int
	DisplayName string
	ProfileID   string
}

const (
	defaultPort     = ":3000"
	defaultDbFolder = "./data"
	ncoreBaseURL    = "https://ncore.pro/profile.php?id="
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := initializeApplication()

	db, err := initializeDatabase(config)
	if err != nil {
		logrus.Fatalf("Database initialization failed: %v", err)
	}
	defer db.Close()

	state := &State{
		config: config,
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}

	// If a command-line flag was handled, the program should exit.
	if handleFlags(state) {
		return
	}

	router := http.NewServeMux()
	router.HandleFunc("/api/profiles", state.profilesHandler)
	router.HandleFunc("/api/history", state.historyHandler)
	router.Handle("/", http.FileServer(http.Dir("web")))

	server := &http.Server{
		Addr:    config.ServerPort,
		Handler: router,
	}

	go state.profileFetcherLoop(ctx)

	startServer(server)
	handleShutdown(server, cancel)
}

// initializeApplication sets up logging and loads the application configuration.
func initializeApplication() *Configuration {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
		ForceColors:     true,
	})

	config, err := loadConfiguration()
	if err != nil {
		logrus.Fatalf("Failed to load configuration: %v", err)
	}

	logrus.SetLevel(config.LogLevel)
	logrus.Info("Application configuration loaded successfully")
	return config
}

// loadConfiguration loads settings from .env files and the environment.
func loadConfiguration() (*Configuration, error) {
	// godotenv.Load will not override existing environment variables,
	// making it safe for use in production environments like Docker.
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	cfg := &Configuration{}

	required := map[string]*string{
		"NICK": &cfg.Ncore.Nick,
		"PASS": &cfg.Ncore.Pass,
	}
	for key, ptr := range required {
		value := os.Getenv(key)
		if value == "" {
			return nil, fmt.Errorf("missing required environment variable: %s", key)
		}
		*ptr = value
	}

	cfg.ServerPort = defaultPort
	if port := os.Getenv("SERVER_PORT"); port != "" {
		cfg.ServerPort = ":" + strings.TrimLeft(port, ":")
	}

	cfg.DatabasePath = defaultDbFolder
	if path := os.Getenv("DATABASE_PATH"); path != "" {
		cfg.DatabasePath = path
	}

	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		cfg.LogLevel = logrus.DebugLevel
	case "warn":
		cfg.LogLevel = logrus.WarnLevel
	case "error":
		cfg.LogLevel = logrus.ErrorLevel
	default:
		cfg.LogLevel = logrus.InfoLevel
	}

	return cfg, nil
}

// initializeDatabase connects to the SQLite database and ensures tables are created.
func initializeDatabase(config *Configuration) (*sql.DB, error) {
	dbPath := fmt.Sprintf("%s/ncore_stats.db", config.DatabasePath)

	if err := os.MkdirAll(config.DatabasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	usersTableSQL := `CREATE TABLE IF NOT EXISTS users ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "display_name" TEXT NOT NULL UNIQUE, "profile_id" TEXT NOT NULL);`
	if _, err := db.Exec(usersTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create users table: %w", err)
	}

	profileHistoryTableSQL := `CREATE TABLE IF NOT EXISTS profile_history ("id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, "user_id" INTEGER NOT NULL, "timestamp" DATETIME NOT NULL, "rank" INTEGER, "upload" TEXT, "current_upload" TEXT, "current_download" TEXT, "points" INTEGER, "seeding_count" INTEGER, FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);`
	if _, err := db.Exec(profileHistoryTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create profile_history table: %w", err)
	}

	logrus.Info("Database initialized successfully")
	return db, nil
}

// handleFlags processes command-line flags and returns true if the program should exit.
func handleFlags(s *State) bool {
	addUserFlag := flag.String("add-user", "", "Add a new user. Provide as 'DisplayName,ProfileID'")
	flag.Parse()

	if *addUserFlag != "" {
		parts := strings.Split(*addUserFlag, ",")
		if len(parts) != 2 {
			logrus.Fatal("Invalid format for --add-user. Use 'DisplayName,ProfileID'")
		}
		s.addUser(parts[0], parts[1])
		return true
	}
	return false
}

// startServer runs the HTTP server in a new goroutine.
func startServer(server *http.Server) {
	go func() {
		logrus.Infof("Server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Server failed to start: %v", err)
		}
	}()
}

// handleShutdown waits for a termination signal and performs a graceful shutdown.
func handleShutdown(server *http.Server, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logrus.Info("Shutdown signal received, initiating graceful shutdown...")
	cancel() // Notify background goroutines to stop.

	shutdownCtx, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logrus.Errorf("Server shutdown error: %v", err)
	}
	logrus.Info("Server shutdown complete")
}

// profilesHandler serves the latest profile data for all tracked users.
func (s *State) profilesHandler(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.db.Query(query)
	if err != nil {
		http.Error(w, "Could not read latest profiles from database", http.StatusInternalServerError)
		logrus.Errorf("Error querying latest profiles: %v", err)
		return
	}
	defer rows.Close()

	var latestProfiles []ProfileData
	for rows.Next() {
		var p ProfileData
		if err := rows.Scan(&p.Owner, &p.Timestamp, &p.Rank, &p.Upload, &p.CurrentUpload, &p.CurrentDownload, &p.Points, &p.SeedingCount); err != nil {
			http.Error(w, "Could not process profile data", http.StatusInternalServerError)
			logrus.Errorf("Error scanning latest profile row: %v", err)
			return
		}
		latestProfiles = append(latestProfiles, p)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(latestProfiles); err != nil {
		logrus.Errorf("Error encoding latest profiles to JSON: %v", err)
	}
}

// historyHandler serves the full profile history for a single user.
func (s *State) historyHandler(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.db.Query(query, owner)
	if err != nil {
		http.Error(w, "Could not read history from database", http.StatusInternalServerError)
		logrus.Errorf("Error querying history for %s: %v", owner, err)
		return
	}
	defer rows.Close()

	var userHistory []ProfileData
	for rows.Next() {
		var p ProfileData
		if err := rows.Scan(&p.Owner, &p.Timestamp, &p.Rank, &p.Upload, &p.CurrentUpload, &p.CurrentDownload, &p.Points, &p.SeedingCount); err != nil {
			http.Error(w, "Could not process history data", http.StatusInternalServerError)
			logrus.Errorf("Error scanning history row for %s: %v", owner, err)
			return
		}
		userHistory = append(userHistory, p)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(userHistory); err != nil {
		logrus.Errorf("Error encoding history for %s to JSON: %v", err, owner)
	}
}

// serveStatic returns an http.HandlerFunc that serves a static file.
func serveStatic(fileName, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, fileName)
	}
}

// profileFetcherLoop runs a background task to fetch profiles on a schedule.
func (s *State) profileFetcherLoop(ctx context.Context) {
	logrus.Info("Starting background profile fetcher...")
	s.fetchAndLogAllProfiles() // Fetch immediately on startup.

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.fetchAndLogAllProfiles()
		case <-ctx.Done():
			logrus.Info("Stopping background profile fetcher.")
			return
		}
	}
}

// addUser inserts a new user into the database.
func (s *State) addUser(displayName, profileID string) {
	stmt, err := s.db.Prepare("INSERT INTO users(display_name, profile_id) VALUES(?, ?)")
	if err != nil {
		logrus.Fatalf("Failed to prepare statement for adding user: %v", err)
	}
	defer stmt.Close()

	if _, err = stmt.Exec(displayName, profileID); err != nil {
		logrus.Fatalf("Failed to add user %s: %v", displayName, err)
	}
	logrus.Infof("User '%s' with profile ID '%s' added successfully.", displayName, profileID)
}

// getUsers retrieves all tracked users from the database.
func (s *State) getUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, display_name, profile_id FROM users")
	if err != nil {
		return nil, fmt.Errorf("error querying users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.DisplayName, &u.ProfileID); err != nil {
			return nil, fmt.Errorf("error scanning user row: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

// logToDB inserts a new profile data point into the history table.
func (s *State) logToDB(profile *ProfileData, userID int) error {
	stmt, err := s.db.Prepare(`INSERT INTO profile_history(user_id, timestamp, rank, upload, current_upload, current_download, points, seeding_count) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("error preparing insert statement: %w", err)
	}
	defer stmt.Close()

	if _, err = stmt.Exec(userID, profile.Timestamp, profile.Rank, profile.Upload, profile.CurrentUpload, profile.CurrentDownload, profile.Points, profile.SeedingCount); err != nil {
		return fmt.Errorf("error executing insert for %s: %w", profile.Owner, err)
	}
	logrus.Infof("Profile for %s logged successfully to database.", profile.Owner)
	return nil
}

// fetchAndLogAllProfiles orchestrates the fetching and logging of all user profiles.
func (s *State) fetchAndLogAllProfiles() {
	users, err := s.getUsers()
	if err != nil {
		logrus.Errorf("Could not get users to fetch: %v", err)
		return
	}

	if len(users) == 0 {
		logrus.Info("No users in database to fetch. Use the --add-user flag to add one.")
		return
	}

	logrus.Infof("Starting profile fetch for %d user(s).", len(users))
	for _, user := range users {
		profile, err := s.fetchProfile(user)
		if err != nil {
			logrus.Errorf("Error fetching profile for %s: %v", user.DisplayName, err)
			continue
		}
		if err := s.logToDB(profile, user.ID); err != nil {
			logrus.Errorf("Error logging profile to DB for %s: %v", user.DisplayName, err)
		}
		// Pause between requests to avoid rate-limiting.
		time.Sleep(2 * time.Second)
	}
	logrus.Info("Profile fetch cycle complete.")
}

// fetchProfile retrieves and parses the profile page for a single user.
func (s *State) fetchProfile(user User) (*ProfileData, error) {
	profileURL := ncoreBaseURL + user.ProfileID
	req, err := http.NewRequest("GET", profileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Cookie", fmt.Sprintf("nick=%s; pass=%s", s.config.Ncore.Nick, s.config.Ncore.Pass))
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error performing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing profile document: %w", err)
	}

	profile := &ProfileData{Owner: user.DisplayName, Timestamp: time.Now()}
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
