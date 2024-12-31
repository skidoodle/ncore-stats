package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
)

type ProfileData struct {
	Owner           string    `json:"owner"`
	Timestamp       time.Time `json:"timestamp"`
	Rank            int       `json:"rank"`
	Upload          string    `json:"upload"`
	CurrentUpload   string    `json:"current_upload"`
	CurrentDownload string    `json:"current_download"`
	Points          int       `json:"points"`
	SeedingCount    int       `json:"seeding_count"`
}

var (
	profiles     = map[string]string{}
	jsonFile     = "./data/data.json"
	profilesFile = "./data/profiles.json"
	baseUrl      = "https://ncore.pro/profile.php?id="
	nick         string
	pass         string
	client       *http.Client
)

func init() {
	_ = godotenv.Load(".env.local")
	godotenv.Load()

	nick = os.Getenv("NICK")
	pass = os.Getenv("PASS")

	if _, err := os.Stat(profilesFile); os.IsNotExist(err) {
		log.Printf("File %s does not exist, creating an empty one", profilesFile)
		err := os.WriteFile(profilesFile, []byte("{}"), 0644)
		if err != nil {
			log.Fatalf("Failed to create profiles file: %v", err)
		}
	}

	loadProfiles()

	client = &http.Client{}
}

func loadProfiles() {
	file, err := os.Open(profilesFile)
	if err != nil {
		log.Fatalf("Failed to open profiles file: %v", err)
	}
	defer file.Close()

	jsonBytes, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read profiles file: %v", err)
	}

	var tempProfiles map[string]string
	err = json.Unmarshal(jsonBytes, &tempProfiles)
	if err != nil {
		log.Fatalf("Failed to unmarshal profiles JSON: %v", err)
	}

	for k, v := range tempProfiles {
		profiles[k] = baseUrl + v
	}
	log.Println("Profiles loaded successfully.")
}

func fetchProfile(url string, displayName string) (*ProfileData, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s: %v", displayName, err)
	}

	req.Header.Set("Cookie", fmt.Sprintf("nick=%s; pass=%s", nick, pass))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching profile for %s: %v", displayName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch profile %s: received status %d", displayName, resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing profile document for %s: %v", displayName, err)
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
			value = strings.TrimSuffix(value, ".")
			rank, err := strconv.Atoi(value)
			if err == nil {
				profile.Rank = rank
			}
		case "Feltöltés:":
			profile.Upload = value
		case "Aktuális feltöltés:":
			profile.CurrentUpload = value
		case "Aktuális letöltés:":
			profile.CurrentDownload = value
		case "Pontok száma:":
			points, err := strconv.Atoi(value)
			if err == nil {
				profile.Points = points
			}
		}
	})

	doc.Find(".lista_mini_fej").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		re := regexp.MustCompile(`\((\d+)\)`)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			fmt.Sscanf(matches[1], "%d", &profile.SeedingCount)
		}
	})

	return profile, nil
}

func readExistingProfiles() ([]ProfileData, error) {
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		log.Printf("File %s does not exist, returning an empty profile list.", jsonFile)
		return []ProfileData{}, nil
	}

	file, err := os.Open(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %v", jsonFile, err)
	}
	defer file.Close()

	var existingProfiles []ProfileData
	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %v", jsonFile, err)
	}

	err = json.Unmarshal(byteValue, &existingProfiles)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling profile data: %v", err)
	}
	return existingProfiles, nil
}

func logToJSON(profile *ProfileData) error {
	existingProfiles, err := readExistingProfiles()
	if err != nil {
		return err
	}

	existingProfiles = append(existingProfiles, *profile)

	file, err := os.OpenFile(jsonFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening file for writing: %v", err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	err = enc.Encode(existingProfiles)
	if err != nil {
		return fmt.Errorf("error encoding JSON data: %v", err)
	}
	log.Printf("Profile for %s logged successfully.", profile.Owner)
	return nil
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	existingProfiles, err := readExistingProfiles()
	if err != nil {
		http.Error(w, "Could not read data", http.StatusInternalServerError)
		log.Printf("Error reading profiles: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(existingProfiles)
	if err != nil {
		log.Printf("Error encoding profiles to JSON: %v", err)
	}
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func watchProfilesFile() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error creating file watcher: %v", err)
	}
	defer watcher.Close()

	err = watcher.Add(profilesFile)
	if err != nil {
		log.Fatalf("Error adding file to watcher: %v", err)
	}

	debounce := time.AfterFunc(0, func() {})

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				log.Println("Profiles file changed, reloading profiles...")
				debounce.Stop()
				debounce = time.AfterFunc(500*time.Millisecond, func() {
					loadProfiles()
					for displayName, url := range profiles {
						profile, err := fetchProfile(url, displayName)
						if err != nil {
							log.Printf("Error fetching profile for %s after file change: %v", displayName, err)
							continue
						}

						if err := logToJSON(profile); err != nil {
							log.Printf("Error logging profile for %s: %v", displayName, err)
						}
					}
				})
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}

func main() {
	ticker := time.NewTicker(time.Hour * 24)
	defer ticker.Stop()

	go func() {
		for {
			for displayName, url := range profiles {
				profile, err := fetchProfile(url, displayName)
				if err != nil {
					log.Printf("Error fetching profile %s: %v", displayName, err)
					continue
				}

				if err := logToJSON(profile); err != nil {
					log.Printf("Error logging profile for %s: %v", displayName, err)
				}
			}
			<-ticker.C
		}
	}()

	go watchProfilesFile()

	http.HandleFunc("/data", dataHandler)
	http.HandleFunc("/", serveHTML)
	log.Println("Server is starting on port 3000...")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
