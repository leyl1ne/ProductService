package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	batchmodel "github.com/leyl1ne/ProductService/internal/model/batch"
	reservationmodel "github.com/leyl1ne/ProductService/internal/model/reservation"
)

func (r *Repository) CreateStockReservation(ctx context.Context, reservation reservationmodel.StockReservation) (*reservationmodel.StockReservation, error) {
	const op = "repository.postgres.CreateStockReservation"

	const query = `
		INSERT INTO stock_reservations (id, order_id, batch_id, quantity, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, order_id, batch_id, quantity, created_at
	`

	var sr reservationmodel.StockReservation

	err := r.querier(ctx).QueryRow(ctx, query,
		reservation.ID,
		reservation.OrderID,
		reservation.BatchID,
		reservation.Quantity,
		reservation.CreatedAt,
	).Scan(
		&sr.ID,
		&sr.OrderID,
		&sr.BatchID,
		&sr.Quantity,
		&sr.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, fmt.Errorf("%s: %w", op, batchmodel.ErrBatchNotFound)
		}
		return nil, fmt.Errorf("%s: query row: %w", op, err)
	}

	return &sr, nil
}

func (r *Repository) GetReservationsByOrderID(ctx context.Context, orderID uuid.UUID) ([]reservationmodel.StockReservation, error) {
	const op = "repository.postgres.GetReservationsByOrderID"

	const query = `
		SELECT id, order_id, batch_id, quantity, created_at
		FROM stock_reservations
		WHERE order_id = $1
		ORDER BY created_at
	`

	rows, err := r.querier(ctx).Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	reservations := make([]reservationmodel.StockReservation, 0)
	for rows.Next() {
		var sr reservationmodel.StockReservation
		if err := rows.Scan(
			&sr.ID,
			&sr.OrderID,
			&sr.BatchID,
			&sr.Quantity,
			&sr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		reservations = append(reservations, sr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows.Err: %w", op, err)
	}

	return reservations, nil
}

func (r *Repository) DeleteReservationsByOrderID(ctx context.Context, orderID uuid.UUID) error {
	const op = "repository.postgres.DeleteReservationsByOrderID"

	const query = `
		DELETE FROM stock_reservations
		WHERE order_id = $1
	`

	_, err := r.querier(ctx).Exec(ctx, query, orderID)
	if err != nil {
		return fmt.Errorf("%s: exec: %w", op, err)
	}

	return nil
}

// IncrementBatchReservedQuantity increases quantity_reserved for a batch.
// Returns ErrInsufficientStock if available quantity is not enough.
func (r *Repository) IncrementBatchReservedQuantity(ctx context.Context, batchID uuid.UUID, quantity float64) error {
	const op = "repository.postgres.IncrementBatchReservedQuantity"

	const query = `
		UPDATE product_batches
		SET quantity_reserved = quantity_reserved + $2
		WHERE id = $1 AND (quantity_total - quantity_reserved) >= $2
	`

	tag, err := r.querier(ctx).Exec(ctx, query, batchID, quantity)
	if err != nil {
		return fmt.Errorf("%s: exec: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, reservationmodel.ErrInsufficientStock)
	}

	return nil
}

// DecrementBatchReservedQuantity decreases quantity_reserved for a batch (on reservation cancellation).
func (r *Repository) DecrementBatchReservedQuantity(ctx context.Context, batchID uuid.UUID, quantity float64) error {
	const op = "repository.postgres.DecrementBatchReservedQuantity"

	const query = `
		UPDATE product_batches
		SET quantity_reserved = quantity_reserved - $2
		WHERE id = $1 AND quantity_reserved >= $2
	`

	tag, err := r.querier(ctx).Exec(ctx, query, batchID, quantity)
	if err != nil {
		return fmt.Errorf("%s: exec: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, batchmodel.ErrBatchNotFound)
	}

	return nil
}
