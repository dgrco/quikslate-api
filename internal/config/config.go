package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strings"
)

type Config struct {
	ApiPort         string
	DatabaseUrl     string
	JWTSecret       string
	SecureMode      string
	CorsOrigins     string
	Mailer          string
	FrontendBaseURL string
}

func Load() *Config {
	// Host runs only. .env is excluded from images, so in a container this
	// finds nothing and the values Compose set are already in the environment.
	_ = godotenv.Load(".env")

	return &Config{
		ApiPort:         getEnv("API_PORT", "8080"),
		DatabaseUrl:     mustGetEnv("DATABASE_URL"),
		JWTSecret:       mustGetEnv("JWT_SECRET"),
		SecureMode:      getEnv("SECURE_MODE", "true"),
		CorsOrigins:     getEnv("CORS_ORIGINS", "http://localhost:5173"),
		Mailer:          mustGetEnv("MAILER"),
		FrontendBaseURL: mustGetEnv("FRONTEND_BASE_URL"),
	}
}

func (c *Config) CorsOriginList() []string {
	origins := strings.Split(c.CorsOrigins, ",")
	for i, o := range origins {
		origins[i] = strings.TrimSpace(o)
	}
	return origins
}

// IsSecureMode fails safe: anything other than an explicit "false" counts
// as production.
func (c *Config) IsSecureMode() bool {
	secureModeStr := strings.ToLower(c.SecureMode)
	if secureModeStr == "false" {
		return false
	}
	if secureModeStr == "true" {
		return true
	}

	log.Println(
		"WARNING: environment variable SECURE_MODE is not set to either 'true' or 'false', defaulting to true...",
	)
	return true
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustGetEnv(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	panic(fmt.Sprintf("Environment variable with key '%s' not set and is required.", key))
}
