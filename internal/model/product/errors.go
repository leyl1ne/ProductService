package product

import "errors"

var (
	ErrCompanyIDIsRequired = errors.New("company id is required")
	ErrNameIsRequired      = errors.New("product name is required")
	ErrNameIsTooLong       = errors.New("product name is too long")
	ErrInvalidPrice        = errors.New("price must be greater than zero")
	ErrUnitIsRequired      = errors.New("unit is required")
)
