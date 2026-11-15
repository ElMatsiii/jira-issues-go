package security_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/security"
)

// Produced by the BFF's code (Node crypto.scryptSync(password, salt, 64)) for "contraseña-segura-2026".
const (
	bffPassword = "contraseña-segura-2026"
	bffHash     = "scrypt$9f2c4e6a8b0d1f3e5a7c9e1b3d5f7a90$" +
		"cc918220639e699ced6548bfa7fd92479f9816f89e0ec3259ac4a6b63e456dc6f761159d10ce371ad8ea3cc2fd8b2ec028b2697cd7ae97e992dd646fed11ffdd"
)

func TestVerifyPassword_AcceptsHashesCreatedByTheBFF(t *testing.T) {
	assert.True(t, security.VerifyPassword(bffPassword, bffHash))
	assert.False(t, security.VerifyPassword("contraseña-segura-2027", bffHash))
}

func TestHashPassword_UsesTheBFFFormat(t *testing.T) {
	hash, err := security.HashPassword(bffPassword)

	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^scrypt\$[0-9a-f]{32}\$[0-9a-f]{128}$`), hash)
	assert.True(t, security.VerifyPassword(bffPassword, hash))
	assert.False(t, security.VerifyPassword(bffPassword+" ", hash))

	again, err := security.HashPassword(bffPassword)
	require.NoError(t, err)
	assert.NotEqual(t, hash, again, "every hash gets its own salt")
}

func TestVerifyPassword_MalformedHashesNeverMatch(t *testing.T) {
	parts := strings.Split(bffHash, "$")
	for name, stored := range map[string]string{
		"empty":           "",
		"other algorithm": "bcrypt$" + parts[1] + "$" + parts[2],
		"missing salt":    "scrypt$$" + parts[2],
		"missing hash":    "scrypt$" + parts[1] + "$",
		"not hex":         "scrypt$" + parts[1] + "$zz",
		"too many parts":  bffHash + "$extra",
		"plain text":      bffPassword,
		"only two parts":  "scrypt$" + parts[1],
		"odd hex length":  "scrypt$" + parts[1] + "$" + parts[2][:127],
	} {
		t.Run(name, func(t *testing.T) {
			assert.False(t, security.VerifyPassword(bffPassword, stored))
		})
	}
}
