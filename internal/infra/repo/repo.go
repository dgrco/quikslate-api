package repo

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// queryExecutor is the method set pgxpool.Pool and pgx.Tx share, which is
// what lets every repo method run unchanged inside or outside a transaction.
type queryExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PgRepository struct {
	pool *pgxpool.Pool
	exec queryExecutor
}

func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool, pool}
}

func (r *PgRepository) beginPgxTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

func (r *PgRepository) BeginTransaction(ctx context.Context) (domain.Tx, error) {
	return r.beginPgxTx(ctx)
}

// WithTx panics on a tx that didn't come from BeginTransaction: that can
// only be a programming error, not a runtime condition to recover from.
func (r *PgRepository) WithTx(tx domain.Tx) domain.Repo {
	pgxTx, ok := tx.(pgx.Tx)
	if !ok {
		panic(fmt.Sprintf("repo.WithTx: domain.Tx of type %T did not originate from PgRepository.BeginTransaction", tx))
	}
	return &PgRepository{r.pool, pgxTx}
}
