package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
)

func (r *Repository) CreateCategory(ctx context.Context, category categorymodel.Category) (*categorymodel.Category, error) {
	const op = "repository.postgres.CreateCategory"

	const query = `
		INSERT INTO categories (id, name)
		VALUES ($1, $2)
		RETURNING id, name
	`

	var c categorymodel.Category

	err := r.querier(ctx).QueryRow(ctx, query, category.ID, category.Name).Scan(&c.ID, &c.Name)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%s: %w", op, categorymodel.ErrCategoryAlreadyExists)
		}
		return nil, fmt.Errorf("%s: query row: %w", op, err)
	}

	return &c, nil
}

func (r *Repository) GetCategoryByID(ctx context.Context, id uuid.UUID) (*categorymodel.Category, error) {
	const op = "repository.postgres.GetCategoryByID"

	const query = `
		SELECT id, name
		FROM categories
		WHERE id = $1
	`

	var c categorymodel.Category

	err := r.querier(ctx).QueryRow(ctx, query, id).Scan(&c.ID, &c.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, categorymodel.ErrCategoryNotFound)
		}
		return nil, fmt.Errorf("%s: queryRow: %w", op, err)
	}

	return &c, nil
}

func (r *Repository) ListCategories(ctx context.Context) ([]categorymodel.Category, error) {
	const op = "repository.postgres.ListCategories"

	const query = `
		SELECT id, name
		FROM categories
		ORDER BY name
	`

	rows, err := r.querier(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	categories := make([]categorymodel.Category, 0)
	for rows.Next() {
		var c categorymodel.Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("%s: scan: %w", op, err)
		}
		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return categories, nil
}

func (r *Repository) UpdateCategory(ctx context.Context, id uuid.UUID, name string) (*categorymodel.Category, error) {
	const op = "repository.postgres.UpdateCategory"

	const query = `
		UPDATE categories
		SET name = $2
		WHERE id = $1
		RETURNING id, name
	`

	var c categorymodel.Category

	err := r.querier(ctx).QueryRow(ctx, query, id, name).Scan(&c.ID, &c.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, categorymodel.ErrCategoryNotFound)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%s: %w", op, categorymodel.ErrCategoryAlreadyExists)
		}
		return nil, fmt.Errorf("%s: query row: %w", op, err)
	}

	return &c, nil
}

func (r *Repository) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	const op = "repository.postgres.DeleteCategory"

	const query = `
		DELETE FROM categories
		WHERE id = $1
	`

	tag, err := r.querier(ctx).Exec(ctx, query, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("%s: %w", op, categorymodel.ErrCategoryInUse)
		}
		return fmt.Errorf("%s: exec: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, categorymodel.ErrCategoryNotFound)
	}

	return nil
}
