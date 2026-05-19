// Package auth implements sigil-manager's single-admin authentication: a
// bcrypt-verified password and an HS256-signed JWT session cookie. Per
// CLAUDE.md and UI/UX §9 this is the ONLY auth in v1 — no SSO, no OAuth,
// no refresh tokens, no multi-user.
package auth

import "golang.org/x/crypto/bcrypt"

// DefaultBcryptCost is the cost used by [Hash]. 10 is golang.org/x/crypto's
// default and the bcrypt tooling default the .env.example points at.
const DefaultBcryptCost = 10

// Hash returns a bcrypt hash of plaintext at [DefaultBcryptCost]. Operator
// tooling + tests use this; the running process never calls it because
// ADMIN_PASSWORD_BCRYPT is set out-of-band.
func Hash(plaintext string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plaintext), DefaultBcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// Verify reports whether plaintext matches the bcrypt hash. Any error
// (mismatch, malformed hash, empty inputs) collapses to `false` — the
// caller never needs to distinguish.
func Verify(plaintext, hash string) bool {
	if plaintext == "" || hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext)) == nil
}
