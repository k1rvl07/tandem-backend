package validate

import (
	"regexp"
	"strings"

	"github.com/google/uuid"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

const (
	minLoginLen    = 3
	maxLoginLen    = 50
	minPasswordLen = 8
	maxPasswordLen = 72
)

var loginRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,49}$`)

func NormalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

func Login(login string) error {
	if login == "" {
		return pkgerrors.NewValidationError("login is required")
	}
	if len([]byte(login)) < minLoginLen {
		return pkgerrors.NewValidationError("login must be at least %d characters", minLoginLen)
	}
	if len([]byte(login)) > maxLoginLen {
		return pkgerrors.NewValidationError("login must be at most %d characters", maxLoginLen)
	}
	if !loginRegexp.MatchString(login) {
		return pkgerrors.NewValidationError("invalid login format")
	}
	return nil
}

func UUID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return pkgerrors.NewValidationError("invalid id")
	}
	return nil
}

func Password(password string) error {
	if password == "" {
		return pkgerrors.NewValidationError("password is required")
	}
	if len([]byte(password)) < minPasswordLen {
		return pkgerrors.NewValidationError("password must be at least %d characters", minPasswordLen)
	}
	if len([]byte(password)) > maxPasswordLen {
		return pkgerrors.NewValidationError("password must be at most %d bytes", maxPasswordLen)
	}
	return nil
}
