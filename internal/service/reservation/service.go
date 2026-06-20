package reservation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	batchmodel "github.com/leyl1ne/ProductService/internal/model/batch"
	stockmodel "github.com/leyl1ne/ProductService/internal/model/reservation"
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

// ReserveStock reserves stock for an order. Called by Order Service via gRPC.
// It uses FIFO strategy — batches with earliest expiration_date are reserved first.
// The entire operation runs in a single transaction: either all items are reserved or none.
func (s *Service) ReserveStock(ctx context.Context, input ReserveStockInput) (*ReserveStockOutput, error) {
	const op = "service.reservation.ReserveStock"

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var output ReserveStockOutput

	err := s.repo.WithTransaction(ctxTimeout, func(txCtx context.Context) error {
		output = ReserveStockOutput{
			OrderID: input.OrderID,
			Batches: make([]ReservedBatchInfo, 0),
		}

		for _, item := range input.Items {
			remaining := item.Quantity

			// Get available batches ordered by expiration_date (FIFO)
			batches, err := s.repo.GetProductAvailability(txCtx, item.ProductID)
			if err != nil {
				return fmt.Errorf("get availability for product %s: %w", item.ProductID, err)
			}

			for _, batch := range batches {
				if remaining <= 0 {
					break
				}

				reserveQty := remaining
				if batch.AvailableQuantity < reserveQty {
					reserveQty = batch.AvailableQuantity
				}

				reservation := stockmodel.StockReservation{
					ID:        uuid.New(),
					OrderID:   input.OrderID,
					BatchID:   batch.BatchID,
					Quantity:  reserveQty,
					CreatedAt: time.Now(),
				}

				if _, err := s.repo.ReserveStock(txCtx, reservation); err != nil {
					if errors.Is(err, stockmodel.ErrInsufficientStock) {
						return fmt.Errorf("insufficient stock in batch %s: %w", batch.BatchID, service.ErrInsufficientStock)
					}
					return fmt.Errorf("reserve stock for batch %s: %w", batch.BatchID, err)
				}

				output.Batches = append(output.Batches, ReservedBatchInfo{
					BatchID:   batch.BatchID,
					ProductID: item.ProductID,
					Quantity:  reserveQty,
				})

				remaining -= reserveQty
			}

			if remaining > 0 {
				return fmt.Errorf("not enough stock for product %s: need %.2f more: %w",
					item.ProductID, remaining, service.ErrInsufficientStock)
			}
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, service.ErrInsufficientStock) {
			return nil, fmt.Errorf("%s: %w", op, service.ErrInsufficientStock)
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &output, nil
}

// ReleaseStock releases all reservations for an order (e.g., order cancelled).
// Called by Order Service via gRPC.
// It decrements the reserved quantity on each batch and deletes the reservation records.
func (s *Service) ReleaseStock(ctx context.Context, orderID uuid.UUID) error {
	const op = "service.reservation.ReleaseStock"

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := s.repo.WithTransaction(ctxTimeout, func(txCtx context.Context) error {
		reservations, err := s.repo.GetReservationsByOrderID(txCtx, orderID)
		if err != nil {
			return fmt.Errorf("get reservations by order id: %w", err)
		}

		if len(reservations) == 0 {
			return nil // nothing to release
		}

		// Decrement reserved quantity for each batch
		batchQuantities := make(map[uuid.UUID]float64)
		for _, r := range reservations {
			batchQuantities[r.BatchID] += r.Quantity
		}

		for batchID, qty := range batchQuantities {
			if err := s.repo.DecrementBatchReservedQuantity(txCtx, batchID, qty); err != nil {
				if errors.Is(err, batchmodel.ErrBatchNotFound) {
					return fmt.Errorf("batch %s not found during release: %w", batchID, service.ErrBatchNotFound)
				}
				return fmt.Errorf("decrement batch reserved quantity for batch %s: %w", batchID, err)
			}
		}

		// Delete all reservations for this order
		if err := s.repo.DeleteReservationsByOrderID(txCtx, orderID); err != nil {
			return fmt.Errorf("delete reservations by order id: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// CommitStock commits all reservations for an order (e.g., order fulfilled/delivered).
// Called by Order Service via gRPC.
// It decreases both quantity_total and quantity_reserved on each batch, then deletes reservations.
func (s *Service) CommitStock(ctx context.Context, orderID uuid.UUID) error {
	const op = "service.reservation.CommitStock"

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := s.repo.WithTransaction(ctxTimeout, func(txCtx context.Context) error {
		reservations, err := s.repo.GetReservationsByOrderID(txCtx, orderID)
		if err != nil {
			return fmt.Errorf("get reservations by order id: %w", err)
		}

		if len(reservations) == 0 {
			return nil // nothing to commit
		}

		// Aggregate quantities per batch
		batchQuantities := make(map[uuid.UUID]float64)
		for _, r := range reservations {
			batchQuantities[r.BatchID] += r.Quantity
		}

		// Commit stock: decrease both quantity_total and quantity_reserved
		for batchID, qty := range batchQuantities {
			if err := s.repo.CommitBatchStock(txCtx, batchID, qty); err != nil {
				if errors.Is(err, batchmodel.ErrBatchNotFound) {
					return fmt.Errorf("batch %s not found during commit: %w", batchID, service.ErrBatchNotFound)
				}
				return fmt.Errorf("commit batch stock for batch %s: %w", batchID, err)
			}
		}

		// Delete all reservations for this order
		if err := s.repo.DeleteReservationsByOrderID(txCtx, orderID); err != nil {
			return fmt.Errorf("delete reservations by order id: %w", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
