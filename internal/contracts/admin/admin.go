package admincontracts

import authcontracts "github.com/tandem/tandem/internal/contracts/auth"

type CreateUserRequest struct {
	Login       string
	Password    string
	DisplayName string
	Role        string
}

type UpdateUserRoleRequest struct {
	Role string
}

type AdminListQuery struct {
	Page     int
	PageSize int
	Query    string
}

type AdminPage struct {
	Items    []authcontracts.UserResponse
	Total    int
	Page     int
	PageSize int
}
