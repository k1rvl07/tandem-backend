package service

import (
	"context"
	"time"
)

type TokenService interface {
	Generate(subject string, ttl time.Duration) (string, error)
	GenerateRefresh(subject string, ttl time.Duration) (string, error)
	Parse(token string) (string, error)
	ParseRefresh(token string) (string, error)
	RotateRefresh(ctx context.Context, refreshToken string, accessTTL, refreshTTL time.Duration) (string, string, error)
	Revoke(ctx context.Context, subject string) error
}
