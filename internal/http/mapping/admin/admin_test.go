package admin

import (
	"reflect"
	"testing"
	"time"

	cadmin "github.com/tandem/tandem/internal/contracts/admin"
	cauth "github.com/tandem/tandem/internal/contracts/auth"
	dtoadmin "github.com/tandem/tandem/internal/http/dto/admin"
)

func TestToContractCreateUser(t *testing.T) {
	got := ToContractCreateUser(dtoadmin.CreateUserRequest{
		Login:       "user",
		Password:    "pass",
		DisplayName: "User",
		Role:        "staff",
	})
	if !reflect.DeepEqual(got, cadmin.CreateUserRequest{
		Login:       "user",
		Password:    "pass",
		DisplayName: "User",
		Role:        "staff",
	}) {
		t.Fatalf("ToContractCreateUser = %+v", got)
	}
}

func TestToContractUpdateRole(t *testing.T) {
	got := ToContractUpdateRole(dtoadmin.UpdateUserRoleRequest{Role: "member"})
	if !reflect.DeepEqual(got, cadmin.UpdateUserRoleRequest{Role: "member"}) {
		t.Fatalf("ToContractUpdateRole = %+v", got)
	}
}

func TestToContractListQuery(t *testing.T) {
	got := ToContractListQuery(dtoadmin.AdminListQuery{Page: 2, PageSize: 25, Query: "qa"})
	if !reflect.DeepEqual(got, cadmin.AdminListQuery{Page: 2, PageSize: 25, Query: "qa"}) {
		t.Fatalf("ToContractListQuery = %+v", got)
	}
}

func TestToWirePage(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := cadmin.AdminPage{
		Items:    []cauth.UserResponse{{ID: "u1", CreatedAt: at, UpdatedAt: at}},
		Total:    1,
		Page:     1,
		PageSize: 25,
	}
	got := ToWirePage(c)
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != "u1" {
		t.Fatalf("ToWirePage = %+v", got)
	}
}
