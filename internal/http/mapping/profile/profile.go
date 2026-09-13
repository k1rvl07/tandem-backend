package profile

import (
	cprofile "github.com/tandem/tandem/internal/contracts/profile"
	dtoprofile "github.com/tandem/tandem/internal/http/dto/profile"
)

func ToContractUpdateProfile(w dtoprofile.UpdateProfileRequest) cprofile.UpdateProfileRequest {
	return cprofile.UpdateProfileRequest{
		DisplayName: w.DisplayName,
		Bio:         w.Bio,
	}
}

func ToContractChangePassword(w dtoprofile.ChangePasswordRequest) cprofile.ChangePasswordRequest {
	return cprofile.ChangePasswordRequest{
		NewPassword:     w.NewPassword,
		CurrentPassword: w.CurrentPassword,
	}
}
