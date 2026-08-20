package main

import (
	"database/sql"
	"embed"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func main() {
	args := os.Args

	migrationAction := "up"
	if len(args) >= 2 {
		migrationAction = strings.ToLower(args[1])
	}

	_ = godotenv.Load(".env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("failed to open database connection: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set SQL dialect: %v", err)
	}

	switch migrationAction {
	case "up":
		if err := goose.Up(db, "migrations"); err != nil {
			log.Fatalf("failed to migrate up: %v", err)
		}
		log.Println("migrate up succeeded")
	case "down":
		if err := goose.Down(db, "migrations"); err != nil {
			log.Fatalf("failed to migrate down (by 1): %v", err)
		}
		log.Println("migrate down succeeded")
	case "reset":
		if err := goose.Reset(db, "migrations"); err != nil {
			log.Fatalf("failed to reset migrations: %v", err)
		}
		log.Println("migrate reset succeeded")
	default:
		log.Fatalf("migration action %q is not valid", migrationAction)
	}
}
