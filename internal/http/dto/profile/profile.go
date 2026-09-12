package profile

type UpdateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
}

type ChangePasswordRequest struct {
	NewPassword     string `json:"new_password"`
	CurrentPassword string `json:"current_password"`
}

type ChangePasswordResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}
