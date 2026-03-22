package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

func initDB(cfg *Configuration) *sql.DB {
	_ = os.MkdirAll(cfg.DatabasePath, 0755)
	db, err := sql.Open("sqlite", fmt.Sprintf("%s/ncore_stats.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", cfg.DatabasePath))
	if err != nil {
		logrus.Fatalf("DB failed: %v", err)
	}

	db.SetMaxOpenConns(1)

	schemas := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			display_name TEXT UNIQUE,
			profile_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS profile_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			timestamp DATETIME,
			rank INTEGER,
			upload TEXT,
			upload_bytes INTEGER,
			current_upload TEXT,
			current_download TEXT,
			points INTEGER,
			seeding_count INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_history_user_ts ON profile_history(user_id, timestamp);`,
	}
	for _, s := range schemas {
		if _, err := db.Exec(s); err != nil {
			logrus.Fatalf("Schema error: %v", err)
		}
	}

	migrate(db)

	return db
}

func migrate(db *sql.DB) {
	var ubExists bool
	_ = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('profile_history') WHERE name='upload_bytes'").Scan(&ubExists)
	if !ubExists {
		logrus.Info("Migrating: Adding upload_bytes column...")
		_, _ = db.Exec("ALTER TABLE profile_history ADD COLUMN upload_bytes INTEGER")

		type updateRow struct {
			id    int64
			bytes int64
		}
		var updates []updateRow
		rows, _ := db.Query("SELECT id, upload FROM profile_history WHERE upload_bytes IS NULL")
		if rows != nil {
			for rows.Next() {
				var id int64
				var upload string
				if err := rows.Scan(&id, &upload); err == nil {
					updates = append(updates, updateRow{id: id, bytes: parseToBytes(upload)})
				}
			}
			rows.Close()
		}
		if len(updates) > 0 {
			tx, _ := db.Begin()
			stmt, _ := tx.Prepare("UPDATE profile_history SET upload_bytes = ? WHERE id = ?")
			for _, up := range updates {
				_, _ = stmt.Exec(up.bytes, up.id)
			}
			stmt.Close()
			_ = tx.Commit()
		}
	}
	var caExists bool
	_ = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='created_at'").Scan(&caExists)
	if !caExists {
		logrus.Info("Migrating: Adding created_at column to users...")
		_, err := db.Exec("ALTER TABLE users ADD COLUMN created_at DATETIME")
		if err != nil {
			logrus.Errorf("User migration failed (add column): %v", err)
		} else {
			_, _ = db.Exec("UPDATE users SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL")
			logrus.Info("User migration complete.")
		}
	}
}

func (s *State) syncUsers() {
	if _, err := os.Stat(s.config.UsersPath); os.IsNotExist(err) {
		logrus.Warnf("Users config file not found at %s, skipping sync", s.config.UsersPath)
		return
	}

	content, err := os.ReadFile(s.config.UsersPath)
	if err != nil {
		logrus.Errorf("Failed to read users file: %v", err)
		return
	}

	type userEntry struct {
		Name string
		ID   string
	}
	var users []userEntry

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			users = append(users, userEntry{
				Name: strings.TrimSpace(parts[0]),
				ID:   strings.TrimSpace(parts[1]),
			})
		}
	}

	for _, u := range users {
		_, err := s.db.Exec(`
			INSERT INTO users (display_name, profile_id)
			VALUES (?, ?)
			ON CONFLICT(display_name) DO UPDATE SET profile_id = excluded.profile_id`,
			u.Name, u.ID)
		if err != nil {
			logrus.Errorf("Sync failed for %s: %v", u.Name, err)
		}
	}

	rows, _ := s.db.Query("SELECT display_name FROM users")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			rows.Scan(&name)

			found := false
			for _, u := range users {
				if u.Name == name {
					found = true
					break
				}
			}

			if !found {
				logrus.Infof("Sync: Removing %s (not in config)", name)
				_, _ = s.db.Exec("DELETE FROM users WHERE display_name = ?", name)
			}
		}
	}
	logrus.Infof("User synchronization complete (%d users)", len(users))
}

func (s *State) getLatest() ([]ProfileData, error) {
	query := `
	SELECT u.display_name, ph.timestamp, ph.rank, ph.upload, ph.current_upload, ph.current_download, ph.points, ph.seeding_count
	FROM profile_history ph
	INNER JOIN (SELECT user_id, MAX(timestamp) as ts FROM profile_history GROUP BY user_id) latest
	ON ph.user_id = latest.user_id AND ph.timestamp = latest.ts
	JOIN users u ON ph.user_id = u.id
	ORDER BY u.id ASC;`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []ProfileData
	for rows.Next() {
		var p ProfileData
		rows.Scan(&p.Owner, &p.Timestamp, &p.Rank, &p.Upload, &p.CurrentUpload, &p.CurrentDownload, &p.Points, &p.SeedingCount)
		res = append(res, p)
	}
	return res, nil
}
