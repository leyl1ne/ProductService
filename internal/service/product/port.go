package product

import (
	"context"
	"time"

	"github.com/google/uuid"
	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
	productmodel "github.com/leyl1ne/ProductService/internal/model/product"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=Repository
type Repository interface {
	CreateProduct(ctx context.Context, product productmodel.Product) (*productmodel.Product, error)
	GetProductByID(ctx context.Context, id uuid.UUID) (*productmodel.Product, error)
	ListProducts(ctx context.Context, filter ProductFilter, page, limit int) (ProductListResult, error)
	UpdateProduct(ctx context.Context, id uuid.UUID, params UpdateProductParams) (*productmodel.Product, error)
	DeleteProduct(ctx context.Context, id uuid.UUID) error
	GetProductAvailability(ctx context.Context, productID uuid.UUID) ([]AvailableBatch, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (*categorymodel.Category, error)
	ListCategories(ctx context.Context) ([]categorymodel.Category, error)
}

// --- Input types ---

type CreateProductInput struct {
	CompanyID  uuid.UUID
	CategoryID *uuid.UUID
	Name       string
	Desc       string
	Price      float64
	Unit       string
}

type UpdateProductInput struct {
	Name        *string
	Description *string
	Price       *float64
	Unit        *string
	CategoryID  *uuid.UUID
	IsActive    *bool
}

type ProductFilter struct {
	CategoryID *uuid.UUID
	CompanyID  *uuid.UUID
	MinPrice   *float64
	MaxPrice   *float64
}

// --- Repository parameter types ---

type UpdateProductParams struct {
	Name        *string
	Description *string
	Price       *float64
	Unit        *string
	CategoryID  *uuid.UUID
	IsActive    *bool
}

type AvailableBatch struct {
	BatchID           uuid.UUID
	AvailableQuantity float64
	ExpirationDate    *time.Time
}

type ProductListResult struct {
	Products []productmodel.Product
	Total    int
}

// --- Output types ---

type CategoryShortOutput struct {
	ID   uuid.UUID
	Name string
}

type ProductOutput struct {
	ID          uuid.UUID
	CompanyID   uuid.UUID
	CategoryID  *uuid.UUID
	Category    *CategoryShortOutput
	Name        string
	Description string
	Price       float64
	Unit        string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProductListOutput struct {
	Products []ProductOutput
	Total    int
}

type AvailableBatchOutput struct {
	BatchID           uuid.UUID
	AvailableQuantity float64
	ExpirationDate    *time.Time
}

type ProductAvailabilityOutput struct {
	ProductID uuid.UUID
	Batches   []AvailableBatchOutput
}

// --- Converters ---

func toProductOutput(p *productmodel.Product, category *CategoryShortOutput) *ProductOutput {
	return &ProductOutput{
		ID:          p.ID,
		CompanyID:   p.CompanyID,
		CategoryID:  p.CategoryID,
		Category:    category,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Unit:        p.Unit,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func toCategoryShortOutput(c *categorymodel.Category) *CategoryShortOutput {
	return &CategoryShortOutput{
		ID:   c.ID,
		Name: c.Name,
	}
}
