package service

import "time"

type TokenService interface {
	Generate(subject string, ttl time.Duration) (string, error)
	Parse(token string) (string, error)
}
