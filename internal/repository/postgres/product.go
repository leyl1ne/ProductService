package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
	productservice "github.com/leyl1ne/ProductService/internal/service/product"
)

func (r *Repository) CreateProduct(ctx context.Context, product productmodel.Product) (*productmodel.Product, error) {
	const op = "repository.postgres.CreateProduct"

	const query = `
                INSERT INTO products (id, company_id, category_id, name, description, price, unit, is_active, created_at, updated_at)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
                RETURNING id, company_id, category_id, name, description, price, unit, is_active, created_at, updated_at
        `

	var p productmodel.Product

	err := r.querier(ctx).QueryRow(ctx, query,
		product.ID,
		product.CompanyID,
		product.CategoryID,
		product.Name,
		product.Description,
		product.Price,
		product.Unit,
		product.IsActive,
		product.CreatedAt,
		product.UpdatedAt,
	).Scan(
		&p.ID,
		&p.CompanyID,
		&p.CategoryID,
		&p.Name,
		&p.Description,
		&p.Price,
		&p.Unit,
		&p.IsActive,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, fmt.Errorf("%s: %w", op, categorymodel.ErrCategoryNotFound)
		}
		return nil, fmt.Errorf("%s: queryRow: %w", op, err)
	}

	return &p, nil
}

// TODO: нужно будет добавить GetProductByCompanyID
func (r *Repository) GetProductByID(ctx context.Context, id uuid.UUID) (*productmodel.Product, error) {
	const op = "repository.postgres.GetProductByID"

	const query = `
                SELECT id, company_id, category_id, name, description, price, unit, is_active, created_at, updated_at
                FROM products
                WHERE id = $1
        `

	var p productmodel.Product

	err := r.querier(ctx).QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.CompanyID,
		&p.CategoryID,
		&p.Name,
		&p.Description,
		&p.Price,
		&p.Unit,
		&p.IsActive,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, productmodel.ErrProductNotFound)
		}
		return nil, fmt.Errorf("%s: queryRow: %w", op, err)
	}

	return &p, nil
}

func (r *Repository) ListProducts(ctx context.Context, filter productservice.ProductFilter, page, limit int) (*productservice.ProductListResult, error) {
	const op = "repository.postgres.ListProducts"

	var whereClauses []string
	var args []any
	argIdx := 1

	if filter.CategoryID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("category_id = $%d", argIdx))
		args = append(args, *filter.CategoryID)
		argIdx++
	}

	if filter.CompanyID != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("company_id = $%d", argIdx))
		args = append(args, *filter.CompanyID)
		argIdx++
	}

	if filter.MinPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("price >= $%d", argIdx))
		args = append(args, *filter.MinPrice)
		argIdx++
	}

	if filter.MaxPrice != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("price <= $%d", argIdx))
		args = append(args, *filter.MaxPrice)
		argIdx++
	}

	whereStr := ""
	if len(whereClauses) > 0 {
		whereStr = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM products %s", whereStr)

	var total int
	if err := r.querier(ctx).QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("%s: count query row: %w", op, err)
	}

	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	// Data query with pagination
	offset := (page - 1) * limit
	dataQuery := fmt.Sprintf(`
                SELECT id, company_id, category_id, name, description, price, unit, is_active, created_at, updated_at
                FROM products
                %s
                ORDER BY created_at DESC
                LIMIT $%d OFFSET $%d
        `, whereStr, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.querier(ctx).Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	products := make([]productmodel.Product, 0)
	for rows.Next() {
		var p productmodel.Product
		if err := rows.Scan(
			&p.ID,
			&p.CompanyID,
			&p.CategoryID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.Unit,
			&p.IsActive,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows next: %w", op, err)
	}

	return &productservice.ProductListResult{
		Products: products,
		Total:    total,
	}, nil
}

func (r *Repository) UpdateProduct(ctx context.Context, id uuid.UUID, params productservice.UpdateProductParams) (*productmodel.Product, error) {
	const op = "repository.postgres.UpdateProduct"

	var setClauses []string
	var args []any
	argIdx := 1

	if params.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *params.Name)
		argIdx++
	}

	if params.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *params.Description)
		argIdx++
	}

	if params.Price != nil {
		setClauses = append(setClauses, fmt.Sprintf("price = $%d", argIdx))
		args = append(args, *params.Price)
		argIdx++
	}

	if params.Unit != nil {
		setClauses = append(setClauses, fmt.Sprintf("unit = $%d", argIdx))
		args = append(args, *params.Unit)
		argIdx++
	}

	if params.CategoryID != nil {
		setClauses = append(setClauses, fmt.Sprintf("category_id = $%d", argIdx))
		args = append(args, *params.CategoryID)
		argIdx++
	}

	if params.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *params.IsActive)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil, productmodel.ErrNothingToUpdate
	}

	setClauses = append(setClauses, "updated_at = now()")

	// WHERE id = $N
	args = append(args, id)

	query := fmt.Sprintf(`
                UPDATE products
                SET %s
                WHERE id = $%d
                RETURNING id, company_id, category_id, name, description, price, unit, is_active, created_at, updated_at
        `, strings.Join(setClauses, ", "), argIdx)

	var p productmodel.Product

	err := r.querier(ctx).QueryRow(ctx, query, args...).Scan(
		&p.ID,
		&p.CompanyID,
		&p.CategoryID,
		&p.Name,
		&p.Description,
		&p.Price,
		&p.Unit,
		&p.IsActive,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, productmodel.ErrProductNotFound)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, fmt.Errorf("%s: %w", op, categorymodel.ErrCategoryNotFound)
		}
		return nil, fmt.Errorf("%s: query row: %w", op, err)
	}

	return &p, nil
}

func (r *Repository) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	const op = "repository.postgres.DeleteProduct"

	const query = `
                DELETE FROM products
                WHERE id = $1
        `

	tag, err := r.querier(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("%s: exec: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, productmodel.ErrProductNotFound)
	}

	return nil
}

func (r *Repository) GetProductAvailability(ctx context.Context, productID uuid.UUID) ([]productservice.AvailableBatch, error) {
	const op = "repository.postgres.GetProductAvailability"

	const query = `
                SELECT id, (quantity_total - quantity_reserved) AS available_quantity, expiration_date
                FROM product_batches
                WHERE product_id = $1 AND (quantity_total - quantity_reserved) > 0
                ORDER BY expiration_date ASC NULLS LAST
        `

	rows, err := r.querier(ctx).Query(ctx, query, productID)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	batches := make([]productservice.AvailableBatch, 0)
	for rows.Next() {
		var b productservice.AvailableBatch
		if err := rows.Scan(&b.BatchID, &b.AvailableQuantity, &b.ExpirationDate); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		batches = append(batches, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows.Err: %w", op, err)
	}

	return batches, nil
}
