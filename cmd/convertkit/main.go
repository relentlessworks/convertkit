package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/relentlessworks/convertkit/internal/api"
	"github.com/relentlessworks/convertkit/internal/auth"
	"github.com/relentlessworks/convertkit/internal/config"
	"github.com/relentlessworks/convertkit/internal/store"
)

func main() {
	cfg := config.FromFlags()

	// Initialize store
	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize auth
	authSvc := auth.New(st)

	// Initialize API server
	server := api.NewServer(st, authSvc)
	handler := server.Routes()

	// Start HTTP server
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Fprintln(os.Stderr, "convertkit: shutting down...")
		httpServer.Close()
	}()

	fmt.Fprintf(os.Stderr, "convertkit: listening on %s\n", cfg.Addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
