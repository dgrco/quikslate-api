package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/dgrco/quikslate/cmd/migrate/migrations"
	"github.com/dgrco/quikslate/internal/database"
	"github.com/dgrco/quikslate/internal/domain"
	_ "github.com/jackc/pgx/v5/stdlib"
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

func TestGetBusinessMemberAuthzContext(t *testing.T) {
	cases := []struct {
		name           string
		isPrimaryAdmin bool
		isAdmin        bool
	}{
		{"admin but not primary admin", false, true},
		{"primary admin", true, true},
		{"standard member", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			txRepo := startRollbackTransaction(t)
			b := addBusiness(t, txRepo)
			u := addUser(t, txRepo, "John Doe")
			if err := txRepo.AddUserToBusiness(t.Context(), u.Id, b.Id, tc.isPrimaryAdmin, tc.isAdmin); err != nil {
				t.Fatalf("failed to add user to business: %v", err)
			}
			authzResult, err := txRepo.GetBusinessMemberAuthzContext(t.Context(), u.Id, b.Id)
			if err != nil {
				t.Fatalf("failed to get business member authz result: %v", err)
			}
			if authzResult.IsPrimaryAdmin != tc.isPrimaryAdmin || authzResult.IsAdmin != tc.isAdmin {
				t.Errorf(
					"expected is_primary_admin=%v and is_admin=%v, got is_primary_admin=%v and is_admin=%v",
					tc.isPrimaryAdmin,
					tc.isAdmin,
					authzResult.IsPrimaryAdmin,
					authzResult.IsAdmin,
				)
			}
		})
	}
}

func TestGetLocationMemberAuthzContext(t *testing.T) {
	cases := []struct {
		name           string
		isPrimaryAdmin bool
		isAdmin        bool
		assignedRole   domain.LRole
	}{
		{"admin but not primary admin", false, true, domain.EmptyRole},
		{"primary admin", true, true, domain.EmptyRole},
		{"standard member", false, false, domain.Employee},
		{"non-admin with empty role", false, false, domain.EmptyRole},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			txRepo := startRollbackTransaction(t)
			b := addBusiness(t, txRepo)
			l := addLocation(t, txRepo, b.Id)
			u := addUser(t, txRepo, "John Doe")
			if err := txRepo.AddUserToBusiness(t.Context(), u.Id, b.Id, tc.isPrimaryAdmin, tc.isAdmin); err != nil {
				t.Fatalf("failed to add user to business: %v", err)
			}

			if tc.assignedRole != domain.EmptyRole {
				if err := txRepo.AssignRole(t.Context(), u.Id, b.Id, l.Id, tc.assignedRole); err != nil {
					t.Fatalf("failed to add location role: %v", err)
				}
			}

			authzResult, err := txRepo.GetLocationMemberAuthzContext(t.Context(), u.Id, b.Id, l.Id)
			if err != nil {
				t.Fatalf("failed to get location member authz context: %v", err)
			}
			if authzResult.IsPrimaryAdmin != tc.isPrimaryAdmin || authzResult.IsAdmin != tc.isAdmin {
				t.Errorf(
					"expected is_primary_admin=%v and is_admin=%v, got is_primary_admin=%v and is_admin=%v",
					tc.isPrimaryAdmin,
					tc.isAdmin,
					authzResult.IsPrimaryAdmin,
					authzResult.IsAdmin,
				)
			}
			if authzResult.Role != tc.assignedRole {
				t.Errorf("expected role=%q, got %q", tc.assignedRole, authzResult.Role)
			}
		})
	}
}

func TestGetLocationMemberAuthzContextFailsOnLocationFromAnotherBusiness(t *testing.T) {
	txRepo := startRollbackTransaction(t)
	b1 := addBusiness(t, txRepo)
	b2 := addBusiness(t, txRepo)
	l2 := addLocation(t, txRepo, b2.Id)

	u := addUser(t, txRepo, "John Doe")
	if err := txRepo.AddUserToBusiness(t.Context(), u.Id, b1.Id, false, false); err != nil {
		t.Fatalf("failed to add user to business: %v", err)
	}

	_, err := txRepo.GetLocationMemberAuthzContext(t.Context(), u.Id, b1.Id, l2.Id)
	expectedErr := domain.ErrForbidden
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error (%v), got (%v)", expectedErr, err)
	}
}
