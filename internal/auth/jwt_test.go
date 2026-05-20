package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "0123456789abcdefghijklmnopqrstuv" // 32 bytes

// newTestSigner returns a Signer with a fixed initial clock so tests can
// advance time deterministically via clk.advance().
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestSigner(t *testing.T) (*Signer, *fakeClock) {
	t.Helper()
	s, err := NewSigner(testSecret, 12*time.Hour)
	require.NoError(t, err)
	clk := &fakeClock{t: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)}
	s.SetClock(clk.now)
	return s, clk
}

// -----------------------------------------------------------------------------
// Constructor
// -----------------------------------------------------------------------------

func TestNewSigner_RejectsShortSecret(t *testing.T) {
	_, err := NewSigner("tooshort", time.Hour)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestNewSigner_RejectsNonPositiveTTL(t *testing.T) {
	_, err := NewSigner(testSecret, 0)
	require.Error(t, err)
	_, err = NewSigner(testSecret, -1*time.Hour)
	require.Error(t, err)
}

func TestNewSigner_AcceptsExactMinSecret(t *testing.T) {
	_, err := NewSigner(testSecret, time.Hour) // 32 bytes
	assert.NoError(t, err)
}

// -----------------------------------------------------------------------------
// Sign / Verify happy path
// -----------------------------------------------------------------------------

func TestSign_RoundTripsSubject(t *testing.T) {
	s, _ := newTestSigner(t)
	tok, exp, err := s.Sign("admin")
	require.NoError(t, err)
	require.NotEmpty(t, tok)
	assert.WithinDuration(t, time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC), exp, time.Second)

	sub, err := s.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, "admin", sub)
}

func TestSign_EmptySubjectErrors(t *testing.T) {
	s, _ := newTestSigner(t)
	_, _, err := s.Sign("")
	require.Error(t, err)
}

func TestSign_TooLongSubjectErrors(t *testing.T) {
	s, _ := newTestSigner(t)
	_, _, err := s.Sign(strings.Repeat("a", subjectMaxLength+1))
	require.Error(t, err)
}

// -----------------------------------------------------------------------------
// Verify failure modes
// -----------------------------------------------------------------------------

func TestVerify_EmptyToken(t *testing.T) {
	s, _ := newTestSigner(t)
	_, err := s.Verify("")
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestVerify_TamperedToken(t *testing.T) {
	s, _ := newTestSigner(t)
	tok, _, err := s.Sign("admin")
	require.NoError(t, err)

	// Flip one character in the middle (the payload section).
	parts := strings.Split(tok, ".")
	require.Len(t, parts, 3)
	mid := []byte(parts[1])
	mid[len(mid)/2] ^= 0x01
	tampered := parts[0] + "." + string(mid) + "." + parts[2]

	_, err = s.Verify(tampered)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidToken)
	assert.False(t, errors.Is(err, ErrExpiredToken),
		"a tampered token must surface as Invalid, not Expired")
}

func TestVerify_WrongSecret(t *testing.T) {
	s1, _ := newTestSigner(t)
	tok, _, err := s1.Sign("admin")
	require.NoError(t, err)

	s2, err := NewSigner("differentdifferentdifferentXXXXX", time.Hour) // 32 bytes
	require.NoError(t, err)
	_, err = s2.Verify(tok)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestVerify_ExpiredToken(t *testing.T) {
	s, clk := newTestSigner(t)
	tok, exp, err := s.Sign("admin")
	require.NoError(t, err)

	// Jump past the expiry.
	clk.advance(exp.Sub(clk.t) + time.Minute)
	_, err = s.Verify(tok)
	assert.ErrorIs(t, err, ErrExpiredToken)
}

func TestVerify_RejectsNoneAlgorithm(t *testing.T) {
	s, _ := newTestSigner(t)
	// Hand-build an `alg: "none"` token (header + payload, empty signature).
	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Issuer:    defaultIssuer,
		Subject:   "admin",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = s.Verify(signed)
	assert.ErrorIs(t, err, ErrInvalidToken,
		"alg=none must be rejected even with matching claims (jwt.WithValidMethods)")
}

func TestVerify_WrongIssuer(t *testing.T) {
	s, clk := newTestSigner(t)
	// Build a token with a different issuer using the same secret.
	// Use the pinned clock so the token's exp isn't accidentally past.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "someone-else",
		Subject:   "admin",
		IssuedAt:  jwt.NewNumericDate(clk.now()),
		ExpiresAt: jwt.NewNumericDate(clk.now().Add(time.Hour)),
	})
	signed, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)

	_, err = s.Verify(signed)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestVerify_MissingSubject(t *testing.T) {
	s, clk := newTestSigner(t)
	// Build a valid token with empty subject — Verify must reject.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    defaultIssuer,
		IssuedAt:  jwt.NewNumericDate(clk.now()),
		ExpiresAt: jwt.NewNumericDate(clk.now().Add(time.Hour)),
	})
	signed, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)

	_, err = s.Verify(signed)
	assert.ErrorIs(t, err, ErrInvalidToken)
}
