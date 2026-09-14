package profilecontracts

type UpdateProfileRequest struct {
	DisplayName string
	Bio         string
}

type ChangePasswordRequest struct {
	NewPassword     string
	CurrentPassword string
}

type ChangePasswordResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}
