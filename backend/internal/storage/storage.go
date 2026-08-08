// Package storage holds the PostgreSQL implementations of the repository
// interfaces declared by the domain packages.
package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolationCode is the SQLSTATE PostgreSQL raises for a unique index
// conflict.
const uniqueViolationCode = "23505"

// Querier is the subset of pgx used by the repositories. Both *pgxpool.Pool and
// pgx.Tx satisfy it, so a repository can later be used inside a transaction
// without changing its code.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// isUniqueViolation reports whether the error is a unique index conflict on the
// named constraint or index. An empty name matches any of them.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolationCode {
		return false
	}

	return constraint == "" || pgErr.ConstraintName == constraint
}
