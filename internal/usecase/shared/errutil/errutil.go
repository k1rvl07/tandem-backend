package errutil

import (
	"errors"

	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

func IsNotFound(err error) bool {
	return errors.Is(err, pkgerrors.ErrNotFound)
}
