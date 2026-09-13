package authcontracts

import "time"

type LoginRequest struct {
	Login    string
	Password string
}

type UserResponse struct {
	ID          string
	Login       string
	Role        string
	DisplayName string
	Bio         string
	AvatarKey   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type LoginResponse struct {
	Token        string
	RefreshToken string
	User         UserResponse
}

type RefreshRequest struct {
	RefreshToken string
}

type RefreshResponse struct {
	Token        string
	RefreshToken string
}
