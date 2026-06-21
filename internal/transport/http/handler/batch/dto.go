package batch

import (
	"time"

	"github.com/google/uuid"
	batchservice "github.com/leyl1ne/ProductService/internal/service/batch"
)

type CreateBatchRequest struct {
	WarehouseCompanyID *string `json:"warehouse_company_id" binding:"omitempty,uuid"`
	QuantityTotal      float64 `json:"quantity_total" binding:"required,gt=0"`
	ProductionDate     *string `json:"production_date" binding:"omitempty,datetime=2006-01-02"`
	ExpirationDate     *string `json:"expiration_date" binding:"omitempty,datetime=2006-01-02"`
	BatchNumber        string  `json:"batch_number" binding:"omitempty,max=100"`
}

type UpdateBatchRequest struct {
	ProductionDate *string `json:"production_date" binding:"omitempty,datetime=2006-01-02"`
	ExpirationDate *string `json:"expiration_date" binding:"omitempty,datetime=2006-01-02"`
	BatchNumber    *string `json:"batch_number" binding:"omitempty,max=100"`
}

type BatchResponse struct {
	ID                 uuid.UUID  `json:"id"`
	ProductID          uuid.UUID  `json:"product_id"`
	WarehouseCompanyID *uuid.UUID `json:"warehouse_company_id,omitempty"`
	QuantityTotal      float64    `json:"quantity_total"`
	QuantityReserved   float64    `json:"quantity_reserved"`
	AvailableQuantity  float64    `json:"available_quantity"`
	BatchNumber        string     `json:"batch_number"`
	ProductionDate     *string    `json:"production_date,omitempty"`
	ExpirationDate     *string    `json:"expiration_date,omitempty"`
	CreatedAt          string     `json:"created_at"`
}

type BatchListResponse struct {
	Batches []BatchResponse `json:"batches"`
}

func (r *CreateBatchRequest) ToInput() (batchservice.CreateBatchInput, error) {
	var warehouseCompanyID *uuid.UUID
	if r.WarehouseCompanyID != nil && *r.WarehouseCompanyID != "" {
		parsed, err := uuid.Parse(*r.WarehouseCompanyID)
		if err != nil {
			return batchservice.CreateBatchInput{}, err
		}
		warehouseCompanyID = &parsed
	}

	var productionDate *time.Time
	if r.ProductionDate != nil && *r.ProductionDate != "" {
		t, err := time.Parse("2006-01-02", *r.ProductionDate)
		if err != nil {
			return batchservice.CreateBatchInput{}, err
		}
		productionDate = &t
	}

	var expirationDate *time.Time
	if r.ExpirationDate != nil && *r.ExpirationDate != "" {
		t, err := time.Parse("2006-01-02", *r.ExpirationDate)
		if err != nil {
			return batchservice.CreateBatchInput{}, err
		}
		expirationDate = &t
	}

	return batchservice.CreateBatchInput{
		WarehouseCompanyID: warehouseCompanyID,
		QuantityTotal:      r.QuantityTotal,
		ProductionDate:     productionDate,
		ExpirationDate:     expirationDate,
		BatchNumber:        r.BatchNumber,
	}, nil
}

func (r *UpdateBatchRequest) ToInput() (batchservice.UpdateBatchInput, error) {
	var productionDate *time.Time
	if r.ProductionDate != nil && *r.ProductionDate != "" {
		t, err := time.Parse("2006-01-02", *r.ProductionDate)
		if err != nil {
			return batchservice.UpdateBatchInput{}, err
		}
		productionDate = &t
	}

	var expirationDate *time.Time
	if r.ExpirationDate != nil && *r.ExpirationDate != "" {
		t, err := time.Parse("2006-01-02", *r.ExpirationDate)
		if err != nil {
			return batchservice.UpdateBatchInput{}, err
		}
		expirationDate = &t
	}

	return batchservice.UpdateBatchInput{
		ProductionDate: productionDate,
		ExpirationDate: expirationDate,
		BatchNumber:    r.BatchNumber,
	}, nil
}

func toBatchResponse(b *batchservice.BatchOutput) BatchResponse {
	resp := BatchResponse{
		ID:                 b.ID,
		ProductID:          b.ProductID,
		WarehouseCompanyID: b.WarehouseCompanyID,
		QuantityTotal:      b.QuantityTotal,
		QuantityReserved:   b.QuantityReserved,
		AvailableQuantity:  b.AvailableQuantity,
		BatchNumber:        b.BatchNumber,
		CreatedAt:          b.CreatedAt.UTC().Format(time.RFC3339),
	}

	if b.ProductionDate != nil {
		s := b.ProductionDate.UTC().Format("2006-01-02")
		resp.ProductionDate = &s
	}

	if b.ExpirationDate != nil {
		s := b.ExpirationDate.UTC().Format("2006-01-02")
		resp.ExpirationDate = &s
	}

	return resp
}

func toBatchListResponse(list []batchservice.BatchOutput) BatchListResponse {
	batches := make([]BatchResponse, 0, len(list))
	for i := range list {
		batches = append(batches, toBatchResponse(&list[i]))
	}
	return BatchListResponse{
		Batches: batches,
	}
}
