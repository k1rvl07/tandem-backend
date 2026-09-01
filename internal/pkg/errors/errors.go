package pkgerrors

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrValidation      = errors.New("validation failed")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrInternal        = errors.New("internal error")
	ErrUnavailable     = errors.New("service unavailable")
	ErrTooManyRequests = errors.New("too many requests")
)

func Wrap(err error, target error) error {
	return errors.Join(target, err)
}
