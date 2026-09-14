package authcontracts

import "time"

type LoginRequest struct {
	Login    string
	Password string
}

type UserResponse struct {
	ID          string    `json:"id"`
	Login       string    `json:"login"`
	Role        string    `json:"role"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	AvatarKey   string    `json:"avatar_key"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type LoginResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	User         UserResponse `json:"user"`
}

type RefreshRequest struct {
	RefreshToken string
}

type RefreshResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}
