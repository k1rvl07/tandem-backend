package password

import "golang.org/x/crypto/bcrypt"

type BCryptHasher struct{}

func NewBCryptHasher() *BCryptHasher {
	return &BCryptHasher{}
}

func (h *BCryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *BCryptHasher) Check(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
