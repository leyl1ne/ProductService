package product

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID         uuid.UUID
	CompanyID  uuid.UUID
	CategoryID *uuid.UUID

	Name        string
	Description string

	Price float64
	Unit  string

	IsActive bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewProduct(
	companyID uuid.UUID,
	categoryID *uuid.UUID,
	name string,
	description string,
	price float64,
	unit string,
) (*Product, error) {

	name = strings.TrimSpace(name)
	unit = strings.TrimSpace(unit)

	switch {
	case companyID == uuid.Nil:
		return nil, ErrCompanyIDIsRequired

	case name == "":
		return nil, ErrNameIsRequired

	case len(name) > 255:
		return nil, ErrNameIsTooLong

	case price <= 0:
		return nil, ErrInvalidPrice

	case unit == "":
		return nil, ErrUnitIsRequired
	}

	now := time.Now()

	return &Product{
		ID:          uuid.New(),
		CompanyID:   companyID,
		CategoryID:  categoryID,
		Name:        name,
		Description: description,
		Price:       price,
		Unit:        unit,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
