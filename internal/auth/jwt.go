package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT signing constants. HS256 is the only algorithm sigil-manager
// accepts — `none` and asymmetric algorithms are rejected by Verify.
const (
	signingMethod    = "HS256"
	minSecretBytes   = 32
	defaultIssuer    = "sigil-manager"
	subjectMaxLength = 256
)

// Errors returned by [Signer.Verify].
var (
	// ErrInvalidToken covers every Verify failure that isn't expiry —
	// malformed token, wrong signature, wrong algorithm, bad claims.
	// Callers should NOT distinguish further to avoid leaking which
	// part of the token an attacker corrupted.
	ErrInvalidToken = errors.New("auth: invalid token")

	// ErrExpiredToken means the JWT parsed and verified but `exp` is past.
	// Separate from ErrInvalidToken so the UI can show "session expired —
	// please log in again" instead of a generic 401.
	ErrExpiredToken = errors.New("auth: token expired")
)

// Signer issues and verifies HS256-signed JWTs for the single-admin
// session. Construct via [NewSigner]; the secret must be at least 32 bytes
// (`config.Config.JWTSecret` enforces this on env load).
//
// Safe for concurrent use.
type Signer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time // injected for tests
}

// NewSigner builds a Signer. Returns an error if secret is < 32 bytes or
// ttl <= 0; the caller (main) has already validated both via config.Load
// so this is defense in depth.
func NewSigner(secret string, ttl time.Duration) (*Signer, error) {
	if len(secret) < minSecretBytes {
		return nil, fmt.Errorf("auth: jwt secret must be >= %d bytes (got %d)", minSecretBytes, len(secret))
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("auth: jwt ttl must be positive (got %v)", ttl)
	}
	return &Signer{
		secret: []byte(secret),
		ttl:    ttl,
		now:    func() time.Time { return time.Now().UTC() },
	}, nil
}

// SetClock overrides the time source. Tests use this to advance past
// expiry without sleeping; production never calls it.
func (s *Signer) SetClock(now func() time.Time) { s.now = now }

// TTL exposes the configured session lifetime so handlers can set the
// cookie's Max-Age to match.
func (s *Signer) TTL() time.Duration { return s.ttl }

// Sign issues a JWT for `subject` (the admin username). Returns the
// compact-serialized token, the absolute expiry, and any sign error.
func (s *Signer) Sign(subject string) (string, time.Time, error) {
	if subject == "" {
		return "", time.Time{}, fmt.Errorf("auth: subject required")
	}
	if len(subject) > subjectMaxLength {
		return "", time.Time{}, fmt.Errorf("auth: subject too long (max %d)", subjectMaxLength)
	}
	now := s.now()
	exp := now.Add(s.ttl)
	claims := jwt.RegisteredClaims{
		Issuer:    defaultIssuer,
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign: %w", err)
	}
	return signed, exp, nil
}

// Verify parses and validates a token. Returns the subject (admin
// username) on success, [ErrExpiredToken] when the only failure is past
// `exp`, and [ErrInvalidToken] for everything else.
//
// Enforces:
//   - alg == HS256 (rejects `none`, RS256, etc.)
//   - signature matches our secret
//   - exp / nbf / iat respect the injected clock
//   - iss == "sigil-manager"
//   - subject is non-empty
func (s *Signer) Verify(token string) (string, error) {
	sub, _, err := s.VerifyWithExp(token)
	return sub, err
}

// VerifyWithExp is [Signer.Verify] plus the parsed `exp` claim. Handlers
// that need to show the session expiry to the user (e.g. /auth/me) call
// this; middleware that just needs to gate use [Signer.Verify].
func (s *Signer) VerifyWithExp(token string) (string, time.Time, error) {
	if token == "" {
		return "", time.Time{}, ErrInvalidToken
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{signingMethod}),
		jwt.WithIssuer(defaultIssuer),
		jwt.WithTimeFunc(s.now),
	)
	var claims jwt.RegisteredClaims
	parsed, err := parser.ParseWithClaims(token, &claims, func(_ *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", time.Time{}, ErrExpiredToken
		}
		return "", time.Time{}, ErrInvalidToken
	}
	if !parsed.Valid {
		return "", time.Time{}, ErrInvalidToken
	}
	if claims.Subject == "" {
		return "", time.Time{}, ErrInvalidToken
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	}
	return claims.Subject, exp, nil
}
