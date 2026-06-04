package reservation

import (
	"time"

	"github.com/google/uuid"
)

type StockReservation struct {
	ID uuid.UUID

	OrderID uuid.UUID
	BatchID uuid.UUID

	Quantity float64

	CreatedAt time.Time
}

func NewStockReservation(
	orderID uuid.UUID,
	batchID uuid.UUID,
	quantity float64,
) (*StockReservation, error) {

	if orderID == uuid.Nil {
		return nil, ErrOrderIdIsRequired
	}

	if batchID == uuid.Nil {
		return nil, ErrBatchIDIsRequired
	}

	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}

	return &StockReservation{
		ID:        uuid.New(),
		OrderID:   orderID,
		BatchID:   batchID,
		Quantity:  quantity,
		CreatedAt: time.Now(),
	}, nil
}
