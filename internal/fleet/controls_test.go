package fleet

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestControlsHTTPAndCacheRoundTrip(t *testing.T) {
	for _, tc := range []struct{ name, field, want string }{
		{"legacy", "", ""},
		{"null", `,"controls":null`, ""},
		{"empty", `,"controls":[]`, `[]`},
		{"reported", `,"controls":[{"id":"future.unknown","source_path":"/etc/policy","setting":"restricted","value":false},{"id":"codex.configured.allowed_sandbox_modes","source_path":"/etc/codex/requirements.toml","setting":"allowed_sandbox_modes","value":["read-only"]},{"id":"future.object","source_path":"/etc/policy","setting":"future","value":{"nested":0}}]`, `[{"id":"future.unknown","source_path":"/etc/policy","setting":"restricted","value":false},{"id":"codex.configured.allowed_sandbox_modes","source_path":"/etc/codex/requirements.toml","setting":"allowed_sandbox_modes","value":["read-only"]},{"id":"future.object","source_path":"/etc/policy","setting":"future","value":{"nested":0}}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c, _ := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
				calls++
				_, _ = w.Write([]byte(`{"host_id":"host","ai_guard":{"by_tool":{"codex":{"score":0,"bucket":"low","assessed_ts":"2026-09-13T00:00:00Z","scope":{"kind":"user_global"},"reasons":[]` + tc.field + `}}}}`))
			})
			cached := NewCachingClient(c, CacheConfig{TTL: time.Minute})
			for range 2 {
				host, err := cached.FleetHostByID(context.Background(), "host")
				require.NoError(t, err)
				wire, err := json.Marshal(host)
				require.NoError(t, err)
				var result struct {
					AiGuard struct {
						ByTool map[string]map[string]json.RawMessage `json:"by_tool"`
					} `json:"ai_guard"`
				}
				require.NoError(t, json.Unmarshal(wire, &result))
				got := result.AiGuard.ByTool["codex"]["controls"]
				if tc.want == "" {
					require.Empty(t, got)
				} else {
					require.JSONEq(t, tc.want, string(got))
				}
			}
			require.Equal(t, 1, calls, "second serialization uses cached response")
			var evidence Evidence
			require.NoError(t, json.Unmarshal([]byte(`{"kind":"ai_guard_risk_assessed"`+tc.field+`}`), &evidence))
			decoded, err := evidence.AsAiGuard()
			require.NoError(t, err)
			if tc.want == "" {
				require.Nil(t, decoded.Controls)
			} else {
				wire, err := json.Marshal(decoded.Controls)
				require.NoError(t, err)
				require.JSONEq(t, tc.want, string(wire))
			}
			wire, err := json.Marshal(evidence)
			require.NoError(t, err)
			require.JSONEq(t, `{"kind":"ai_guard_risk_assessed"`+tc.field+`}`, string(wire))
		})
	}
}
