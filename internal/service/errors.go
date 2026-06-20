package service

import "errors"

var (
	ErrProductNotFound       = errors.New("product not found")
	ErrCategoryNotFound      = errors.New("category not found")
	ErrCategoryAlreadyExists = errors.New("category already exists")
	ErrCategoryInUse         = errors.New("category in use")
	ErrBatchNotFound         = errors.New("batch not found")
	ErrInsufficientStock     = errors.New("insufficient stock")
	ErrForbidden             = errors.New("forbidden")
	ErrBadRequest            = errors.New("bad request")
)
