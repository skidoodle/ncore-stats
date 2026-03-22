package main

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func loadConfig() *Configuration {
	_ = godotenv.Load()
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, ForceColors: true})

	cfg := &Configuration{}
	cfg.Ncore.Nick = os.Getenv("NICK")
	cfg.Ncore.Pass = os.Getenv("PASS")
	if cfg.Ncore.Nick == "" || cfg.Ncore.Pass == "" {
		logrus.Fatal("NICK and PASS environment variables are required")
	}

	cfg.ServerPort = os.Getenv("SERVER_PORT")
	if cfg.ServerPort == "" {
		cfg.ServerPort = defaultPort
	} else if !strings.HasPrefix(cfg.ServerPort, ":") {
		cfg.ServerPort = ":" + cfg.ServerPort
	}

	cfg.DatabasePath = os.Getenv("DATABASE_PATH")
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = defaultDbFolder
	}

	lvl, _ := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if lvl == 0 {
		lvl = logrus.InfoLevel
	}
	logrus.SetLevel(lvl)
	return cfg
}
