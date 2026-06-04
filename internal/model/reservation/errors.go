package reservation

import "errors"

var (
	ErrOrderIdIsRequired = errors.New("order id is required")
	ErrBatchIDIsRequired = errors.New("batch id is required")
	ErrInvalidQuantity   = errors.New("quantity must be greater than zero")
)
