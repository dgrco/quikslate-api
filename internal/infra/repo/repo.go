package repo

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PgRepository struct {
	pool *pgxpool.Pool // used for starting transactions, if needed
	exec queryExecutor // used at every repo call (transaction and non-transaction)
}

func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool, pool}
}

// beginPgxTx is a wrapper function that allows BeginTransaction
// and WithTx to use domain.Tx signature types.
// This is required to avoid leaking infra details in domain.
func (r *PgRepository) beginPgxTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *PgRepository) BeginTransaction(ctx context.Context) (domain.Tx, error) {
	return r.beginPgxTx(ctx)
}

func (r *PgRepository) WithTx(tx domain.Tx) domain.Repo {
	// We can do interface-to-interface conversion!
	// This checks if the underlying value also implements pgx.Tx (which it should)
	pgxTx, ok := tx.(pgx.Tx) 
	if !ok {
		panic(fmt.Sprintf("repo.WithTx: domain.Tx of type %T did not originate from PgRepository.BeginTransaction", tx))
	}
	return &PgRepository{r.pool, pgxTx}
}
