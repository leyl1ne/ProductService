package auth

import "github.com/google/uuid"

func CanCreateProduct(user User) bool {

	if user.Role == RoleAdmin {
		return true
	}

	if user.Role == RoleSeller {
		return true
	}

	return false
}

func CanCreateBatch(user User) bool {
	if user.Role == RoleAdmin {
		return true
	}

	if user.Role == RoleSeller {
		return true
	}

	return false
}

func CanManageCategory(user User) bool {

	if user.Role == RoleAdmin {
		return true
	}

	return false
}

func CanManageCompanyResource(user User, companyID uuid.UUID) bool {

	if user.Role == RoleAdmin {
		return true
	}

	if user.CompanyID == companyID {
		return true
	}

	return false
}
