package product

import (
	"time"

	"github.com/google/uuid"
	productservice "github.com/leyl1ne/ProductService/internal/service/product"
)

type CreateProductRequest struct {
	Name        string  `json:"name" binding:"required,min=1,max=255"`
	Description string  `json:"description" binding:"omitempty,max=2000"`
	Price       float64 `json:"price" binding:"required,gt=0"`
	Unit        string  `json:"unit" binding:"required,min=1,max=50"`
	CategoryID  *string `json:"category_id" binding:"omitempty,uuid"`
}

type UpdateProductRequest struct {
	Name        *string  `json:"name" binding:"omitempty,min=1,max=255"`
	Description *string  `json:"description" binding:"omitempty,max=2000"`
	Price       *float64 `json:"price" binding:"omitempty,gt=0"`
	Unit        *string  `json:"unit" binding:"omitempty,min=1,max=50"`
	CategoryID  *string  `json:"category_id" binding:"omitempty,uuid"`
	IsActive    *bool    `json:"is_active"`
}

type ListProductsQuery struct {
	CategoryID *string  `form:"category_id" binding:"omitempty,uuid"`
	CompanyID  *string  `form:"company_id" binding:"omitempty,uuid"`
	MinPrice   *float64 `form:"min_price" binding:"omitempty,gte=0"`
	MaxPrice   *float64 `form:"max_price" binding:"omitempty,gte=0"`
	Page       int      `form:"page" binding:"omitempty,min=1"`
	Limit      int      `form:"limit" binding:"omitempty,min=1,max=100"`
}

type CategoryShortResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type ProductResponse struct {
	ID          uuid.UUID              `json:"id"`
	CompanyID   uuid.UUID              `json:"company_id"`
	Category    *CategoryShortResponse `json:"category,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Price       float64                `json:"price"`
	Unit        string                 `json:"unit"`
	IsActive    bool                   `json:"is_active"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

type ProductListResponse struct {
	Products []ProductResponse `json:"products"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	Limit    int               `json:"limit"`
}

type AvailableBatchResponse struct {
	BatchID           uuid.UUID `json:"batch_id"`
	AvailableQuantity float64   `json:"available_quantity"`
	ExpirationDate    *string   `json:"expiration_date,omitempty"`
}

type ProductAvailabilityResponse struct {
	ProductID uuid.UUID                `json:"product_id"`
	Batches   []AvailableBatchResponse `json:"batches"`
}

func (r *CreateProductRequest) ToInput() (productservice.CreateProductInput, error) {
	var categoryID *uuid.UUID
	if r.CategoryID != nil && *r.CategoryID != "" {
		parsed, err := uuid.Parse(*r.CategoryID)
		if err != nil {
			return productservice.CreateProductInput{}, err
		}
		categoryID = &parsed
	}

	return productservice.CreateProductInput{
		CategoryID: categoryID,
		Name:       r.Name,
		Desc:       r.Description,
		Price:      r.Price,
		Unit:       r.Unit,
	}, nil
}

func (r *UpdateProductRequest) ToInput() (productservice.UpdateProductInput, error) {
	var categoryID *uuid.UUID
	if r.CategoryID != nil && *r.CategoryID != "" {
		parsed, err := uuid.Parse(*r.CategoryID)
		if err != nil {
			return productservice.UpdateProductInput{}, err
		}
		categoryID = &parsed
	}

	return productservice.UpdateProductInput{
		Name:        r.Name,
		Description: r.Description,
		Price:       r.Price,
		Unit:        r.Unit,
		CategoryID:  categoryID,
		IsActive:    r.IsActive,
	}, nil
}

func (q *ListProductsQuery) ToFilter() (productservice.ProductFilter, error) {
	var categoryID *uuid.UUID
	if q.CategoryID != nil && *q.CategoryID != "" {
		parsed, err := uuid.Parse(*q.CategoryID)
		if err != nil {
			return productservice.ProductFilter{}, err
		}
		categoryID = &parsed
	}

	var companyID *uuid.UUID
	if q.CompanyID != nil && *q.CompanyID != "" {
		parsed, err := uuid.Parse(*q.CompanyID)
		if err != nil {
			return productservice.ProductFilter{}, err
		}
		companyID = &parsed
	}

	return productservice.ProductFilter{
		CategoryID: categoryID,
		CompanyID:  companyID,
		MinPrice:   q.MinPrice,
		MaxPrice:   q.MaxPrice,
	}, nil
}

func toProductResponse(p *productservice.ProductOutput) ProductResponse {
	resp := ProductResponse{
		ID:          p.ID,
		CompanyID:   p.CompanyID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Unit:        p.Unit,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.UTC().Format(time.RFC3339),
	}

	if p.Category != nil {
		resp.Category = &CategoryShortResponse{
			ID:   p.Category.ID,
			Name: p.Category.Name,
		}
	}

	return resp
}

func toProductListResponse(list *productservice.ProductListOutput, page, limit int) ProductListResponse {
	products := make([]ProductResponse, 0, len(list.Products))
	for i := range list.Products {
		products = append(products, toProductResponse(&list.Products[i]))
	}

	return ProductListResponse{
		Products: products,
		Total:    list.Total,
		Page:     page,
		Limit:    limit,
	}
}

func toProductAvailabilityResponse(a *productservice.ProductAvailabilityOutput) ProductAvailabilityResponse {
	batches := make([]AvailableBatchResponse, 0, len(a.Batches))
	for _, b := range a.Batches {
		var expDate *string
		if b.ExpirationDate != nil {
			s := b.ExpirationDate.UTC().Format("2006-01-02")
			expDate = &s
		}
		batches = append(batches, AvailableBatchResponse{
			BatchID:           b.BatchID,
			AvailableQuantity: b.AvailableQuantity,
			ExpirationDate:    expDate,
		})
	}

	return ProductAvailabilityResponse{
		ProductID: a.ProductID,
		Batches:   batches,
	}
}
