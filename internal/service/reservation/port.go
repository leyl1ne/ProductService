package reservation

import (
	"context"
	"time"

	"github.com/google/uuid"
	reservationmodel "github.com/leyl1ne/ProductService/internal/model/reservation"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=Repository
type Repository interface {
	WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
	GetProductAvailability(txCtx context.Context, productID uuid.UUID) ([]AvailableBatch, error)
	ReserveStock(txCtx context.Context, reservationmodel reservationmodel.StockReservation) (*reservationmodel.StockReservation, error)
	GetReservationsByOrderID(ctx context.Context, orderID uuid.UUID) ([]reservationmodel.StockReservation, error)
	DeleteReservationsByOrderID(txCtx context.Context, orderID uuid.UUID) error
	IncrementBatchReservedQuantity(txCtx context.Context, batchID uuid.UUID, quantity float64) error
	DecrementBatchReservedQuantity(txCtx context.Context, batchID uuid.UUID, quantity float64) error
	CommitBatchStock(txCtx context.Context, batchID uuid.UUID, quantity float64) error
}

// --- Input types ---

type ReserveItem struct {
	ProductID uuid.UUID
	Quantity  float64
}

type ReserveStockInput struct {
	OrderID uuid.UUID
	Items   []ReserveItem
}

// --- Shared types ---

type AvailableBatch struct {
	BatchID           uuid.UUID
	AvailableQuantity float64
	ExpirationDate    *time.Time
}

// --- Output types ---

type ReservedBatchInfo struct {
	BatchID   uuid.UUID
	ProductID uuid.UUID
	Quantity  float64
}

type ReserveStockOutput struct {
	OrderID uuid.UUID
	Batches []ReservedBatchInfo
}

type ReservationOutput struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	BatchID   uuid.UUID
	Quantity  float64
	CreatedAt time.Time
}

// --- Converters ---

func toReservationOutput(r reservationmodel.StockReservation) ReservationOutput {
	return ReservationOutput{
		ID:        r.ID,
		OrderID:   r.OrderID,
		BatchID:   r.BatchID,
		Quantity:  r.Quantity,
		CreatedAt: r.CreatedAt,
	}
}

func toReservationListOutput(reservations []reservationmodel.StockReservation) []ReservationOutput {
	result := make([]ReservationOutput, 0, len(reservations))
	for _, r := range reservations {
		result = append(result, toReservationOutput(r))
	}
	return result
}
