package batch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/leyl1ne/ProductService/internal/model/auth"
	batchmodel "github.com/leyl1ne/ProductService/internal/model/batch"
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

func (s *Service) checkProductOwnership(ctx context.Context, user auth.User, productID uuid.UUID) error {
	product, err := s.repo.GetProductByID(ctx, productID)
	if err != nil {
		if errors.Is(err, productmodel.ErrProductNotFound) {
			return service.ErrProductNotFound
		}
		return fmt.Errorf("get product by id: %w", err)
	}

	if !auth.CanManageCompanyResource(user, product.CompanyID) {
		return service.ErrForbidden
	}

	return nil
}

func (s *Service) CreateBatch(ctx context.Context, productID uuid.UUID, input CreateBatchInput) (*BatchOutput, error) {
	const op = "service.batch.CreateBatch"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	if !auth.CanCreateBatch(user) {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.checkProductOwnership(ctxTimeout, user, productID); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	batch := batchmodel.ProductBatch{
		ID:                 uuid.New(),
		ProductID:          productID,
		WarehouseCompanyID: input.WarehouseCompanyID,
		QuantityTotal:      input.QuantityTotal,
		QuantityReserved:   0,
		ProductionDate:     input.ProductionDate,
		ExpirationDate:     input.ExpirationDate,
		BatchNumber:        input.BatchNumber,
		CreatedAt:          time.Now(),
	}

	created, err := s.repo.CreateBatch(ctxTimeout, batch)
	if err != nil {
		if errors.Is(err, productmodel.ErrProductNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrProductNotFound)
		}
		return nil, fmt.Errorf("%s: create batch: %w", op, err)
	}

	return toBatchOutput(created), nil
}

func (s *Service) GetBatchByID(ctx context.Context, id uuid.UUID) (*BatchOutput, error) {
	const op = "service.batch.GetBatchByID"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	batch, err := s.repo.GetBatchByID(ctxTimeout, id)
	if err != nil {
		if errors.Is(err, batchmodel.ErrBatchNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrBatchNotFound)
		}
		return nil, fmt.Errorf("%s: get batch by id: %w", op, err)
	}

	// Ownership check via product's company
	if err := s.checkProductOwnership(ctxTimeout, user, batch.ProductID); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return toBatchOutput(batch), nil
}

func (s *Service) ListBatchesByProduct(ctx context.Context, productID uuid.UUID) ([]BatchOutput, error) {
	const op = "service.batch.ListBatchesByProduct"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.checkProductOwnership(ctxTimeout, user, productID); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	batches, err := s.repo.ListBatchesByProduct(ctxTimeout, productID)
	if err != nil {
		return nil, fmt.Errorf("%s: list batches by product: %w", op, err)
	}

	return toBatchListOutput(batches), nil
}

// TODO: add batch status for success update
func (s *Service) UpdateBatch(ctx context.Context, id uuid.UUID, input UpdateBatchInput) (*BatchOutput, error) {
	const op = "service.batch.UpdateBatch"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	batch, err := s.repo.GetBatchByID(ctxTimeout, id)
	if err != nil {
		if errors.Is(err, batchmodel.ErrBatchNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrBatchNotFound)
		}
		return nil, fmt.Errorf("%s: get batch by id: %w", op, err)
	}

	if err := s.checkProductOwnership(ctxTimeout, user, batch.ProductID); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	params := UpdateBatchParams{
		ProductionDate: input.ProductionDate,
		ExpirationDate: input.ExpirationDate,
		BatchNumber:    input.BatchNumber,
	}

	updated, err := s.repo.UpdateBatch(ctxTimeout, id, params)
	if err != nil {
		if errors.Is(err, batchmodel.ErrBatchNotFound) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrBatchNotFound)
		}
		return nil, fmt.Errorf("%s: update batch: %w", op, err)
	}

	return toBatchOutput(updated), nil
}

func (s *Service) DeleteBatch(ctx context.Context, id uuid.UUID) error {
	const op = "service.batch.DeleteBatch"

	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return fmt.Errorf("%s: %w", op, service.ErrForbidden)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	batch, err := s.repo.GetBatchByID(ctxTimeout, id)
	if err != nil {
		if errors.Is(err, batchmodel.ErrBatchNotFound) {
			return fmt.Errorf("%s: %w", op, service.ErrBatchNotFound)
		}
		return fmt.Errorf("%s: get batch by id: %w", op, err)
	}

	if err := s.checkProductOwnership(ctxTimeout, user, batch.ProductID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.repo.DeleteBatch(ctxTimeout, id); err != nil {
		if errors.Is(err, batchmodel.ErrBatchNotFound) {
			return fmt.Errorf("%s: %w", op, service.ErrBatchNotFound)
		}
		return fmt.Errorf("%s: delete batch: %w", op, err)
	}

	return nil
}
