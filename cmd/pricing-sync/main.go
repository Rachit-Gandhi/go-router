package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	dbquery "github.com/Rachit-Gandhi/go-router/db/query"
	"github.com/Rachit-Gandhi/go-router/internal/pricing"
	"github.com/Rachit-Gandhi/go-router/internal/store"
)

func main() {
	dsn := stringsTrimFirstNonEmpty(os.Getenv("PRICING_DB_DSN"), os.Getenv("CONTROL_DB_DSN"))
	if dsn == "" {
		log.Fatal("PRICING_DB_DSN or CONTROL_DB_DSN is required")
	}

	db, err := store.OpenPostgres(context.Background(), dsn)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	queries := dbquery.New(db)
	runLoop, err := parseBoolEnv("PRICING_SYNC_LOOP", false)
	if err != nil {
		log.Fatal(err)
	}

	interval := 24 * time.Hour
	if raw := os.Getenv("PRICING_SYNC_INTERVAL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			log.Fatalf("invalid PRICING_SYNC_INTERVAL: %v", err)
		}
		if parsed <= 0 {
			log.Fatal("PRICING_SYNC_INTERVAL must be > 0")
		}
		interval = parsed
	}

	runOnce := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		result, err := pricing.SyncQuotes(ctx, queries, time.Now().UTC(), pricing.DefaultFetchers())
		if err != nil {
			return err
		}
		log.Printf("pricing sync complete: created=%d updated=%d skipped=%d fetch_failures=%d", result.Created, result.Updated, result.Skipped, result.Failed)
		return nil
	}

	if err := runOnce(); err != nil {
		log.Fatal(err)
	}
	if !runLoop {
		return
	}

	log.Printf("pricing sync loop enabled; next run every %s", interval)
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := runOnce(); err != nil {
				log.Printf("pricing sync failed: %v", err)
			}
		case <-signalCh:
			log.Print("pricing sync loop stopped")
			return
		}
	}
}

func parseBoolEnv(key string, fallback bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errors.New("invalid " + key + " value")
	}
	return v, nil
}

func stringsTrimFirstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
