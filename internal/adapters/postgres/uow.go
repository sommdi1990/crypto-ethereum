package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type txKey struct{}

// UnitOfWork runs callbacks inside a database transaction.
type UnitOfWork struct {
	db *DB
}

// NewUnitOfWork creates a transaction manager.
func NewUnitOfWork(db *DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

// WithinTx executes fn inside a transaction.
func (u *UnitOfWork) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := u.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func querier(ctx context.Context, db *DB) pgx.Tx {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return nil
}

func exec(ctx context.Context, db *DB) interface {
	Exec(context.Context, string, ...any) (pgx.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
} {
	if tx := querier(ctx, db); tx != nil {
		return tx
	}
	return db.Pool
}
