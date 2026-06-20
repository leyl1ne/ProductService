package auth

import "github.com/google/uuid"

type Role string

const (
	RoleAdmin     Role = "ADMIN"
	RoleSeller    Role = "SELLER"
	RoleBuyer     Role = "BUYER"
	RoleWarehouse Role = "WAREHOUSE"
	RoleLogistics Role = "LOGISTICS"
)

type User struct {
	ID        uuid.UUID
	CompanyID uuid.UUID
	Role      Role
}
