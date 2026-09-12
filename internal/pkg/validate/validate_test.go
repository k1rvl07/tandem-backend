package validate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeLogin(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"admin", "admin"},
		{"  Admin  ", "admin"},
		{"Dev.User", "dev.user"},
		{" AbC ", "abc"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, NormalizeLogin(tc.in))
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "empty", in: "", wantErr: "login is required"},
		{name: "too short", in: "ab", wantErr: "login must be at least 3 characters"},
		{name: "too long", in: strings.Repeat("a", 51), wantErr: "login must be at most 50 characters"},
		{name: "uppercase", in: "Admin", wantErr: "invalid login format"},
		{name: "space", in: "ad min", wantErr: "invalid login format"},
		{name: "invalid first char", in: "_admin", wantErr: "invalid login format"},
		{name: "min boundary", in: "abc", wantErr: ""},
		{name: "max boundary", in: strings.Repeat("a", 50), wantErr: ""},
		{name: "leading digit", in: "1admin", wantErr: ""},
		{name: "with separators", in: "dev.user_1-a", wantErr: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Login(tc.in)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestUUID(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "valid", in: "f47ac10b-58cc-4372-a567-0e02b2c3d479", wantErr: ""},
		{name: "empty", in: "", wantErr: "invalid id"},
		{name: "garbage", in: "not-a-uuid", wantErr: "invalid id"},
		{name: "short", in: "f47ac10b", wantErr: "invalid id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := UUID(tc.in)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "empty", in: "", wantErr: "name is required"},
		{name: "whitespace", in: "   ", wantErr: "name is required"},
		{name: "max boundary", in: strings.Repeat("я", 80), wantErr: ""},
		{name: "too long in runes", in: strings.Repeat("я", 81), wantErr: "name must be at most 80 characters"},
		{name: "trimmed", in: "  dev team  ", wantErr: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Name(tc.in)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestTitle(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "empty", in: "", wantErr: "title is required"},
		{name: "whitespace", in: "  ", wantErr: "title is required"},
		{name: "max boundary", in: strings.Repeat("и", 120), wantErr: ""},
		{name: "too long in runes", in: strings.Repeat("и", 121), wantErr: "title must be at most 120 characters"},
		{name: "trimmed", in: "  hello  ", wantErr: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Title(tc.in)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestPrefix(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "empty", in: "", wantErr: ""},
		{name: "single letter", in: "A", wantErr: ""},
		{name: "letters and digits", in: "E2E", wantErr: ""},
		{name: "with dash", in: "TST-1", wantErr: ""},
		{name: "max length", in: "A1-B2C3D4E", wantErr: ""},
		{name: "lowercase", in: "e2e", wantErr: "prefix must start with a letter, contain only A-Z, 0-9 and dashes, max 10 characters"},
		{name: "leading digit", in: "123", wantErr: "prefix must start with a letter, contain only A-Z, 0-9 and dashes, max 10 characters"},
		{name: "underscore", in: "A_1", wantErr: "prefix must start with a letter, contain only A-Z, 0-9 and dashes, max 10 characters"},
		{name: "too long", in: "A1-B2C3D4E5", wantErr: "prefix must start with a letter, contain only A-Z, 0-9 and dashes, max 10 characters"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Prefix(tc.in)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestColor(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "empty", in: "", wantErr: ""},
		{name: "lowercase hex", in: "a1b2c3", wantErr: ""},
		{name: "uppercase hex", in: "AABBCC", wantErr: ""},
		{name: "mixed hex", in: "aAbB0F", wantErr: ""},
		{name: "too short", in: "abc", wantErr: "color must be a 6-digit hex value"},
		{name: "non hex", in: "gggggg", wantErr: "color must be a 6-digit hex value"},
		{name: "too long", in: "ffffff0", wantErr: "color must be a 6-digit hex value"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Color(tc.in)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestPassword(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{name: "empty", in: "", wantErr: "password is required"},
		{name: "too short", in: "1234567", wantErr: "password must be at least 8 characters"},
		{name: "min boundary", in: "12345678", wantErr: ""},
		{name: "max boundary", in: strings.Repeat("a", 72), wantErr: ""},
		{name: "too long in bytes", in: strings.Repeat("a", 73), wantErr: "password must be at most 72 bytes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Password(tc.in)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
		})
	}
}
