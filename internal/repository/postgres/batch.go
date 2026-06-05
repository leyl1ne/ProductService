package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	batchmodel "github.com/leyl1ne/ProductService/internal/model/batch"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
)

func (r *Repository) CreateBatch(ctx context.Context, batch batchmodel.ProductBatch) (*batchmodel.ProductBatch, error) {
	const op = "repository.postgres.CreateBatch"

	const query = `
		INSERT INTO product_batches (id, product_id, warehouse_company_id, quantity_total, quantity_reserved, production_date, expiration_date, batch_number, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, product_id, warehouse_company_id, quantity_total, quantity_reserved, production_date, expiration_date, batch_number, created_at
	`

	var b batchmodel.ProductBatch

	err := r.querier(ctx).QueryRow(ctx, query,
		batch.ID,
		batch.ProductID,
		batch.WarehouseCompanyID,
		batch.QuantityTotal,
		batch.QuantityReserved,
		batch.ProductionDate,
		batch.ExpirationDate,
		batch.BatchNumber,
		batch.CreatedAt,
	).Scan(
		&b.ID,
		&b.ProductID,
		&b.WarehouseCompanyID,
		&b.QuantityTotal,
		&b.QuantityReserved,
		&b.ProductionDate,
		&b.ExpirationDate,
		&b.BatchNumber,
		&b.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, fmt.Errorf("%s: %w", op, productmodel.ErrProductNotFound)
		}
		return nil, fmt.Errorf("%s: query row: %w", op, err)
	}

	return &b, nil
}

func (r *Repository) GetBatchByID(ctx context.Context, id uuid.UUID) (*batchmodel.ProductBatch, error) {
	const op = "repository.postgres.GetBatchByID"

	const query = `
		SELECT id, product_id, warehouse_company_id, quantity_total, quantity_reserved, production_date, expiration_date, batch_number, created_at
		FROM product_batches
		WHERE id = $1
	`

	var b batchmodel.ProductBatch

	err := r.querier(ctx).QueryRow(ctx, query, id).Scan(
		&b.ID,
		&b.ProductID,
		&b.WarehouseCompanyID,
		&b.QuantityTotal,
		&b.QuantityReserved,
		&b.ProductionDate,
		&b.ExpirationDate,
		&b.BatchNumber,
		&b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, batchmodel.ErrBatchNotFound)
		}
		return nil, fmt.Errorf("%s: query row: %w", op, err)
	}

	return &b, nil
}

func (r *Repository) ListBatchesByProduct(ctx context.Context, productID uuid.UUID) ([]batchmodel.ProductBatch, error) {
	const op = "repository.postgres.ListBatchesByProduct"

	const query = `
		SELECT id, product_id, warehouse_company_id, quantity_total, quantity_reserved, production_date, expiration_date, batch_number, created_at
		FROM product_batches
		WHERE product_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.querier(ctx).Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	batches := make([]batchmodel.ProductBatch, 0)
	for rows.Next() {
		var b batchmodel.ProductBatch
		if err := rows.Scan(
			&b.ID,
			&b.ProductID,
			&b.WarehouseCompanyID,
			&b.QuantityTotal,
			&b.QuantityReserved,
			&b.ProductionDate,
			&b.ExpirationDate,
			&b.BatchNumber,
			&b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		batches = append(batches, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return batches, nil
}

type UpdateBatchParams struct {
	ProductionDate *time.Time
	ExpirationDate *time.Time
	BatchNumber    *string
}

func (r *Repository) UpdateBatch(ctx context.Context, id uuid.UUID, params UpdateBatchParams) (*batchmodel.ProductBatch, error) {
	const op = "repository.postgres.UpdateBatch"

	var setClauses []string
	var args []any
	argIdx := 1

	if params.ProductionDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("production_date = $%d", argIdx))
		args = append(args, *params.ProductionDate)
		argIdx++
	}

	if params.ExpirationDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("expiration_date = $%d", argIdx))
		args = append(args, *params.ExpirationDate)
		argIdx++
	}

	if params.BatchNumber != nil {
		setClauses = append(setClauses, fmt.Sprintf("batch_number = $%d", argIdx))
		args = append(args, *params.BatchNumber)
		argIdx++
	}

	if len(setClauses) == 0 {
		return r.GetBatchByID(ctx, id)
	}

	// WHERE id = $N
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE product_batches
		SET %s
		WHERE id = $%d
		RETURNING id, product_id, warehouse_company_id, quantity_total, quantity_reserved, production_date, expiration_date, batch_number, created_at
	`, strings.Join(setClauses, ", "), argIdx)

	var b batchmodel.ProductBatch

	err := r.querier(ctx).QueryRow(ctx, query, args...).Scan(
		&b.ID,
		&b.ProductID,
		&b.WarehouseCompanyID,
		&b.QuantityTotal,
		&b.QuantityReserved,
		&b.ProductionDate,
		&b.ExpirationDate,
		&b.BatchNumber,
		&b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, batchmodel.ErrBatchNotFound)
		}
		return nil, fmt.Errorf("%s: query row: %w", op, err)
	}

	return &b, nil
}

func (r *Repository) DeleteBatch(ctx context.Context, id uuid.UUID) error {
	const op = "repository.postgres.DeleteBatch"

	const query = `
		DELETE FROM product_batches
		WHERE id = $1
	`

	tag, err := r.querier(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("%s: exec: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, batchmodel.ErrBatchNotFound)
	}

	return nil
}
