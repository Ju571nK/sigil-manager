package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownHash is `change-me-on-first-boot` hashed at cost 10 — the same
// value the .env.example ships with. Lets us assert Verify works without
// regenerating a hash at test time (bcrypt at cost 10 takes ~50ms per call).
const (
	knownPassword = "change-me-on-first-boot"
	knownHash     = "$2a$10$55LlvKwbVdEZOVEdFBFshuFyd1fqHDSKTTXgK5L890cxNNZxATtg2"
)

func TestVerify_AcceptsCorrectPassword(t *testing.T) {
	assert.True(t, Verify(knownPassword, knownHash))
}

func TestVerify_RejectsWrongPassword(t *testing.T) {
	assert.False(t, Verify("wrong-password", knownHash))
	assert.False(t, Verify(knownPassword+"x", knownHash))
	assert.False(t, Verify("", knownHash))
}

func TestVerify_RejectsEmptyHash(t *testing.T) {
	assert.False(t, Verify(knownPassword, ""))
	assert.False(t, Verify("", ""))
}

func TestVerify_RejectsMalformedHash(t *testing.T) {
	assert.False(t, Verify(knownPassword, "not-a-bcrypt-hash"))
	assert.False(t, Verify(knownPassword, "$2a$10$tooshort"))
}

func TestHash_RoundTripsThroughVerify(t *testing.T) {
	// Use a low-cost-friendly path: Hash uses DefaultBcryptCost (10) which
	// is OK for one round-trip per test process.
	h, err := Hash("a-fresh-password")
	require.NoError(t, err)
	require.NotEmpty(t, h)

	assert.True(t, Verify("a-fresh-password", h))
	assert.False(t, Verify("not-the-password", h))
}
