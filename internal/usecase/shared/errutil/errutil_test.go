package errutil

import (
	"errors"
	"fmt"
	"testing"

	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(pkgerrors.ErrNotFound) {
		t.Fatal("ErrNotFound should match")
	}
	if !IsNotFound(fmt.Errorf("wrapped: %w", pkgerrors.ErrNotFound)) {
		t.Fatal("wrapped ErrNotFound should match")
	}
	if IsNotFound(errors.New("other")) {
		t.Fatal("other error should not match")
	}
	if IsNotFound(nil) {
		t.Fatal("nil error should not match")
	}
}
