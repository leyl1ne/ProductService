package category

import (
	"context"

	"github.com/google/uuid"
	categorymodel "github.com/leyl1ne/ProductService/internal/model/category"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=Repository
type Repository interface {
	CreateCategory(ctx context.Context, category categorymodel.Category) (*categorymodel.Category, error)
	GetCategoryByID(ctx context.Context, id uuid.UUID) (*categorymodel.Category, error)
	ListCategories(ctx context.Context) ([]categorymodel.Category, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, name string) (*categorymodel.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
}

type CreateCategoryInput struct {
	Name string
}

type UpdateCategoryInput struct {
	Name string
}

type CategoryOutput struct {
	ID   uuid.UUID
	Name string
}

func toCategoryOutput(c *categorymodel.Category) *CategoryOutput {
	return &CategoryOutput{
		ID:   c.ID,
		Name: c.Name,
	}
}

func toCategoryListOutput(categories []categorymodel.Category) []CategoryOutput {
	result := make([]CategoryOutput, 0, len(categories))
	for _, c := range categories {
		result = append(result, *toCategoryOutput(&c))
	}
	return result
}
