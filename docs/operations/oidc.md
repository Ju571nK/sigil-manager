# Organization login with a standard OIDC provider

The manager accepts one configured OIDC provider per installation. Discovery and
standard claims determine behavior; there is no vendor allowlist. Local admin
login remains available for recovery. All admitted users have the same console
permissions. This does not add directory synchronization, device ownership,
enterprise federation, SAML, SCIM, roles or user management.

## Configuration

Register a confidential web application with Authorization Code flow, PKCE S256
and query-mode callbacks. Register this exact callback at the provider:
`https://<manager-host>/api/v1/auth/oidc/callback`.

Set all five variables on the Go server:

| Variable | Value |
| --- | --- |
| `OIDC_ISSUER_URL` | Exact HTTPS issuer, discoverable via `.well-known/openid-configuration` |
| `OIDC_CLIENT_ID` | Application client ID |
| `OIDC_CLIENT_SECRET` | Secret supplied through server environment/secret storage |
| `OIDC_REDIRECT_URL` | Exact public HTTPS callback above |
| `OIDC_ALLOWED_SUBJECTS` | JSON string array of explicitly admitted ID-token `sub` values |

Leave all five unset to disable organization login. Partial configuration,
empty admission lists and invalid URLs stop startup. Local `ADMIN_USERNAME`,
`ADMIN_PASSWORD_BCRYPT` and `JWT_SECRET` remain required. Keep local recovery
credentials separate from organizational credentials. Leave
`SIGIL_INSECURE_COOKIE` unset in production.

Only `openid` scope is requested. No email/group claims or proprietary API are
required. Obtain the application's actual subject values from the provider's
administrative tooling or a controlled test application: subjects can be
application-specific and are not necessarily user object IDs. Application
assignment at the provider is recommended, but does not replace the manager's
explicit subject allowlist. Never paste production tokens into external tools.

Login redirects to the configured provider. On success, the manager checks token
signature, issuer, audience, expiry, nonce and authorized party, then checks the
subject allowlist. The session identity is `oidc:` plus a SHA-256 digest of the
JSON pair `[issuer, subject]`; it does not change when email/display name changes.
This identifier is also the author recorded on new triage actions. Provider access
and ID tokens are never returned to the browser or retained for refresh.

## Compatibility and vendor examples

| Provider category | Configuration approach | Validation status |
| --- | --- | --- |
| Standard discovery-based OIDC | Issuer, confidential client, S256, `sub` admission | Local TLS protocol fixture tested |
| Okta | Application's exact issuer and registered web callback | Live vendor validation pending |
| Microsoft Entra ID | Tenant-specific issuer; do not use `common`/`organizations` templates | Live vendor validation pending |
| LDAP / on-premises AD | Operator-managed OIDC broker; manager connects to the broker | Live broker/directory validation pending |
| Other OIDC providers | Same settings; no new vendor branch required if the contract is met | Validate each deployment before claiming support |

A provider must support discovery and the code flow above. Off-spec discovery,
non-HTTPS endpoints and implicit/form-post flows are not implemented. Vendor
names are examples, not a guarantee of compatibility. LDAP is not an OIDC
provider; a broker remains independently deployed infrastructure.

References: [OIDC library verification](https://pkg.go.dev/github.com/coreos/go-oidc/v3/oidc),
[Okta web application configuration](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm),
[Entra OIDC](https://learn.microsoft.com/en-us/entra/identity-platform/v2-protocols-oidc),
[Keycloak federation](https://www.keycloak.org/docs/latest/server_admin/#_user-storage-federation).

## Session, deployment and recovery behavior

- State, browser binding, nonce and PKCE verifier are generated for each login.
  Transactions expire after five minutes, are consumed once, and are capped at
  1,024 outstanding requests. Restarting invalidates pending logins. Starting a
  second login in the same browser replaces its binding cookie; retry the latest
  flow if the older tab fails. Replicas require sticky routing for the flow.
- A session lasts at most `JWT_TTL_HOURS` and never beyond the ID token's expiry.
  There is no refresh token. Logout clears the manager cookie only; it does not
  end the provider's session or revoke a copied JWT. Provider logout/back-channel
  logout and live revocation are not implemented. A provider session may allow
  immediate sign-in again.
- Removing a subject from the configured allowlist takes effect on new logins
  after restart. Existing sessions remain valid until expiry. Rotate `JWT_SECRET`
  and restart all replicas to invalidate **all** manager sessions immediately.
  For deployments needing shorter access-removal latency, reduce `JWT_TTL_HOURS`.
- Discovery runs on startup with bounded network requests. If it fails, startup
  fails closed. To recover, disable all OIDC variables and restart, then use the
  local administrator. During a later provider outage, local login still works;
  external login may fail until the provider recovers.
- Application access logs omit query strings. Configure the reverse proxy and
  external request tracing to redact callback queries too: they contain codes
  and state. Never log client secrets or token responses.
- Login failures show a generic retry/contact-admin message. Provider errors and
  claims are not reflected into the page. Successful OIDC login opens `/alerts`;
  a pre-login deep link is not restored by this first implementation.

## Verification performed

Local TLS discovery → authorization → token exchange → JWKS verification tests
exercise PKCE, browser binding, callback replay, nonce, issuer/audience/authorized
party checks, expired or incorrectly signed tokens, subject denial and local
session expiry. HTTP route tests exercise secure cookies and `/auth/me` after
successful login. Browser tests verify the optional organization action and
failure UI while retaining local login. No live provider credentials are used.
