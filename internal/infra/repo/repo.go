package repo

import "github.com/jackc/pgx/v5/pgxpool"

type PgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool}
}
