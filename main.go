package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/joho/godotenv"
)

type ProfileData struct {
	Owner           string    `json:"owner"`
	Timestamp       time.Time `json:"timestamp"`
	Rank            string    `json:"rank"`
	Upload          string    `json:"upload"`
	CurrentUpload   string    `json:"current_upload"`
	CurrentDownload string    `json:"current_download"`
	Points          string    `json:"points"`
}

var (
	profiles = map[string]string{}
	jsonFile = "./data/data.json"
	profilesFile = "profiles.json"
	baseUrl  = "https://ncore.pro/profile.php?id="
	nick     string
	pass     string
	client   *http.Client
)

func init() {
	godotenv.Load()

	nick = os.Getenv("NICK")
	pass = os.Getenv("PASS")

	file, err := os.Open(profilesFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	jsonBytes, _ := io.ReadAll(file)

	var tempProfiles map[string]string = make(map[string]string)
	json.Unmarshal(jsonBytes, &tempProfiles)

	for k, v := range tempProfiles {
		profiles[k] = baseUrl + v
	}

	client = &http.Client{}
}

func fetchProfile(url string, displayName string) (*ProfileData, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Cookie", fmt.Sprintf("nick=%s; pass=%s", nick, pass))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch profile: %s", displayName)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	profile := &ProfileData{
		Owner:     displayName,
		Timestamp: time.Now(),
	}

	doc.Find(".userbox_tartalom_mini .profil_jobb_elso2").Each(func(i int, s *goquery.Selection) {
		label := s.Text()
		value := s.Next().Text()

		switch label {
		case "Helyezés:":
			profile.Rank = value
		case "Feltöltés:":
			profile.Upload = value
		case "Aktuális feltöltés:":
			profile.CurrentUpload = value
		case "Aktuális letöltés:":
			profile.CurrentDownload = value
		case "Pontok száma:":
			profile.Points = value
		}
	})

	return profile, nil
}

func readExistingProfiles() ([]ProfileData, error) {
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		return []ProfileData{}, nil
	}

	file, err := os.Open(jsonFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var existingProfiles []ProfileData
	byteValue, _ := io.ReadAll(file)
	err = json.Unmarshal(byteValue, &existingProfiles)
	return existingProfiles, err
}

func logToJSON(profile *ProfileData) error {
	existingProfiles, err := readExistingProfiles()
	if err != nil {
		return err
	}

	existingProfiles = append(existingProfiles, *profile)

	file, err := os.OpenFile(jsonFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(existingProfiles)
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	existingProfiles, err := readExistingProfiles()
	if err != nil {
		http.Error(w, "Could not read data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existingProfiles)
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func main() {
	ticker := time.NewTicker(time.Minute * 30)
	defer ticker.Stop()

	go func() {
		for {
			for displayName, url := range profiles {
				profile, err := fetchProfile(url, displayName)
				if err != nil {
					log.Println(err)
					continue
				}

				if err := logToJSON(profile); err != nil {
					log.Println(err)
				}
			}
			<-ticker.C
		}
	}()

	http.HandleFunc("/data", dataHandler)
	http.HandleFunc("/", serveHTML)
	log.Fatal(http.ListenAndServe(":3000", nil))
}
