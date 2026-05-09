package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/bot"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/dailyhistory"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/db"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/settings"
)

func main() {
	_ = godotenv.Load()

	if err := os.MkdirAll("data", 0o755); err != nil {
		log.Fatalf("mkdir data: %v", err)
	}

	conn, err := db.Open(filepath.Join("data", "data.db"))
	if err != nil {
		log.Fatalf("db.Open: %v", err)
	}
	defer conn.Close()

	settingsRepo := settings.New(conn)
	historyRepo := dailyhistory.New(conn)

	logDir := os.Getenv("LOG_DIR")
	if logDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("UserHomeDir: %v", err)
		}
		logDir = filepath.Join(home, "AppData", "LocalLow", "VRChat", "VRChat")
	}

	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_BOT_TOKEN is required")
	}

	b, err := bot.New(settingsRepo, historyRepo, token, logDir, "data")
	if err != nil {
		log.Fatalf("bot.New: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := b.Run(ctx); err != nil {
		log.Fatalf("bot.Run: %v", err)
	}
}
