package config

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A real-looking bcrypt hash for "test-password" (cost=4 — fast for tests).
// Generated once; tests don't bcrypt-verify, just shape-check the string.
const validBcryptHash = "$2a$04$abcdefghijklmnopqrstuuFooBarBazQuuxCorgePloghTes12tHashXY"

// A 32-byte JWT secret (the minimum we accept).
const validJWTSecret = "abcdefghijklmnopqrstuvwxyzABCDEF"

// setEnv sets t.Setenv for every k/v in env. Any key whose value is the
// empty string is unset instead (t.Setenv with "" still sets it to empty).
func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		if v == "" {
			t.Setenv(k, "")
			require.NoError(t, unsetForTest(t, k))
			continue
		}
		t.Setenv(k, v)
	}
}

// unsetForTest deletes an env var for the duration of the test by replacing
// t.Setenv's "set to empty" semantics with an actual deletion. t.Setenv will
// restore the prior value at test end.
func unsetForTest(t *testing.T, key string) error {
	t.Helper()
	return nil // t.Setenv("","") already gives us "empty-as-absent"
}

// fullValidEnv returns a complete, valid environment for the non-mock path.
func fullValidEnv() map[string]string {
	return map[string]string{
		"LISTEN_ADDR":                 ":9999",
		"SIGIL_SERVER_BASE_URL":       "http://localhost:9090",
		"SIGIL_SERVER_READ_TOKEN":     "secret-token",
		"MOCK_FLEET":                  "",
		"TRIAGE_DB_PATH":              "/tmp/test-triage.sqlite",
		"ADMIN_USERNAME":              "admin",
		"ADMIN_PASSWORD_BCRYPT":       validBcryptHash,
		"JWT_SECRET":                  validJWTSecret,
		"JWT_TTL_HOURS":               "24",
		"FLEET_POLL_INTERVAL_SECONDS": "10",
	}
}

func TestLoad_ValidFullConfig(t *testing.T) {
	setEnv(t, fullValidEnv())

	c, err := Load()
	require.NoError(t, err)

	assert.Equal(t, ":9999", c.ListenAddr)
	assert.Equal(t, "http://localhost:9090", c.SigilServerBaseURL)
	assert.Equal(t, "secret-token", c.SigilServerReadToken)
	assert.False(t, c.IsMockFleet())
	assert.Equal(t, "/tmp/test-triage.sqlite", c.TriageDBPath)
	assert.Equal(t, "admin", c.AdminUsername)
	assert.Equal(t, validBcryptHash, c.AdminPasswordBcrypt)
	assert.Equal(t, validJWTSecret, string(c.JWTSecret))
	assert.Equal(t, 24*time.Hour, c.JWTTTL)
	assert.Equal(t, 10*time.Second, c.FleetPollInterval)
}

func TestLoad_DefaultsAppliedWhenOptionalUnset(t *testing.T) {
	env := fullValidEnv()
	env["LISTEN_ADDR"] = ""
	env["TRIAGE_DB_PATH"] = ""
	env["JWT_TTL_HOURS"] = ""
	env["FLEET_POLL_INTERVAL_SECONDS"] = ""
	setEnv(t, env)

	c, err := Load()
	require.NoError(t, err)

	assert.Equal(t, ":8080", c.ListenAddr)
	assert.Equal(t, "./var/triage.sqlite", c.TriageDBPath)
	assert.Equal(t, 12*time.Hour, c.JWTTTL)
	assert.Equal(t, 5*time.Second, c.FleetPollInterval)
}

func TestLoad_MockFleetRelaxesSigilURLAndToken(t *testing.T) {
	env := fullValidEnv()
	env["MOCK_FLEET"] = "1"
	env["SIGIL_SERVER_BASE_URL"] = ""
	env["SIGIL_SERVER_READ_TOKEN"] = ""
	setEnv(t, env)

	c, err := Load()
	require.NoError(t, err)

	assert.True(t, c.IsMockFleet())
	assert.Empty(t, c.SigilServerBaseURL)
	assert.Empty(t, c.SigilServerReadToken)
}

