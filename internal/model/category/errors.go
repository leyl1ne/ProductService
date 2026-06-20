package category

import "errors"

var (
	ErrNameIsReuired         = errors.New("category name is required")
	ErrNameIsTooLong         = errors.New("category name is too long")
	ErrCategoryAlreadyExists = errors.New("category already exists")
	ErrCategoryNotFound      = errors.New("category not found")
	ErrCategoryInUse         = errors.New("category is in use by products")
)
