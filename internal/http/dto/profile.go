package dto

type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
}

type ChangePasswordRequest struct {
	NewPassword     string `json:"new_password"`
	CurrentPassword string `json:"current_password"`
}
