# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repo status

As of 2026-05-19 the repo is in **Plan 02 implementation** (Foundation +
Alerts queue). Plan 01 (scaffold + CI) shipped on `main`. Plan 02 work lives
on `feat/plan-02-foundation-and-alerts`. See
[`Ju571nK/sigil-manager#1`](https://github.com/Ju571nK/sigil-manager/issues/1)
for current task state.

Source-of-truth docs:
- `sigil-strategy.md` — productization strategy + scope decisions
- `docs/superpowers/specs/2026-05-16-fleet-api-contract.md` — v1.0 fleet API
  consumed from `sigil-server`, locked against producer Phase 3b.4 plus the
  additive §14 notes for 3b.6 / 3b.6.1 / 3b.6.2
- `docs/superpowers/specs/2026-05-16-ui-ux-design.md` — UI/UX spec
- `docs/superpowers/plans/2026-05-18-plan-02-foundation-and-alerts-queue.md` —
  current plan, 16 tasks T0–T15

Read these before architectural suggestions or scope decisions.

## Cross-repo sync workflow (run at every session start)

The producer (`Ju571nK/sigil`) does NOT write files into this repo, and we do
NOT write files into the producer repo. **All cross-repo state flows through
GitHub issues, the Project board, and (optionally) Discussions.** Producer
visibility depends entirely on those surfaces being current — keep them in
sync at session boundaries.

At the start of every session that touches Plan 02 (or any later plan), run
the following before doing new implementation work. None of these steps is
optional — skipping step 4 in particular hides progress from the producer
and reopens the alignment problems the v1.0 contract closed.

1. **Pull producer state.** `git fetch origin` here AND
   `git -C ../anti_i fetch origin` (the producer clone). Skim recent commits
   on `../anti_i` main for any post-lock wire additions; if any new
   `Evidence` variants, `tool` enum values, or `scope.kind` shapes have
   shipped, update `docs/superpowers/specs/2026-05-16-fleet-api-contract.md
   §14` as a new sub-section. The contract is the consumer-side mirror of
   producer's wire surface — it must absorb additive changes the moment
   they ship.
2. **Check producer issues for new asks.**
   `gh issue list -R Ju571nK/sigil --state open --search "consumer OR sigil-manager"`.
   Anything that blocks current work gets surfaced before proceeding.
3. **Push any unpushed commits.** `git log @{u}..HEAD` shows local-only
   commits; push immediately so producer can pull and review. Feature
   branches are fair game. Pushing to `main` requires explicit per-session
   user authorization, even if `.claude/settings.local.json` has the rule.
4. **Refresh issue #1's "Progress log" table.** The Plan 02 epic at
   `Ju571nK/sigil-manager#1` has a Progress-log table near the top. Append
   a row for any task completed since the last refresh, then update the
   "Currently in progress" line:
   ```bash
   gh issue view 1 -R Ju571nK/sigil-manager --json body --jq .body > /tmp/issue1.md
   # edit /tmp/issue1.md — add a row + update T-checkboxes + bump "Currently in progress"
   gh issue edit 1 -R Ju571nK/sigil-manager --body-file /tmp/issue1.md
   ```
   Producer polls this section.
5. **File producer asks via issues, not direct edits.** When work surfaces
   something the producer needs to change (D-class divergences, missing
   endpoint, spec ambiguity), file it on `Ju571nK/sigil` with a clear repro
   and proposed fix. Cross-link from
   `docs/superpowers/specs/2026-05-16-fleet-api-contract.md §13.1` when the
   gap is contract-relevant. The user has authorized cross-repo issue
   creation for this purpose (2026-05-19, transcript: "너도 필요한게 있으면
   git이슈로 등록해라").
6. **Use Discussions for open-ended questions.** When something is
   design-shaped rather than bug-shaped (e.g., "should the retention floor
   become a contract field?"), open a thread on `Ju571nK/sigil-manager`
   Discussions and cross-link from the relevant spec section. Issues are
   for trackable items; Discussions are for chatter.

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
