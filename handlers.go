package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
)

func (s *State) profilesHandler(w http.ResponseWriter, r *http.Request) {
	data, _ := s.getLatest()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *State) historyHandler(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	rows, _ := s.db.Query(`SELECT u.display_name, ph.timestamp, ph.rank, ph.upload, ph.current_upload, ph.current_download, ph.points, ph.seeding_count FROM profile_history ph JOIN users u ON ph.user_id = u.id WHERE u.display_name = ? ORDER BY ph.timestamp ASC`, owner)
	defer rows.Close()

	var history []ProfileData
	for rows.Next() {
		var p ProfileData
		rows.Scan(&p.Owner, &p.Timestamp, &p.Rank, &p.Upload, &p.CurrentUpload, &p.CurrentDownload, &p.Points, &p.SeedingCount)
		history = append(history, p)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func (s *State) historyModalHandler(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	fmt.Fprintf(w, `<div id="chart-data-container" data-owner="%s" x-init="renderChart('%s')"></div>`, owner, owner)
}

func (s *State) rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	latest, _ := s.getLatest()
	tmpl, _ := template.ParseFiles("web/index.html")
	tmpl.Execute(w, struct{ Profiles []ProfileData }{latest})
}
