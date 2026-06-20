package category

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
	"github.com/leyl1ne/ProductService/internal/service"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) ListCategories(ctx context.Context) ([]CategoryOutput, error) {
	const op = "service.category.ListCategories"

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	categories, err := s.repo.ListCategories(ctxTimeout)
	if err != nil {
		return nil, fmt.Errorf("%s: list categories: %w", op, err)
	}

	return toCategoryListOutput(categories), nil
}

func (s *Service) CreateCategory(ctx context.Context, input CreateCategoryInput) (*CategoryOutput, error) {
	const op = "service.category.CreateCategory"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	if !auth.CanManageCategory(user) {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	category := categorymodel.Category{
		ID:   uuid.New(),
		Name: input.Name,
	}

	created, err := s.repo.CreateCategory(ctxTimeout, category)
	if err != nil {
		if errors.Is(err, categorymodel.ErrCategoryAlreadyExists) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrCategoryAlreadyExists)
		}
		return nil, fmt.Errorf("%s: create category: %w", op, err)
	}

	return toCategoryOutput(created), nil
}

func (s *Service) UpdateCategory(ctx context.Context, id uuid.UUID, input UpdateCategoryInput) (*CategoryOutput, error) {
	const op = "service.category.UpdateCategory"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	if !auth.CanManageCategory(user) {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	updated, err := s.repo.UpdateCategory(ctxTimeout, id, input.Name)
	if err != nil {
		if errors.Is(err, categorymodel.ErrCategoryNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrCategoryNotFound)
		}
		if errors.Is(err, categorymodel.ErrCategoryAlreadyExists) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrCategoryAlreadyExists)
		}
		return nil, fmt.Errorf("%s: update category: %w", op, err)
	}

	return toCategoryOutput(updated), nil
}

func (s *Service) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	const op = "service.category.DeleteCategory"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	if !auth.CanManageCategory(user) {
		return fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.repo.DeleteCategory(ctxTimeout, id); err != nil {
		if errors.Is(err, categorymodel.ErrCategoryNotFound) {
			return fmt.Errorf("%s: %w", op, service.ErrCategoryNotFound)
		}
		if errors.Is(err, categorymodel.ErrCategoryInUse) {
			return fmt.Errorf("%s: %w", op, service.ErrCategoryInUse)
		}
		return fmt.Errorf("%s: delete category: %w", op, err)
	}

	return nil
}
