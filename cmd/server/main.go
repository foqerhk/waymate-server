package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/foqerhk/waymate-server/internal/api"
	"github.com/foqerhk/waymate-server/internal/config"
	"github.com/foqerhk/waymate-server/internal/db"
	"github.com/foqerhk/waymate-server/internal/push"
	"github.com/foqerhk/waymate-server/internal/ws"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	migrationDir := os.Getenv("MIGRATION_DIR")
	if migrationDir == "" {
		migrationDir = "migrations"
	}
	for _, name := range []string{"001_init.sql", "002_permanent_invite.sql", "003_calls.sql"} {
		path := filepath.Join(migrationDir, name)
		if err := db.Migrate(ctx, pool, path); err != nil {
			log.Fatalf("migrate %s: %v", name, err)
		}
	}

	store := db.NewStore(pool)
	hub := ws.NewHub()
	pusher := push.NewFromEnv(cfg)
	push.WarmKeyPath(cfg.APNsKeyPath)
	server := api.New(cfg, store, hub, pusher)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("WayMate API listening on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	log.Println("shutdown complete")
}
