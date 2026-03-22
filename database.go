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
		`CREATE TABLE IF NOT EXISTS profile_history (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, timestamp DATETIME, rank INTEGER, upload TEXT, current_upload TEXT, current_download TEXT, points INTEGER, seeding_count INTEGER, FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);`,
	}
	for _, s := range schemas {
		if _, err := db.Exec(s); err != nil {
			logrus.Fatalf("Schema error: %v", err)
		}
	}
	return db
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
