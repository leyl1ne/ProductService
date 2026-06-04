package category

import (
	"strings"

	"github.com/google/uuid"
)

type Category struct {
	ID   uuid.UUID
	Name string
}

func NewCategory(name string) (*Category, error) {

	name = strings.TrimSpace(name)

	if name == "" {
		return nil, ErrNameIsReuired
	}

	if len(name) > 100 {
		return nil, ErrNameIsTooLong
	}

	return &Category{
		ID:   uuid.New(),
		Name: name,
	}, nil
}
