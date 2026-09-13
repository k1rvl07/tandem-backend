package admin

import (
	cadmin "github.com/tandem/tandem/internal/contracts/admin"
	dtoadmin "github.com/tandem/tandem/internal/http/dto/admin"
	mpauth "github.com/tandem/tandem/internal/http/mapping/auth"
)

func ToContractCreateUser(w dtoadmin.CreateUserRequest) cadmin.CreateUserRequest {
	return cadmin.CreateUserRequest{
		Login:       w.Login,
		Password:    w.Password,
		DisplayName: w.DisplayName,
		Role:        w.Role,
	}
}

func ToContractUpdateRole(w dtoadmin.UpdateUserRoleRequest) cadmin.UpdateUserRoleRequest {
	return cadmin.UpdateUserRoleRequest{Role: w.Role}
}

func ToContractListQuery(w dtoadmin.AdminListQuery) cadmin.AdminListQuery {
	return cadmin.AdminListQuery{
		Page:     w.Page,
		PageSize: w.PageSize,
		Query:    w.Query,
	}
}

func ToWirePage(c cadmin.AdminPage) dtoadmin.AdminPage {
	return dtoadmin.AdminPage{
		Items:    mpauth.ToWireUsers(c.Items),
		Total:    c.Total,
		Page:     c.Page,
		PageSize: c.PageSize,
	}
}
