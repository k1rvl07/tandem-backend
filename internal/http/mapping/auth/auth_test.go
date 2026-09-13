package auth

import (
	"testing"
	"time"

	cauth "github.com/tandem/tandem/internal/contracts/auth"
	dtoauth "github.com/tandem/tandem/internal/http/dto/auth"
)

func TestToContractLogin(t *testing.T) {
	got := ToContractLogin(dtoauth.LoginRequest{Login: "admin", Password: "secret"})
	if got.Login != "admin" || got.Password != "secret" {
		t.Fatalf("ToContractLogin = %+v", got)
	}
}

func TestToContractRefresh(t *testing.T) {
	got := ToContractRefresh(dtoauth.RefreshRequest{RefreshToken: "tok"})
	if got.RefreshToken != "tok" {
		t.Fatalf("ToContractRefresh = %+v", got)
	}
}

func TestToWireUser(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := ToWireUser(cauth.UserResponse{
		ID: "u1", Login: "admin", Role: "owner", DisplayName: "Admin", Bio: "b",
		AvatarKey: "k", CreatedAt: at, UpdatedAt: at,
	})
	if got.ID != "u1" || got.Role != "owner" || got.DisplayName != "Admin" {
		t.Fatalf("ToWireUser = %+v", got)
	}
}

func TestToWireUsers(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := ToWireUsers([]cauth.UserResponse{{ID: "u1", CreatedAt: at}, {ID: "u2", CreatedAt: at}})
	if len(got) != 2 {
		t.Fatalf("ToWireUsers len = %d", len(got))
	}
}

func TestToWireLogin(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	user := cauth.UserResponse{ID: "u1", CreatedAt: at, UpdatedAt: at}
	got := ToWireLogin(cauth.LoginResponse{
		Token:        "access",
		RefreshToken: "refresh",
		User:         user,
	})
	if got.Token != "access" || got.RefreshToken != "refresh" || got.User.ID != "u1" {
		t.Fatalf("ToWireLogin = %+v", got)
	}
}

func TestToWireRefresh(t *testing.T) {
	got := ToWireRefresh(cauth.RefreshResponse{Token: "a", RefreshToken: "r"})
	if got.Token != "a" || got.RefreshToken != "r" {
		t.Fatalf("ToWireRefresh = %+v", got)
	}
}