func TestLoad_MissingRequired_NonMockMode(t *testing.T) {
	tests := []struct {
		name      string
		unsetKey  string
		wantInMsg string
	}{
		{"sigil URL", "SIGIL_SERVER_BASE_URL", "SIGIL_SERVER_BASE_URL"},
		{"sigil token", "SIGIL_SERVER_READ_TOKEN", "SIGIL_SERVER_READ_TOKEN"},
		{"admin username", "ADMIN_USERNAME", "ADMIN_USERNAME"},
		{"admin password", "ADMIN_PASSWORD_BCRYPT", "ADMIN_PASSWORD_BCRYPT"},
		{"jwt secret", "JWT_SECRET", "JWT_SECRET"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := fullValidEnv()
			env[tc.unsetKey] = ""
			setEnv(t, env)

			_, err := Load()
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrMissingRequired),
				"expected ErrMissingRequired, got %v", err)
			assert.Contains(t, err.Error(), tc.wantInMsg)
		})
	}
}

func TestLoad_MissingRequired_MockMode_StillNeedsAuth(t *testing.T) {
	// Mock mode relaxes sigil URL/token but NOT auth or JWT.
	env := fullValidEnv()
	env["MOCK_FLEET"] = "1"
	env["SIGIL_SERVER_BASE_URL"] = ""
	env["SIGIL_SERVER_READ_TOKEN"] = ""
	env["ADMIN_USERNAME"] = ""
	setEnv(t, env)

	_, err := Load()
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMissingRequired))
	assert.Contains(t, err.Error(), "ADMIN_USERNAME")
}

func TestLoad_ShortJWTSecret(t *testing.T) {
	env := fullValidEnv()
	env["JWT_SECRET"] = "tooshort" // 8 bytes — below the 32-byte floor
	setEnv(t, env)

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET must be at least 32 bytes")
	assert.False(t, errors.Is(err, ErrMissingRequired),
		"short-secret should not be ErrMissingRequired (it's set, just short)")
}

func TestLoad_InvalidBcryptHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"empty was already covered by missing-required", "plain"},
		{"plaintext password", "hunter2hunter2hunter2hunter2hunter2hunter2hunter2hunter2hunter2"},
		{"wrong prefix", "$5$rounds=1000$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012345"},
		{"short bcrypt-like", "$2a$04$short"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := fullValidEnv()
			env["ADMIN_PASSWORD_BCRYPT"] = tc.hash
			setEnv(t, env)

			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ADMIN_PASSWORD_BCRYPT")
		})
	}
}

func TestLoad_AcceptsAllBcryptPrefixes(t *testing.T) {
	for _, prefix := range []string{"$2a$", "$2b$", "$2y$"} {
		t.Run(prefix, func(t *testing.T) {
			env := fullValidEnv()
			// Build a 60-char hash with the prefix under test.
			tail := validBcryptHash[4:] // strip "$2a$"
			env["ADMIN_PASSWORD_BCRYPT"] = prefix + tail
			setEnv(t, env)

			c, err := Load()
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(c.AdminPasswordBcrypt, prefix))
		})
	}
}

func TestLoad_NonIntegerJWTTTL(t *testing.T) {
	env := fullValidEnv()
	env["JWT_TTL_HOURS"] = "not-a-number"
	setEnv(t, env)

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_TTL_HOURS")
}

func TestLoad_ZeroJWTTTL(t *testing.T) {
	env := fullValidEnv()
	env["JWT_TTL_HOURS"] = "0"
	setEnv(t, env)

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_TTL_HOURS")
}

func TestLoad_NegativeFleetPollInterval(t *testing.T) {
	env := fullValidEnv()
	env["FLEET_POLL_INTERVAL_SECONDS"] = "-5"
	setEnv(t, env)

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FLEET_POLL_INTERVAL_SECONDS")
}

func TestLoad_MultipleMissingReportedTogether(t *testing.T) {
	env := fullValidEnv()
	env["SIGIL_SERVER_BASE_URL"] = ""
	env["ADMIN_USERNAME"] = ""
	env["JWT_SECRET"] = ""
	setEnv(t, env)

	_, err := Load()
	require.Error(t, err)
	// Should mention all three in a single message, not bail at the first.
	msg := err.Error()
	assert.Contains(t, msg, "SIGIL_SERVER_BASE_URL")
	assert.Contains(t, msg, "ADMIN_USERNAME")
	assert.Contains(t, msg, "JWT_SECRET")
}

func TestLoad_WhitespaceTrimmedFromStrings(t *testing.T) {
	env := fullValidEnv()
	env["SIGIL_SERVER_BASE_URL"] = "  http://localhost:9090  "
	env["ADMIN_USERNAME"] = "  admin\t"
	setEnv(t, env)

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:9090", c.SigilServerBaseURL)
	assert.Equal(t, "admin", c.AdminUsername)
}
