package batch

import (
	"context"
	"time"

	"github.com/google/uuid"
	batchmodel "github.com/leyl1ne/ProductService/internal/model/batch"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=Repository
type Repository interface {
	CreateBatch(ctx context.Context, batch batchmodel.ProductBatch) (*batchmodel.ProductBatch, error)
	GetBatchByID(ctx context.Context, id uuid.UUID) (*batchmodel.ProductBatch, error)
	ListBatchesByProduct(ctx context.Context, productID uuid.UUID) ([]batchmodel.ProductBatch, error)
	UpdateBatch(ctx context.Context, id uuid.UUID, params UpdateBatchParams) (*batchmodel.ProductBatch, error)
	DeleteBatch(ctx context.Context, id uuid.UUID) error
	GetProductByID(ctx context.Context, id uuid.UUID) (*productmodel.Product, error)
}

// --- Input types ---

type CreateBatchInput struct {
	CompanyID          uuid.UUID
	UserRole           string
	WarehouseCompanyID *uuid.UUID
	QuantityTotal      float64
	ProductionDate     *time.Time
	ExpirationDate     *time.Time
	BatchNumber        string
}

type UpdateBatchInput struct {
	ProductionDate *time.Time
	ExpirationDate *time.Time
	BatchNumber    *string
}

// --- Repository parameter types ---

type UpdateBatchParams struct {
	ProductionDate *time.Time
	ExpirationDate *time.Time
	BatchNumber    *string
}

// --- Output types ---

type BatchOutput struct {
	ID                 uuid.UUID
	ProductID          uuid.UUID
	WarehouseCompanyID *uuid.UUID
	QuantityTotal      float64
	QuantityReserved   float64
	AvailableQuantity  float64
	BatchNumber        string
	ProductionDate     *time.Time
	ExpirationDate     *time.Time
	CreatedAt          time.Time
}

// --- Converters ---

func toBatchOutput(b *batchmodel.ProductBatch) *BatchOutput {
	return &BatchOutput{
		ID:                 b.ID,
		ProductID:          b.ProductID,
		WarehouseCompanyID: b.WarehouseCompanyID,
		QuantityTotal:      b.QuantityTotal,
		QuantityReserved:   b.QuantityReserved,
		AvailableQuantity:  b.QuantityTotal - b.QuantityReserved,
		BatchNumber:        b.BatchNumber,
		ProductionDate:     b.ProductionDate,
		ExpirationDate:     b.ExpirationDate,
		CreatedAt:          b.CreatedAt,
	}
}

func toBatchListOutput(batches []batchmodel.ProductBatch) []BatchOutput {
	result := make([]BatchOutput, 0, len(batches))
	for _, b := range batches {
		result = append(result, *toBatchOutput(&b))
	}
	return result
}
