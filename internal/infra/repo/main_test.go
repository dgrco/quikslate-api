package repo

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/dgrco/quikslate/cmd/migrate/migrations"
	"github.com/dgrco/quikslate/internal/database"
	"github.com/dgrco/quikslate/internal/domain"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var pgRepo *PgRepository

const (
	userPasswordHash = "password-hash"
)

func setupContainer(ctx context.Context) (*postgres.PostgresContainer, error) {
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return postgresContainer, fmt.Errorf("failed to start container: %w", err)
	}

	return postgresContainer, nil
}

func runMigrations(databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.EmbedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set SQL dialect: %w", err)
	}

	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("failed to migrate up: %w", err)
	}

	log.Println("migrate up succeeded")
	return nil
}

func runTests(ctx context.Context, m *testing.M) error {
	container, err := setupContainer(ctx)
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Printf("failed to terminate container: %v", err)
		}
	}()
	if err != nil {
		return err
	}

	databaseUrl, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("failed to get container connection string: %w", err)
	}

	if err := runMigrations(databaseUrl); err != nil {
		return err
	}

	pool, err := database.Connect(databaseUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to the test database: %w", err)
	}
	defer pool.Close()

	pgRepo = NewPgRepository(pool)

	m.Run()

	return nil
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	if err := runTests(ctx, m); err != nil {
		log.Printf("failed to run tests: %v", err)
		os.Exit(1)
	}
}

func addUser(t *testing.T, repo domain.Repo, name string) domain.User {
	t.Helper()
	email := fmt.Sprintf("%s@example.test", strings.ReplaceAll(name, " ", "_"))
	user, err := repo.CreateUser(t.Context(), email, name, userPasswordHash)
	if err != nil {
		t.Fatalf("addUser: %v", err)
	}
	return user
}

func addBusiness(t *testing.T, repo domain.Repo) domain.Business {
	t.Helper()
	business, err := repo.CreateBusiness(t.Context(), "Acme Corp.")
	if err != nil {
		t.Fatalf("addBusiness: %v", err)
	}
	return business
}

func addLocation(t *testing.T, repo domain.Repo, businessId string) domain.Location {
	t.Helper()
	location, err := repo.CreateLocation(t.Context(), businessId, "XYZ St.", nil, "America/Vancouver")
	if err != nil {
		t.Fatalf("addLocation: %v", err)
	}
	return location
}

func startRollbackTransaction(t *testing.T) domain.Repo {
	t.Helper()
	tx, err := pgRepo.BeginTransaction(t.Context())
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// context.Background() here since t.Context() cancels prior to cleanup
	t.Cleanup(func() { tx.Rollback(context.Background()) })

	return pgRepo.WithTx(tx)
}
