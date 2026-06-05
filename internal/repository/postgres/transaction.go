package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leyl1ne/ProductService/pkg/transaction"
)

type RepositoryTransaction interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repository) querier(ctx context.Context) RepositoryTransaction {
	if tx := transaction.Extract(ctx); tx != nil {
		return tx.Tx
	}
	return r.pool
}

func (r *Repository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pool.Begin: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	ctx = transaction.SetTransaction(ctx, tx)

	if err = fn(ctx); err != nil {
		return fmt.Errorf("fn: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("tx.Commit: %w", err)
	}

	return nil
}
