package repo

import (
	"fmt"
	"strings"
	"time"
)

// builder.go provides updateBuilder, a small helper for constructing partial
// "UPDATE ... SET" queries where only some columns are being written. It's
// used by the repo methods that implement partial updates (e.g.
// UpdateShiftById, UpdateLocationById).

type updateBuilder struct {
	args       []any
	setClauses []string
	argIdx     int
}

// newUpdateBuilder initializes a updateBuilder.
// NOTE: this builder assumes the table has a 'updated_at' TIMESTAMPTZ column
func newUpdateBuilder() *updateBuilder {
	return &updateBuilder{argIdx: 1}
}

// Add records column and value as a "set clause" to be included the next
// time Build is called.
func (b *updateBuilder) Add(column string, value any) {
	b.args = append(b.args, value)
	b.setClauses = append(b.setClauses, fmt.Sprintf("%s = $%d", column, b.argIdx))
	b.argIdx++
}

// Build builds the final query and returns the args slice.
// NOTE: this adds the updated_at column automatically!
func (b *updateBuilder) Build(table, idColumn string, id any) (string, []any) {
	b.Add("updated_at", time.Now())
	b.args = append(b.args, id)
	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d",
		table,
		strings.Join(b.setClauses, ", "),
		idColumn,
		b.argIdx,
	)
	return query, b.args
}

// IsEmpty reports whether Add has been called yet, letting callers skip
// issuing an UPDATE with no columns to set.
func (b *updateBuilder) IsEmpty() bool {
	return len(b.setClauses) == 0
}
