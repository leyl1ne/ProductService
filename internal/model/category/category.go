package category

import (
	"errors"
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
		return nil, errors.New("category name is required")
	}

	if len(name) > 100 {
		return nil, errors.New("category name is too long")
	}

	return &Category{
		ID:   uuid.New(),
		Name: name,
	}, nil
}
