package category

import (
	"github.com/google/uuid"
	categoryservice "github.com/leyl1ne/ProductService/internal/service/category"
)

type CreateCategoryRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
}

type UpdateCategoryRequest struct {
	Name string `json:"name" binding:"required,min=1,max=255"`
}

type CategoryResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type CategoryListResponse struct {
	Categories []CategoryResponse `json:"categories"`
}

func (r *CreateCategoryRequest) ToInput() categoryservice.CreateCategoryInput {
	return categoryservice.CreateCategoryInput{
		Name: r.Name,
	}
}

func (r *UpdateCategoryRequest) ToInput() categoryservice.UpdateCategoryInput {
	return categoryservice.UpdateCategoryInput{
		Name: r.Name,
	}
}

func toCategoryResponse(c *categoryservice.CategoryOutput) CategoryResponse {
	return CategoryResponse{
		ID:   c.ID,
		Name: c.Name,
	}
}

func toCategoryListResponse(list []categoryservice.CategoryOutput) CategoryListResponse {
	categories := make([]CategoryResponse, 0, len(list))
	for i := range list {
		categories = append(categories, toCategoryResponse(&list[i]))
	}
	return CategoryListResponse{
		Categories: categories,
	}
}
