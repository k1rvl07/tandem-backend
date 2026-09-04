package service

type PasswordHasher interface {
	Hash(password string) (string, error)
	Check(hash, password string) bool
}
