package category

import "errors"

var (
	ErrNameIsReuired = errors.New("category name is required")
	ErrNameIsTooLong = errors.New("category name is too long")
)
