package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPClient implements [Client] against a real sigil-server `/v1/*` API
// (contract v1.0). Construct with [NewHTTPClient]; safe for concurrent use.
type HTTPClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewHTTPClient builds an HTTPClient pointing at sigil-server. baseURL must
// have no trailing slash (e.g. "http://localhost:9090"). token is the shared
// bearer per contract F2 / §3.1. timeout=0 falls back to 5s.
func NewHTTPClient(baseURL, token string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HTTPClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// -----------------------------------------------------------------------------
// Endpoint implementations
// -----------------------------------------------------------------------------

// Healthz implements [Client.Healthz]. No auth header (contract §3 / §5.1).
func (c *HTTPClient) Healthz(ctx context.Context) (*Healthz, error) {
	return doJSON[Healthz](ctx, c, http.MethodGet, "/v1/healthz", nil, false)
}

// Meta implements [Client.Meta].
func (c *HTTPClient) Meta(ctx context.Context) (*Meta, error) {
	return doJSON[Meta](ctx, c, http.MethodGet, "/v1/meta", nil, true)
}

// PolicyMeta implements [Client.PolicyMeta].
func (c *HTTPClient) PolicyMeta(ctx context.Context) (*PolicyMeta, error) {
	return doJSON[PolicyMeta](ctx, c, http.MethodGet, "/v1/policy/meta", nil, true)
}

// Events implements [Client.Events].
func (c *HTTPClient) Events(ctx context.Context, p EventsParams) (*EventsPage, error) {
	return doJSON[EventsPage](ctx, c, http.MethodGet, "/v1/events", buildEventsQuery(p), true)
}

// EventByID implements [Client.EventByID]. 404 here is translated from
// [ErrReadAPIDisabled] to [ErrNotFound] because id-lookup endpoints can be
// reached only when the read API is exposed.
func (c *HTTPClient) EventByID(ctx context.Context, eventID string) (*Event, error) {
	out, err := doJSON[Event](ctx, c, http.MethodGet, "/v1/events/"+url.PathEscape(eventID), nil, true)
	if errors.Is(err, ErrReadAPIDisabled) {
		return nil, ErrNotFound
	}
	return out, err
}

// FleetHosts implements [Client.FleetHosts].
func (c *HTTPClient) FleetHosts(ctx context.Context, p HostsParams) (*HostsPage, error) {
	return doJSON[HostsPage](ctx, c, http.MethodGet, "/v1/fleet/hosts", buildHostsQuery(p), true)
}

// FleetHostByID implements [Client.FleetHostByID]. See [HTTPClient.EventByID]
// note about the 404 → [ErrNotFound] translation.
func (c *HTTPClient) FleetHostByID(ctx context.Context, hostID string) (*HostDetail, error) {
	out, err := doJSON[HostDetail](ctx, c, http.MethodGet, "/v1/fleet/hosts/"+url.PathEscape(hostID), nil, true)
	if errors.Is(err, ErrReadAPIDisabled) {
		return nil, ErrNotFound
	}
	return out, err
}

// FleetRisk implements [Client.FleetRisk].
func (c *HTTPClient) FleetRisk(ctx context.Context, p RiskParams) (*RiskPage, error) {
	return doJSON[RiskPage](ctx, c, http.MethodGet, "/v1/fleet/risk", buildRiskQuery(p), true)
}

// FleetCompliance implements [Client.FleetCompliance].
func (c *HTTPClient) FleetCompliance(ctx context.Context, p ComplianceParams) (*CompliancePage, error) {
	return doJSON[CompliancePage](ctx, c, http.MethodGet, "/v1/fleet/compliance", buildComplianceQuery(p), true)
}

// -----------------------------------------------------------------------------
// Core request helper
// -----------------------------------------------------------------------------

// doJSON issues a request and decodes the JSON response into T. It maps:
//   - 401              → [ErrUnauthorized]
//   - 404 (non-id)     → [ErrReadAPIDisabled] (callers translate to [ErrNotFound] for id lookups)
//   - 503              → *[ServiceUnavailableError] (wraps [ErrServiceUnavailable])
//   - other non-2xx    → *[APIError]
//   - decode failure   → wrapped error
//
// auth=false skips the Authorization header (only /v1/healthz).
func doJSON[T any](ctx context.Context, c *HTTPClient, method, path string, query url.Values, auth bool) (*T, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, fmt.Errorf("fleet: build request: %w", err)
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fleet: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		var out T
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, fmt.Errorf("fleet: decode %s: %w", path, err)
		}
		return &out, nil
	case http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case http.StatusNotFound:
		return nil, ErrReadAPIDisabled
	case http.StatusServiceUnavailable:
		return nil, &ServiceUnavailableError{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	default:
		return nil, parseAPIError(resp)
	}
}

// parseRetryAfter parses the `Retry-After` header in its delta-seconds form
// (contract §6.1 F15 + RFC 7231 §7.1.3). Falls back to 5s when the header is
// missing or unparseable — matches sigil-server's documented default.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 5 * time.Second
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		return 5 * time.Second
	}
	return time.Duration(n) * time.Second
}

// parseAPIError reads the response body and decodes the contract §6.1 error
// shape `{"error": {...}}`. Falls back to a synthetic APIError when the body
// is missing or malformed.
func parseAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var wire struct {
		Error APIError `json:"error"`
	}
	if jerr := json.Unmarshal(body, &wire); jerr == nil && wire.Error.Code != "" {
		wire.Error.Status = resp.StatusCode
		return &wire.Error
	}
	return &APIError{
		Status:  resp.StatusCode,
		Code:    "unknown",
		Message: strings.TrimSpace(string(body)),
	}
}

// -----------------------------------------------------------------------------
// Query builders
// -----------------------------------------------------------------------------

func buildEventsQuery(p EventsParams) url.Values {
	q := url.Values{}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	for _, h := range p.HostIDs {
		if h != "" {
			q.Add("host_id", h)
		}
	}
	if !p.Since.IsZero() {
		q.Set("since", p.Since.UTC().Format(time.RFC3339))
	}
	if !p.Until.IsZero() {
		q.Set("until", p.Until.UTC().Format(time.RFC3339))
	}
	if len(p.EvidenceKinds) > 0 {
		q.Set("evidence_kind", strings.Join(p.EvidenceKinds, ","))
	}
	if len(p.Severity) > 0 {
		q.Set("severity", strings.Join(p.Severity, ","))
	}
	if len(p.Source) > 0 {
		q.Set("source", strings.Join(p.Source, ","))
	}
	if p.MinAiGuardBucket != "" {
		q.Set("min_ai_guard_bucket", p.MinAiGuardBucket)
	}
	return q
}

func buildHostsQuery(p HostsParams) url.Values {
	q := url.Values{}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if len(p.Status) > 0 {
		q.Set("status", strings.Join(p.Status, ","))
	}
	if len(p.Bucket) > 0 {
		q.Set("bucket", strings.Join(p.Bucket, ","))
	}
	if p.Sort != "" {
		q.Set("sort", p.Sort)
	}
	return q
}

func buildRiskQuery(p RiskParams) url.Values {
	q := url.Values{}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if len(p.Tool) > 0 {
		q.Set("tool", strings.Join(p.Tool, ","))
	}
	if p.MinBucket != "" {
		q.Set("min_bucket", p.MinBucket)
	}
	return q
}

func buildComplianceQuery(p ComplianceParams) url.Values {
	q := url.Values{}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	if p.Limit > 0 {
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	return q
}
