package main

import (
	"database/sql"
	"fmt"
	"os"

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
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, display_name TEXT UNIQUE, profile_id TEXT);`,
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
	var columnExists bool
	_ = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('profile_history') WHERE name='upload_bytes'").Scan(&columnExists)
	if !columnExists {
		logrus.Info("Migrating: Adding upload_bytes column...")
		_, err := db.Exec("ALTER TABLE profile_history ADD COLUMN upload_bytes INTEGER")
		if err != nil {
			logrus.Errorf("Migration failed (add column): %v", err)
			return
		}

		logrus.Info("Migrating: Backfilling upload_bytes from existing strings...")

		type updateRow struct {
			id    int64
			bytes int64
		}
		var updates []updateRow

		rows, err := db.Query("SELECT id, upload FROM profile_history WHERE upload_bytes IS NULL")
		if err == nil {
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
			tx, err := db.Begin()
			if err != nil {
				logrus.Errorf("Transaction start failed: %v", err)
				return
			}
			stmt, _ := tx.Prepare("UPDATE profile_history SET upload_bytes = ? WHERE id = ?")
			for _, up := range updates {
				_, _ = stmt.Exec(up.bytes, up.id)
			}
			stmt.Close()
			if err := tx.Commit(); err != nil {
				logrus.Errorf("Transaction commit failed: %v", err)
			}
		}
		logrus.Info("Migration complete.")
	}
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
