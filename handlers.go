package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/sirupsen/logrus"
)

func (s *State) profilesHandler(w http.ResponseWriter, r *http.Request) {
	data, err := s.getLatest()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.Errorf("Encode profiles failed: %v", err)
	}
}

func (s *State) historyHandler(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	if owner == "" {
		http.Error(w, "Owner required", http.StatusBadRequest)
		return
	}
	rows, err := s.db.Query(`SELECT u.display_name, ph.timestamp, ph.rank, ph.upload, ph.current_upload, ph.current_download, ph.points, ph.seeding_count FROM profile_history ph JOIN users u ON ph.user_id = u.id WHERE u.display_name = ? ORDER BY ph.timestamp ASC`, owner)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []ProfileData
	for rows.Next() {
		var p ProfileData
		if err := rows.Scan(&p.Owner, &p.Timestamp, &p.Rank, &p.Upload, &p.CurrentUpload, &p.CurrentDownload, &p.Points, &p.SeedingCount); err != nil {
			logrus.Errorf("Scan history failed: %v", err)
			continue
		}
		history = append(history, p)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func (s *State) historyModalHandler(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	if owner == "" {
		http.Error(w, "Owner required", http.StatusBadRequest)
		return
	}
	fmt.Fprintf(w, `<div id="chart-data-container" data-owner="%s" x-init="renderChart('%s')"></div>`, owner, owner)
}

func (s *State) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	latest, err := s.getLatest()
	if err != nil {
		logrus.Errorf("Get latest failed: %v", err)
	}
	tmpl, err := template.ParseFiles("web/index.html")
	if err != nil {
		logrus.Errorf("Template parse failed: %v", err)
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, struct{ Profiles []ProfileData }{latest}); err != nil {
		logrus.Errorf("Template execute failed: %v", err)
	}
}
