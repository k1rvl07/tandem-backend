package mapping

import (
	muser "github.com/tandem/tandem/internal/domain/models/user"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
)

func UserResponse(u *muser.User) *dauth.UserResponse {
	return &dauth.UserResponse{
		ID:          u.ID,
		Login:       u.Login,
		Role:        u.Role,
		DisplayName: u.DisplayName,
		Bio:         u.Bio,
		AvatarKey:   u.AvatarKey,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
