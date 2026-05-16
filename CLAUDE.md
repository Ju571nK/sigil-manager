# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repo status

This repo is **pre-code**. As of 2026-05-16 the only file is `sigil-strategy.md`,
which records the productization strategy. No implementation, build, test, or
lint commands exist yet. Do not invent them — when the user is ready to scaffold
the stack, brainstorm the choice with them rather than assuming.

`sigil-strategy.md` is the source of truth for scope decisions in this repo.
Read it before any architectural suggestion.

## What this repo IS

`sigil-manager` is the **self-hostable, open** web console for the Sigil
AI-SPM project. It is the middle tier of a three-tier structure:

1. `Ju571nK/sigil` — OSS per-host daemon (Apache-2.0, public, already exists)
2. **`Ju571nK/sigil-manager` — this repo.** Self-hostable web console for fleet
   visibility. Apache-2.0 (or BSL/source-available — decision pending). Public.
3. `Ju571nK/sigil-cloud` — Multi-tenant hosted SaaS. **Commercial, private.**
   Does not yet exist.

The manager is a console a user runs next to their own `sigil-server` via
`docker run`. It reads fleet evidence from `sigil-server` and renders dashboards
("AI Guard risk by host", "events over time", "policy compliance per host").
It is **read-only** against `sigil-server`.

## What this repo MUST NOT contain

These belong in `sigil-cloud` (the private commercial repo), not here. Pushing
them into this repo creates license-isolation problems and leaks IP that is
meant to be the SaaS moat:

- Billing, payments, Stripe/Lemon Squeezy/Paddle integration
- Multi-tenancy, tenant routing, row-level security for tenants, per-tenant schemas
- Usage metering, invoicing, dunning, refunds
- SSO beyond a single basic provider (no SAML, no enterprise identity federation here)
- Admin / CS panels with impersonation
- Compliance evidence pipeline (SOC 2, GDPR DPA), per-tenant quotas, abuse detection
- Customer accounts with org/team hierarchies, API token rotation/scoping/revocation

Auth in this repo should be **at most** simple username/password or a single
basic SSO. Anything more sophisticated is a signal you're building `sigil-cloud`
features in the wrong repo — stop and confirm with the user.

## Cross-repo dependencies

This repo consumes data from the **fleet aggregation API on `sigil-server`**
(Phase 3b.4 in the parent `sigil` repo). The data shape produced by that API
defines what this console can render. Before designing UI that depends on
fleet data, check the actual `sigil-server` API surface — don't assume it
from the dashboards' names.

Related context lives in:
- `Ju571nK/sigil` README — AI-SPM positioning this console presents
- `Ju571nK/sigil` issue #9 / Roadmap project #3 — phase tracking
- `Ju571nK/sigil` `docs/superpowers/specs/2026-05-16-phase-3b1-ai-guard-risk-index-design.md`
  (gitignored, local-only)

These are in a different repo on the same machine, not in this one.
