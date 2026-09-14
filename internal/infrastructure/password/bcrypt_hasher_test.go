package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCheck(t *testing.T) {
	h := NewBCryptHasher(bcrypt.DefaultCost)
	hash, err := h.Hash("secret-password")
	require.NoError(t, err)
	require.NotEqual(t, "secret-password", hash)
	require.True(t, h.Check(hash, "secret-password"))
	require.False(t, h.Check(hash, "wrong-password"))
	require.False(t, h.Check(hash, ""))
}

func TestHashInvalidCostFallsBackToDefault(t *testing.T) {
	low := NewBCryptHasher(bcrypt.MinCost - 1)
	require.Equal(t, defaultCost, low.cost)

	high := NewBCryptHasher(32)
	require.Equal(t, defaultCost, high.cost)

	valid := NewBCryptHasher(bcrypt.DefaultCost)
	require.Equal(t, bcrypt.DefaultCost, valid.cost)
}

func TestCheckInvalidHash(t *testing.T) {
	h := NewBCryptHasher(bcrypt.DefaultCost)
	require.False(t, h.Check("not-a-hash", "password"))
	require.False(t, h.Check("", "password"))
}

func TestHashRejectsOversizedPassword(t *testing.T) {
	h := NewBCryptHasher(bcrypt.DefaultCost)
	_, err := h.Hash(strings.Repeat("x", 73))
	require.Error(t, err)
}
