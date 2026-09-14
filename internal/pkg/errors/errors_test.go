package pkgerrors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrapMakesBothErrorsMatch(t *testing.T) {
	root := errors.New("root cause")
	target := ErrValidation

	wrapped := Wrap(root, target)
	require.ErrorIs(t, wrapped, target)
	require.ErrorIs(t, wrapped, root)
	require.Error(t, wrapped)
	require.ErrorContains(t, wrapped, "validation failed")
	require.ErrorContains(t, wrapped, "root cause")
}

func TestWrapNilRoot(t *testing.T) {
	wrapped := Wrap(nil, ErrNotFound)
	require.ErrorIs(t, wrapped, ErrNotFound)
}

func TestWrapFormatting(t *testing.T) {
	wrapped := Wrap(fmt.Errorf("boom"), ErrInternal)
	var err error = wrapped
	require.Contains(t, err.Error(), "boom")
	require.Contains(t, err.Error(), "internal error")
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("bad field %s", "title")
	require.Equal(t, "bad field title", err.Error())
	require.ErrorIs(t, err, ErrValidation)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, "bad field title", ve.Message)
}

func TestSentinelErrors(t *testing.T) {
	require.NotNil(t, ErrNotFound)
	require.NotNil(t, ErrConflict)
	require.NotNil(t, ErrValidation)
	require.NotNil(t, ErrUnauthorized)
	require.NotNil(t, ErrForbidden)
	require.NotNil(t, ErrInternal)
	require.NotNil(t, ErrUnavailable)
	require.NotNil(t, ErrTooManyRequests)
	require.ErrorIs(t, NewValidationError("x"), ErrValidation)
}
