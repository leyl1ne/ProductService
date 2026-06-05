package transaction

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type ctxKey struct{}

type Transaction struct {
	Tx pgx.Tx
}

func Extract(ctx context.Context) *Transaction {
	val := ctx.Value(ctxKey{})
	if val == nil {
		return nil
	}

	tx, ok := val.(*Transaction)
	if !ok {
		return nil
	}

	return tx
}

func SetTransaction(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, &Transaction{Tx: tx})
}
