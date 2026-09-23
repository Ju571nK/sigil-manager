# Devices and internal deployment integration

## Implemented scope

Fleet → Devices consumes the existing fleet hosts list (§5.3), including hosts
without risk assessments. It displays the immutable Sigil host ID, reported
hostname/version, connection status, last seen and nullable risk, with host detail
links, server-side status filtering and cursor pagination. Search covers loaded
hostnames, host IDs and versions only. The optional target version is a temporary
operator-supplied exact comparison, ignoring a leading `v`; differences can mean
older, newer, prerelease or custom builds, not necessarily upgrade requirements.
No automatic release lookup or update action is performed.

This is the server's observed host index, not an authoritative corporate asset
inventory. A healthy connection does not prove policy compliance, directory
membership, successful shutdown drain or watcher recovery. Versions are last
reported and may be stale. No producer wire change is needed for this view.

## Corporate identity requirements under consideration

The requested Okta, Microsoft and LDAP support has two independent purposes:
console operator authentication, and enrichment of devices with owners/departments.
The usage clarification is pending; neither integration is implemented by this
change. Microsoft is provisionally interpreted as Entra ID; on-premises AD and
Intune inventory require separate contracts.

### Operator authentication proposal

Keep the manager an OIDC relying party configured with **one provider per
installation**, within the repository's single basic SSO scope. Okta and Entra ID
both support OIDC web applications. An LDAP/AD deployment could expose OIDC via an
operator-managed broker such as Keycloak; the manager would not store directory
bind credentials or implement LDAP federation itself.

Before implementation, specify issuer/client/redirect configuration, Authorization
Code + PKCE, state/nonce validation, issuer/audience/signature/expiry validation,
server-side secret storage, secure session cookies, logout and explicit operator
admission. Authentication alone must not authorize every directory user as a
console administrator. Provider application assignment plus an explicit admission
rule must be tested. Define recovery access and session revocation semantics.
Login identity uses issuer + subject, never email as a durable key.

SAML, simultaneous identity federation, enterprise RBAC and organizational account
hierarchies are outside this proposal and the current public manager scope.
Existing local admin authentication is unchanged.

Primary references (checked 2026-09-23):
- [Okta OIDC app integrations](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm)
- [Microsoft Entra OIDC](https://learn.microsoft.com/en-us/entra/identity-platform/v2-protocols-oidc)
- [Keycloak user federation](https://www.keycloak.org/docs/latest/server_admin/#_user-storage-federation)

### Device identity and ownership proposal

SSO login does not establish who owns a device. A hostname, email or local OS user
must not automatically bind an observed Sigil host to a directory identity.
Any future producer-owned enrichment contract should include:

- Stable Sigil host ID and an explicit binding to a provider namespace and immutable
  directory device/user ID; support unbound, ambiguous and retired devices.
- Owner/department as optional display attributes, with provider provenance,
  last synchronization timestamp and unavailable/stale/error states.
- Defined shared-device and reassignment behavior. Missing ownership must remain
  unknown; identity-provider groups are not automatically Sigil policy groups.
- Server-side connector credentials and minimal directory access. The browser
  receives only approved display data, never Okta/Graph/LDAP credentials.

No ownership, department, directory ID or synchronization state exists in the
current FleetHosts contract. Do not fabricate those fields or create a separate
manager authority for them. Discuss producer additions through existing
[policy management design #33](https://github.com/Ju571nK/sigil-manager/issues/33)
and [MDM follow-up #31](https://github.com/Ju571nK/sigil-manager/issues/31)
([producer #211](https://github.com/Ju571nK/sigil/issues/211)); these are related
work, not evidence that directory enrichment already exists. Obtain a concrete
read contract and fixtures before implementing owner/department filters.

## Validation boundary

Tests exercise authenticated host-list routing, status filtering and pagination;
browser fixtures exercise unassessed hosts, deduplication, version comparison,
loaded-only search, server-filter cursor reset, upstream error/retry and drilldown.
No live Okta, Entra, LDAP, or v0.8.3 daemon integration is claimed.
