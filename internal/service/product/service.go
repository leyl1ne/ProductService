package product

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
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

func (s *Service) CreateProduct(ctx context.Context, input CreateProductInput) (*ProductOutput, error) {
	const op = "service.product.CreateProduct"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	if !auth.CanCreateProduct(user) {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	product := productmodel.Product{
		ID:          uuid.New(),
		CompanyID:   input.CompanyID,
		CategoryID:  input.CategoryID,
		Name:        input.Name,
		Description: input.Desc,
		Price:       input.Price,
		Unit:        input.Unit,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	created, err := s.repo.CreateProduct(ctxTimeout, product)
	if err != nil {
		if errors.Is(err, categorymodel.ErrCategoryNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrCategoryNotFound)
		}
		return nil, fmt.Errorf("%s: create product: %w", op, err)
	}

	var category *CategoryShortOutput
	if created.CategoryID != nil {
		cat, err := s.repo.GetCategoryByID(ctxTimeout, *created.CategoryID)
		if err == nil {
			category = toCategoryShortOutput(cat)
		}
	}

	return toProductOutput(created, category), nil
}

func (s *Service) GetProductByID(ctx context.Context, id uuid.UUID) (*ProductOutput, error) {
	const op = "service.product.GetProductByID"

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	p, err := s.repo.GetProductByID(ctxTimeout, id)
	if err != nil {
		if errors.Is(err, productmodel.ErrProductNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrProductNotFound)
		}
		return nil, fmt.Errorf("%s: get product by id: %w", op, err)
	}

	var category *CategoryShortOutput
	if p.CategoryID != nil {
		cat, err := s.repo.GetCategoryByID(ctxTimeout, *p.CategoryID)
		if err == nil {
			category = toCategoryShortOutput(cat)
		}
	}

	return toProductOutput(p, category), nil
}

func (s *Service) ListProducts(ctx context.Context, filter ProductFilter, page, limit int) (*ProductListOutput, error) {
	const op = "service.product.ListProducts"

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := s.repo.ListProducts(ctxTimeout, filter, page, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: list products: %w", op, err)
	}

	categories, err := s.repo.ListCategories(ctxTimeout)
	if err != nil {
		return nil, fmt.Errorf("%s: list categories: %w", op, err)
	}
	categoryMap := make(map[uuid.UUID]categorymodel.Category, len(categories))
	for _, c := range categories {
		categoryMap[c.ID] = c
	}

	outputs := make([]ProductOutput, 0, len(result.Products))
	for _, p := range result.Products {
		var category *CategoryShortOutput
		if p.CategoryID != nil {
			if cat, ok := categoryMap[*p.CategoryID]; ok {
				category = toCategoryShortOutput(&cat)
			}
		}
		outputs = append(outputs, *toProductOutput(&p, category))
	}

	return &ProductListOutput{
		Products: outputs,
		Total:    result.Total,
	}, nil
}

func (s *Service) UpdateProduct(ctx context.Context, id uuid.UUID, input UpdateProductInput) (*ProductOutput, error) {
	const op = "service.product.UpdateProduct"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	existing, err := s.repo.GetProductByID(ctxTimeout, id)
	if err != nil {
		if errors.Is(err, productmodel.ErrProductNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrProductNotFound)
		}
		return nil, fmt.Errorf("%s: get product by id: %w", op, err)
	}

	if !auth.CanManageCompanyResource(user, existing.CompanyID) {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	params := UpdateProductParams{
		Name:        input.Name,
		Description: input.Description,
		Price:       input.Price,
		Unit:        input.Unit,
		CategoryID:  input.CategoryID,
		IsActive:    input.IsActive,
	}

	updated, err := s.repo.UpdateProduct(ctxTimeout, id, params)
	if err != nil {
		if errors.Is(err, productmodel.ErrProductNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrProductNotFound)
		}
		if errors.Is(err, categorymodel.ErrCategoryNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrCategoryNotFound)
		}
		return nil, fmt.Errorf("%s: update product: %w", op, err)
	}

	var category *CategoryShortOutput
	if updated.CategoryID != nil {
		cat, err := s.repo.GetCategoryByID(ctxTimeout, *updated.CategoryID)
		if err == nil {
			category = toCategoryShortOutput(cat)
		}
	}

	return toProductOutput(updated, category), nil
}

func (s *Service) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	const op = "service.product.DeleteProduct"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	existing, err := s.repo.GetProductByID(ctxTimeout, id)
	if err != nil {
		if errors.Is(err, productmodel.ErrProductNotFound) {
			return fmt.Errorf("%s: %w", op, service.ErrProductNotFound)
		}
		return fmt.Errorf("%s: get product by id: %w", op, err)
	}

	// Ownership check via policy
	if !auth.CanManageCompanyResource(user, existing.CompanyID) {
		return fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	if err := s.repo.DeleteProduct(ctxTimeout, id); err != nil {
		if errors.Is(err, productmodel.ErrProductNotFound) {
			return fmt.Errorf("%s: %w", op, service.ErrProductNotFound)
		}
		return fmt.Errorf("%s: delete product: %w", op, err)
	}

	return nil
}

func (s *Service) GetProductAvailability(ctx context.Context, productID uuid.UUID) (*ProductAvailabilityOutput, error) {
	const op = "service.product.GetProductAvailability"

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.repo.GetProductByID(ctxTimeout, productID)
	if err != nil {
		if errors.Is(err, productmodel.ErrProductNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrProductNotFound)
		}
		return nil, fmt.Errorf("%s: get product by id: %w", op, err)
	}

	batches, err := s.repo.GetProductAvailability(ctxTimeout, productID)
	if err != nil {
		return nil, fmt.Errorf("%s: get product availability: %w", op, err)
	}

	result := &ProductAvailabilityOutput{
		ProductID: productID,
		Batches:   make([]AvailableBatchOutput, 0, len(batches)),
	}

	for _, b := range batches {
		result.Batches = append(result.Batches, AvailableBatchOutput{
			BatchID:           b.BatchID,
			AvailableQuantity: b.AvailableQuantity,
			ExpirationDate:    b.ExpirationDate,
		})
	}

	return result, nil
}
