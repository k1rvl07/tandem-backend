package auth

import (
	cauth "github.com/tandem/tandem/internal/contracts/auth"
	dtoauth "github.com/tandem/tandem/internal/http/dto/auth"
)

func ToContractLogin(w dtoauth.LoginRequest) cauth.LoginRequest {
	return cauth.LoginRequest{Login: w.Login, Password: w.Password}
}

func ToContractRefresh(w dtoauth.RefreshRequest) cauth.RefreshRequest {
	return cauth.RefreshRequest{RefreshToken: w.RefreshToken}
}

func ToWireUser(c cauth.UserResponse) dtoauth.UserResponse {
	return dtoauth.UserResponse{
		ID:          c.ID,
		Login:       c.Login,
		Role:        c.Role,
		DisplayName: c.DisplayName,
		Bio:         c.Bio,
		AvatarKey:   c.AvatarKey,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func ToWireUsers(cc []cauth.UserResponse) []dtoauth.UserResponse {
	out := make([]dtoauth.UserResponse, 0, len(cc))
	for _, c := range cc {
		out = append(out, ToWireUser(c))
	}
	return out
}

func ToWireLogin(c cauth.LoginResponse) dtoauth.LoginResponse {
	return dtoauth.LoginResponse{
		Token:        c.Token,
		RefreshToken: c.RefreshToken,
		User:         ToWireUser(c.User),
	}
}

func ToWireRefresh(c cauth.RefreshResponse) dtoauth.RefreshResponse {
	return dtoauth.RefreshResponse{
		Token:        c.Token,
		RefreshToken: c.RefreshToken,
	}
}
