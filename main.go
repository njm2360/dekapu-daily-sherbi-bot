package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/bot"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/dailyhistory"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/db"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/detector"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/settings"
	"github.com/njm2360/dekapu-daily-sherbi-bot/internal/watcher"
)

func main() {
	_ = godotenv.Load()

	const dataDir = "data"

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("mkdir data: %v", err)
	}

	conn, err := db.Open(filepath.Join(dataDir, "data.db"))
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

	b, err := bot.New(settingsRepo, historyRepo, token)
	if err != nil {
		log.Fatalf("bot.New: %v", err)
	}

	det, err := detector.New(historyRepo, b.Notifier())
	if err != nil {
		log.Fatalf("init detector: %v", err)
	}

	stateRepo := watcher.NewFileRepo(filepath.Join(dataDir, "state.json"))
	newHandler := func(_ string) watcher.LineHandler {
		return func(_, line string) {
			det.OnLine(line)
		}
	}
	w := watcher.NewLogWatcher(logDir, newHandler, stateRepo, true)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	wg.Go(func() {
		select {
		case <-ctx.Done():
			return
		case <-b.Ready():
		}
		_ = w.Run(ctx)
	})

	err = b.Run(ctx)
	stop()
	wg.Wait()
	if err != nil {
		log.Fatalf("bot.Run: %v", err)
	}
}
