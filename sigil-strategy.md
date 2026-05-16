# Sigil Productization Strategy: Open Core + SaaS

**Status:** Strategic decision recorded 2026-05-15
**Owner:** Sigil project (Justin Kwon)
**Applies to:** Future repos `sigil-manager` and `sigil-cloud`

This document persists outside any individual repo so sessions started in
`sigil-manager` and `sigil-cloud` (which will live in their own repos with their
own `CLAUDE.md` / memory) can reference the same strategy decisions.

## Three-tier structure

| Tier | Repo | License | Visibility | Purpose |
|------|------|---------|------------|---------|
| OSS daemon | `Ju571nK/sigil` | Apache-2.0 | Public | Per-host AI-SPM daemon, agents, binaries (current repo) |
| Self-hostable manager | `Ju571nK/sigil-manager` | Apache-2.0 *or* BSL/source-available | Public | Web console for fleet visibility — reads fleet data from `sigil-server`; owns its own triage state (ack/assign/resolve/notes) |
| Hosted SaaS | `Ju571nK/sigil-cloud` | **Commercial** | **Private** | Multi-tenant hosted offering with billing, SSO, compliance |

## Why SaaS must be a separate, private repo

1. **License isolation.** Commercial code cannot accidentally be consumed under
   Apache-2.0. If billing logic lives in the same repo as Apache-2.0 code,
   downstream "is this Apache?" questions become arguable.
2. **Secret leakage risk.** SaaS pulls together database creds, Stripe keys,
   JWT signing material, OAuth client secrets, internal admin endpoints. None of
   that should ever exist in a public repo's history (even gitignored —
   accidents happen, and git history is forever).
3. **IP / competitive moat.** Multi-tenant routing, tenant isolation, billing
   and metering, compliance evidence pipeline. These ARE the differentiation.
   Open-sourcing them invites a fork that costs you the business.
4. **Customer trust signal.** Enterprise procurement expects "Yes, our SaaS
   code is in a private repo with audited access." It's a checkbox in vendor
   security questionnaires.
5. **Release cadence and team permissions.** SaaS ships multiple times a day,
   often hotfixes. OSS ships on a monthly+ cadence with broader review. Different
   CI gates, different reviewer pools, different deploy keys.

## Open-core precedents

| Company | OSS repo (license) | Commercial offering |
|---------|-------------------|---------------------|
| Grafana Labs | `grafana/grafana` (AGPL) | Grafana Cloud (grafana.com) |
| HashiCorp | `hashicorp/terraform`, `vault` (BSL since 2023, ex-MPL) | Terraform Cloud, HCP Vault |
| Sentry | `getsentry/sentry` (BSL/Apache mix) | sentry.io |
| PostHog | `PostHog/posthog` (MIT) | posthog.com Cloud |

All four ship the OSS as install-it-yourself; the SaaS is private/hosted. None
mix billing logic with the OSS repo.

## What `sigil-manager` is (the open part)

A self-hostable web console that users can `docker run` next to their own
`sigil-server`. Reads fleet evidence from `sigil-server` (hosts, events,
policy compliance, AI Guard risk) and renders dashboards — "AI Guard risk by
host", "events over time", "policy compliance per host". It does **not** mutate
`sigil-server`'s fleet data. The manager owns a small amount of its own state
for SOC analyst workflow — alert triage status (open / acknowledged / resolved),
assignee, and analyst notes — stored in its own local DB. No billing, no
multi-tenancy, no auth beyond simple username/password or single SSO.

This repo can be public Apache-2.0 (or BSL/source-available if there's a real
risk of competitors selling a hosted version of it). The decision is between
"maximally permissive" (drives adoption) and "protects the SaaS economics"
(slows hosted forks).

## What `sigil-cloud` is (the closed part)

The hosted, multi-tenant version of `sigil-manager` plus the commercial
infrastructure that turns it into a product:

- Customer accounts + SSO (OAuth providers; SAML for enterprise tier)
- Multi-tenant database with strict tenant isolation (row-level security or
  per-tenant schemas)
- Payment provider: Stripe (US/EU), Lemon Squeezy / Paddle (Merchant of Record
  for VAT/tax simplification)
- Usage metering (hosts monitored, events ingested, retention days)
- Invoicing, dunning, refunds workflow
- API tokens with rotation, scoping, revocation
- Admin / CS panel (support agent view, impersonation with audit log)
- Compliance: SOC 2 Type II, GDPR DPA template, optional data residency
- Per-tenant quotas, rate limits, abuse detection
- Status page + incident comms

## Recommended sequence

1. **Phase 3b.1** — AI Guard Risk Index local emission ships in `sigil`
   (current sprint, issue #9 sub-scope)
2. **Phase 3b.4** — Fleet aggregation API on `sigil-server` ships in `sigil`
   (defines the data shape the SaaS will consume)
3. **New repo: `Ju571nK/sigil-manager`** — Apache-2.0 self-hostable web console
   reading from `sigil-server`. Brainstorm in its own session.
4. **`sigil-manager` matures** — Real fleet deployments validate the data model
   and the UX patterns.
5. **New repo: `Ju571nK/sigil-cloud`** — Private, multi-tenant hosted version.
   Extracts the manager UI, adds billing / SSO / multi-tenancy / compliance.
   Brainstorm in its own session.

## Cross-references

- This decision was reached during the Phase 3b.1 brainstorm session 2026-05-15.
- Phase 3b.1 spec: `Ju571nK/sigil:docs/superpowers/specs/2026-05-16-phase-3b1-ai-guard-risk-index-design.md`
  (gitignored, local-only on dev machine).
- Sigil Roadmap project board (#3) tracks epics 3b.1 through 3b.5.
- README.md in `Ju571nK/sigil` has the AI-SPM positioning that this productization
  strategy assumes.
