// Package security hashes passwords in the format the BFF already stored in app_users.password_hash,
// so existing accounts keep working after identity moved to MS1.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/scrypt"
)

// Node's crypto.scryptSync defaults (N=16384, r=8, p=1), which the BFF used with a 64-byte key.
const (
	scryptN      = 16384
	scryptR      = 8
	scryptP      = 1
	keyLength    = 64
	saltBytes    = 16
	hashAlgoName = "scrypt"
)

// HashPassword returns "scrypt$<salt>$<hex key>". Like the BFF, the salt is 16 random bytes written
// as hex, and that hex text (not the raw bytes) is what scrypt receives as salt.
func HashPassword(password string) (string, error) {
	raw := make([]byte, saltBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	salt := hex.EncodeToString(raw)
	key, err := scrypt.Key([]byte(password), []byte(salt), scryptN, scryptR, scryptP, keyLength)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return strings.Join([]string{hashAlgoName, salt, hex.EncodeToString(key)}, "$"), nil
}

// VerifyPassword checks a password against a stored hash in constant time. Malformed or unknown
// formats are a mismatch, never an error, exactly like the BFF's verifyPassword.
func VerifyPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 3 || parts[0] != hashAlgoName || parts[1] == "" || parts[2] == "" {
		return false
	}
	expected, err := hex.DecodeString(parts[2])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual, err := scrypt.Key([]byte(password), []byte(parts[1]), scryptN, scryptR, scryptP, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(expected, actual) == 1
}
