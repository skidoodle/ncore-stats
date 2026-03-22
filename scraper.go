package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

func (s *State) worker(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	s.scrapeAll(ctx)

	for {
		select {
		case <-ticker.C:
			s.scrapeAll(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *State) scrapeAll(ctx context.Context) {
	rows, err := s.db.Query("SELECT id, display_name, profile_id FROM users")
	if err != nil {
		logrus.Errorf("User query failed: %v", err)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.DisplayName, &u.ProfileID)
		users = append(users, u)
	}

	if len(users) == 0 {
		return
	}

	logrus.Infof("Starting concurrent scrape for %d users", len(users))

	var wg sync.WaitGroup
	for _, u := range users {
		wg.Add(1)
		go func(user User) {
			defer wg.Done()

			time.Sleep(time.Duration(100+(user.ID%500)) * time.Millisecond)

			profile, err := s.fetchProfile(user)
			if err != nil {
				logrus.Errorf("[%s] Fetch failed: %v", user.DisplayName, err)
				return
			}

			_, err = s.db.Exec(`INSERT INTO profile_history(user_id, timestamp, rank, upload, current_upload, current_download, points, seeding_count) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
				user.ID, profile.Timestamp, profile.Rank, profile.Upload, profile.CurrentUpload, profile.CurrentDownload, profile.Points, profile.SeedingCount)
			if err != nil {
				logrus.Errorf("[%s] DB log failed: %v", user.DisplayName, err)
			} else {
				logrus.Infof("[%s] Metrics recorded", user.DisplayName)
			}
		}(u)
	}

	wg.Wait()
	logrus.Info("Scrape cycle complete")
}

func (s *State) fetchProfile(user User) (*ProfileData, error) {
	req, _ := http.NewRequest("GET", ncoreBaseURL+user.ProfileID, nil)
	req.Header.Set("Cookie", fmt.Sprintf("nick=%s; pass=%s", s.config.Ncore.Nick, s.config.Ncore.Pass))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	p := &ProfileData{Owner: user.DisplayName, Timestamp: time.Now()}
	doc.Find(".userbox_tartalom_mini .profil_jobb_elso2").Each(func(i int, sel *goquery.Selection) {
		label, value := sel.Text(), sel.Next().Text()
		switch label {
		case "Helyezés:":
			p.Rank, _ = strconv.Atoi(strings.TrimSuffix(value, "."))
		case "Feltöltés:":
			p.Upload = value
		case "Pontok száma:":
			p.Points, _ = strconv.Atoi(strings.ReplaceAll(value, " ", ""))
		}
	})

	doc.Find(".lista_mini_fej").Each(func(i int, sel *goquery.Selection) {
		text := sel.Text()
		if m := regexp.MustCompile(`\((\d+)\)`).FindStringSubmatch(text); len(m) > 1 {
			p.SeedingCount, _ = strconv.Atoi(m[1])
		}
		if m := regexp.MustCompile(`fel: ([\d.]+ \w+/s)`).FindStringSubmatch(text); len(m) > 1 {
			p.CurrentUpload = m[1]
		}
		if m := regexp.MustCompile(`le: ([\d.]+ \w+/s)`).FindStringSubmatch(text); len(m) > 1 {
			p.CurrentDownload = m[1]
		}
	})

	return p, nil
}
