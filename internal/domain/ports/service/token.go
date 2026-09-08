package service

import (
	"context"
	"time"
)

type TokenService interface {
	Generate(subject string, ttl time.Duration) (string, error)
	Parse(token string) (string, error)
	Revoke(ctx context.Context, subject string) error
}
