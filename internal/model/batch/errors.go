package batch

import "errors"

var (
	ErrProductIDIsRequired   = errors.New("product id is required")
	ErrQuantityIsZero        = errors.New("quantity must be greater than zero")
	ErrIncorrectDateSequence = errors.New("expiration date before production date")
)
