package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

const (
	defaultPort     = ":3000"
	defaultDbFolder = "./data"
	ncoreBaseURL    = "https://ncore.pro/profile.php?id="
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config := loadConfig()
	db := initDB(config)
	defer db.Close()

	state := &State{
		config: config,
		db:     db,
		client: &http.Client{Timeout: 45 * time.Second},
	}

	state.syncUsers()

	if handleFlags(state) {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/profiles", state.profilesHandler)
	mux.HandleFunc("/api/history", state.historyHandler)
	mux.HandleFunc("/api/history-modal", state.historyModalHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))
	mux.HandleFunc("/", state.rootHandler)

	server := &http.Server{
		Addr:    config.ServerPort,
		Handler: mux,
	}

	go state.worker(ctx)

	go func() {
		logrus.Infof("Server active on %s", config.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("Server failure: %v", err)
		}
	}()

	<-ctx.Done()
	logrus.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logrus.Errorf("Shutdown error: %v", err)
	}
}

func handleFlags(s *State) bool {
	addUser := flag.String("add-user", "", "Format: DisplayName,ProfileID")
	flag.Parse()
	if *addUser != "" {
		parts := strings.Split(*addUser, ",")
		if len(parts) == 2 {
			_, err := s.db.Exec("INSERT INTO users(display_name, profile_id) VALUES(?, ?)", parts[0], parts[1])
			if err != nil {
				logrus.Fatalf("Add user failed: %v", err)
			}
			logrus.Infof("User %s added", parts[0])
		}
		return true
	}
	return false
}
