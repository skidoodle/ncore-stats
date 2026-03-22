package main

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Configuration holds application settings.
type Configuration struct {
	ServerPort   string
	DatabasePath string
	LogLevel     logrus.Level
	Ncore        struct {
		Nick string
		Pass string
	}
}

// ProfileData represents a snapshot of a user's profile statistics.
type ProfileData struct {
	Owner           string    `json:"owner"`
	Timestamp       time.Time `json:"timestamp"`
	Rank            int       `json:"rank"`
	Upload          string    `json:"upload"`
	UploadBytes     int64     `json:"upload_bytes"`
	CurrentUpload   string    `json:"current_upload"`
	CurrentDownload string    `json:"current_download"`
	Points          int       `json:"points"`
	SeedingCount    int       `json:"seeding_count"`
}

// User represents a tracked user.
type User struct {
	ID          int
	DisplayName string
	ProfileID   string
}

type State struct {
	config *Configuration
	db     *sql.DB
	client *http.Client
}

// CompactHistory represents an optimized, columnar history format.
type CompactHistory struct {
	Owner     string    `json:"owner"`
	Timestamp []int64   `json:"t"`
	Rank      []int     `json:"r"`
	Upload    []float64 `json:"u"`
	Points    []int     `json:"p"`
	Seeding   []int     `json:"s"`
}
