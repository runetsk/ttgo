package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"ttgo/internal/api"
	"ttgo/internal/config"
	"ttgo/pkg/tracker/store"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := config.Load()

	s, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to init store: %v", err)
	}

	if err := s.SeedAdminIfNeeded(cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("Admin seed failed: %v", err)
	}

	srv := api.NewServer(s, api.WithCORSOrigin(cfg.CORSOrigin))

	// Start backup scheduler with context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	srv.StartBackupScheduler(ctx)
	srv.StartFailureAnalysisWorker(ctx)

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv,
		// Bound slow-client / slow-loris and runaway request handling (F-030).
		// WriteTimeout is generous because backup download streams large files.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown goroutine
	go func() {
		<-ctx.Done()
		log.Println("Shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
		srv.Shutdown()
	}()

	log.Printf("Server starting on %s", cfg.ListenAddr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Println("Server stopped")
}
