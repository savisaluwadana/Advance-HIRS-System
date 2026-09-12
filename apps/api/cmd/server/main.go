package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/auth"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/httpapi"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/api/internal/store"
)

func main() {
	ctx := context.Background()
	dataStore, err := openStoreWithRetry(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("database startup failed: %v", err)
	}
	defer dataStore.Close()

	authManager, err := auth.NewManager(os.Getenv("JWT_SECRET"), 8*time.Hour)
	if err != nil {
		log.Fatalf("auth configuration failed: %v", err)
	}

	if strings.TrimSpace(os.Getenv("BOOTSTRAP_ORG_SLUG")) != "" {
		seedDemo := strings.EqualFold(os.Getenv("SEED_DEMO_DATA"), "true")
		orgID, err := dataStore.BootstrapV2(
			ctx,
			envOr("BOOTSTRAP_ORG_NAME", "Northstar Labs"),
			os.Getenv("BOOTSTRAP_ORG_SLUG"),
			envOr("BOOTSTRAP_ADMIN_NAME", "HRIS Administrator"),
			os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
			os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
			seedDemo,
		)
		if err != nil {
			log.Fatalf("bootstrap failed: %v", err)
		}
		if seedDemo && strings.TrimSpace(os.Getenv("SEED_DEMO_PASSWORD")) != "" {
			if err := dataStore.SeedDemoAccess(ctx, orgID, os.Getenv("SEED_DEMO_PASSWORD")); err != nil {
				log.Fatalf("demo access bootstrap failed: %v", err)
			}
		}
	}

	port := envOr("API_PORT", "8080")
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewHandlerV4(dataStore, authManager, envOr("WEB_ORIGIN", "http://localhost:3000")),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("advance HRIS API listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}

func openStoreWithRetry(ctx context.Context, dsn string) (*store.Store, error) {
	var lastErr error
	for attempt := 1; attempt <= 12; attempt++ {
		dataStore, err := store.Open(ctx, dsn)
		if err == nil {
			return dataStore, nil
		}
		lastErr = err
		log.Printf("database not ready (attempt %d/12): %v", attempt, err)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}
	return nil, lastErr
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
