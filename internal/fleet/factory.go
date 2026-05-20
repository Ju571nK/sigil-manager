package fleet

import (
	"time"

	"github.com/Ju571nK/sigil-manager/internal/config"
)

// NewFromConfig returns the [Client] selected by the operator's config.
// MOCK_FLEET=1 yields [MockClient]; otherwise an [HTTPClient] pointed at
// SIGIL_SERVER_BASE_URL with the SIGIL_SERVER_READ_TOKEN bearer.
func NewFromConfig(cfg *config.Config) Client {
	if cfg.IsMockFleet() {
		return NewMock(time.Time{})
	}
	return NewHTTPClient(cfg.SigilServerBaseURL, cfg.SigilServerReadToken, 0)
}
