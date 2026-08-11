package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strings"
)

type Config struct {
	ApiPort     string
	DatabaseUrl string
	JWTSecret   string
	SecureMode  string
	CorsOrigins string
}

// Load collects a set of environment variables based on Config; panics if required variables are not set
func Load() *Config {
	_ = godotenv.Load(".env.local", ".env.api")

	return &Config{
		ApiPort:     getEnv("API_PORT", "8080"),
		DatabaseUrl: mustGetEnv("DATABASE_URL"),
		JWTSecret:   mustGetEnv("JWT_SECRET"),
		SecureMode:  getEnv("SECURE_MODE", "true"),
		CorsOrigins: getEnv("CORS_ORIGINS", "http://localhost:5173"),
	}
}

// CorsOriginList splits the comma-separated CorsOrigins value into a slice, trimming whitespace.
func (c *Config) CorsOriginList() []string {
	origins := strings.Split(c.CorsOrigins, ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}
	return origins
}

// IsSecureMode is a safe way to extract a boolean from the SecureMode string value.
func (c *Config) IsSecureMode() bool {
	secureModeStr := strings.ToLower(c.SecureMode)
	if secureModeStr == "false" {
		return false
	}
	if secureModeStr == "true" {
		return true
	}

	// Warn about malformed value
	log.Println(
		"WARNING: environment variable SECURE_MODE is not set to either 'true' or 'false', defaulting to true...",
	)
	return true
}

// getEnv returns an environment variable's value from its key with a fallback
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// mustGetEnv returns an environment variable's value from its key and panics if the key is not set
func mustGetEnv(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	panic(fmt.Sprintf("Environment variable with key '%s' not set and is required.", key))
}
