package batch

import (
	"time"

	"github.com/google/uuid"
)

type ProductBatch struct {
	ID                 uuid.UUID
	ProductID          uuid.UUID
	WarehouseCompanyID *uuid.UUID

	QuantityTotal    float64
	QuantityReserved float64

	ProductionDate *time.Time
	ExpirationDate *time.Time

	BatchNumber string

	CreatedAt time.Time
}

func NewProductBatch(
	productID uuid.UUID,
	warehouseCompanyID *uuid.UUID,
	quantityTotal float64,
	productionDate *time.Time,
	expirationDate *time.Time,
	batchNumber string,
) (*ProductBatch, error) {

	if productID == uuid.Nil {
		return nil, ErrProductIDIsRequired
	}

	if quantityTotal <= 0 {
		return nil, ErrQuantityIsZero
	}

	if productionDate != nil &&
		expirationDate != nil &&
		expirationDate.Before(*productionDate) {
		return nil, ErrIncorrectDateSequence
	}

	return &ProductBatch{
		ID:                 uuid.New(),
		ProductID:          productID,
		WarehouseCompanyID: warehouseCompanyID,
		QuantityTotal:      quantityTotal,
		QuantityReserved:   0,
		ProductionDate:     productionDate,
		ExpirationDate:     expirationDate,
		BatchNumber:        batchNumber,
		CreatedAt:          time.Now(),
	}, nil
}

func (b *ProductBatch) AvailableQuantity() float64 {
	return b.QuantityTotal - b.QuantityReserved
}
