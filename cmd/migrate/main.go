package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func main() {
	_ = godotenv.Load(".env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to open database connection: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("failed to set SQL dialect: %v", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		log.Fatalf("failed to migrate up: %v", err)
	}

	fmt.Println("Migrations are up to date.")
}
