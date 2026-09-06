package validate

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

const (
	minLoginLen    = 3
	maxLoginLen    = 50
	minPasswordLen = 8
	maxPasswordLen = 72

	maxTitleLen = 120
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

const maxNameLen = 80

func Name(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return pkgerrors.NewValidationError("name is required")
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		return pkgerrors.NewValidationError("name must be at most %d characters", maxNameLen)
	}
	return nil
}

func Title(title string) error {
	if strings.TrimSpace(title) == "" {
		return pkgerrors.NewValidationError("title is required")
	}
	if utf8.RuneCountInString(strings.TrimSpace(title)) > maxTitleLen {
		return pkgerrors.NewValidationError("title must be at most %d characters", maxTitleLen)
	}
	return nil
}

var prefixRegexp = regexp.MustCompile(`^[A-Z][A-Z0-9-]{0,9}$`)
var colorRegexp = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

func Prefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	if !prefixRegexp.MatchString(prefix) {
		return pkgerrors.NewValidationError("prefix must start with a letter, contain only A-Z, 0-9 and dashes, max 10 characters")
	}
	return nil
}

func Color(color string) error {
	if color == "" {
		return nil
	}
	if !colorRegexp.MatchString(color) {
		return pkgerrors.NewValidationError("color must be a 6-digit hex value")
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
