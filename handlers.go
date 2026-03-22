package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

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
	rows, err := s.db.Query(`SELECT ph.timestamp, ph.rank, ph.upload, ph.points, ph.seeding_count FROM profile_history ph JOIN users u ON ph.user_id = u.id WHERE u.display_name = ? ORDER BY ph.timestamp ASC`, owner)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []ProfileData
	for rows.Next() {
		var p ProfileData
		if err := rows.Scan(&p.Timestamp, &p.Rank, &p.Upload, &p.Points, &p.SeedingCount); err != nil {
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

	rows, err := s.db.Query(`SELECT ph.timestamp, ph.rank, ph.upload_bytes, ph.points, ph.seeding_count
	          FROM profile_history ph
	          JOIN users u ON ph.user_id = u.id
	          WHERE u.display_name = ?
	          ORDER BY ph.timestamp ASC`, owner)
	if err != nil {
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	res := CompactHistory{Owner: owner}
	for rows.Next() {
		var (
			ts          time.Time
			rank        int
			uploadBytes int64
			points      int
			seeding     int
		)
		if err := rows.Scan(&ts, &rank, &uploadBytes, &points, &seeding); err == nil {
			res.Timestamp = append(res.Timestamp, ts.Unix()*1000)
			res.Rank = append(res.Rank, rank)

			tib := float64(uploadBytes) / (1024 * 1024 * 1024 * 1024)
			res.Upload = append(res.Upload, tib)

			res.Points = append(res.Points, points)
			res.Seeding = append(res.Seeding, seeding)
		}
	}

	dataJSON, _ := json.Marshal(res)

	fmt.Fprintf(w, `
		<div id="chart-mount"
			 style="height: 100%%; width: 100%%;"
			 x-init='renderChart(%s)'>
		</div>`, string(dataJSON))
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
