package admin

import "github.com/tandem/tandem/internal/http/dto/auth"

type CreateUserRequest struct {
	Login       string `json:"login"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role"`
}

type AdminListQuery struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
	Query    string `form:"q" json:"q"`
}

type AdminPage struct {
	Items    []auth.UserResponse `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}
