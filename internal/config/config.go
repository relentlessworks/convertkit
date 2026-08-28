package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
)

// Config holds all configuration for the service.
type Config struct {
	Addr   string
	DBPath string
	Secret string
}

// Default returns a config with sensible defaults.
func Default() *Config {
	return &Config{
		Addr:   ":7700",
		DBPath: "convertkit.json",
		Secret: "",
	}
}

// FromFlags parses CLI flags and env vars, layered over defaults.
func FromFlags() *Config {
	cfg := Default()

	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "listen address")
	flag.StringVar(&cfg.DBPath, "db", cfg.DBPath, "database file path")
	flag.StringVar(&cfg.Secret, "secret", cfg.Secret, "token signing secret (auto-generated if empty)")
	flag.Parse()

	// Env vars override flags
	if v := os.Getenv("CONVERTKIT_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("CONVERTKIT_DB"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("CONVERTKIT_SECRET"); v != "" {
		cfg.Secret = v
	}

	// Auto-generate secret if not provided
	if cfg.Secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		cfg.Secret = hex.EncodeToString(b)
		fmt.Fprintf(os.Stderr, "convertkit: generated random secret (set CONVERTKIT_SECRET to persist)\n")
	}

	return cfg
}
