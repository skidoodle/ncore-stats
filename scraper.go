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
		if err := rows.Scan(&u.ID, &u.DisplayName, &u.ProfileID); err != nil {
			logrus.Errorf("User scan failed: %v", err)
			continue
		}
		users = append(users, u)
	}

	if len(users) == 0 {
		return
	}

	logrus.Infof("Starting concurrent scrape for %d users", len(users))

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, u := range users {
		select {
		case <-ctx.Done():
			logrus.Info("Scrape cycle cancelled by context")
			return
		case sem <- struct{}{}:
			wg.Add(1)
			go func(user User) {
				defer wg.Done()
				defer func() { <-sem }()

				time.Sleep(time.Duration(200+(user.ID%1000)) * time.Millisecond)

				profile, err := s.fetchProfile(ctx, user)
				if err != nil {
					logrus.Errorf("[%s] Fetch failed: %v", user.DisplayName, err)
					return
				}

				_, err = s.db.Exec(`INSERT INTO profile_history(user_id, timestamp, rank, upload, upload_bytes, current_upload, current_download, points, seeding_count) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					user.ID, profile.Timestamp, profile.Rank, profile.Upload, profile.UploadBytes, profile.CurrentUpload, profile.CurrentDownload, profile.Points, profile.SeedingCount)
				if err != nil {
					logrus.Errorf("[%s] DB log failed: %v", user.DisplayName, err)
				} else {
					logrus.Infof("[%s] Metrics recorded", user.DisplayName)
				}
			}(u)
		}
	}

	wg.Wait()
	logrus.Info("Scrape cycle complete")
}

func (s *State) fetchProfile(ctx context.Context, user User) (*ProfileData, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", ncoreBaseURL+user.ProfileID, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Cookie", fmt.Sprintf("nick=%s; pass=%s", s.config.Ncore.Nick, s.config.Ncore.Pass))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse document: %w", err)
	}

	p := &ProfileData{Owner: user.DisplayName, Timestamp: time.Now()}

	doc.Find(".userbox_tartalom_mini .profil_jobb_elso2").Each(func(i int, sel *goquery.Selection) {
		label := strings.ToLower(sel.Text())
		value := strings.TrimSpace(sel.Next().Text())

		if strings.Contains(label, "helyezés") { // Rank
			p.Rank, _ = strconv.Atoi(strings.TrimSuffix(value, "."))
		} else if strings.Contains(label, "feltöltés") { // Upload
			p.Upload = value
			p.UploadBytes = parseToBytes(value)
		} else if strings.Contains(label, "pontok") { // Points
			p.Points, _ = strconv.Atoi(strings.ReplaceAll(value, " ", ""))
		}
	})

	doc.Find(".lista_mini_fej").Each(func(i int, sel *goquery.Selection) {
		text := sel.Text()
		if strings.Contains(text, "seeding") || strings.Contains(text, "futó") {
			if m := regexp.MustCompile(`\((\d+)\)`).FindStringSubmatch(text); len(m) > 1 {
				p.SeedingCount, _ = strconv.Atoi(m[1])
			}
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

func parseToBytes(value string) int64 {
	valStr := strings.ReplaceAll(value, ",", "")
	parts := strings.Fields(valStr)
	if len(parts) < 2 {
		return 0
	}
	num, _ := strconv.ParseFloat(parts[0], 64)
	unit := strings.ToLower(parts[1])

	var multiplier float64
	switch unit {
	case "tib":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "gib":
		multiplier = 1024 * 1024 * 1024
	case "mib":
		multiplier = 1024 * 1024
	case "kib":
		multiplier = 1024
	default:
		multiplier = 1
	}
	return int64(num * multiplier)
}
