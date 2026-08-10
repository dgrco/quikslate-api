package repo

// errors.go holds Postgres error codes referenced by other repo files to
// recognize specific constraint violations (e.g. unique-key conflicts)
// without importing pgconn everywhere.
const (
	ErrPgUniqueConstraintViolation = "23505"
)
