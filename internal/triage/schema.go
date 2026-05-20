package triage

// schemaSQL is the single source of truth for the triage DB. [Repo.Open]
// runs it on every connect; CREATE ... IF NOT EXISTS makes that idempotent.
//
// v1 has no schema versioning — column renames / type changes are out of
// scope until there's a second schema to migrate from. When that day comes,
// add a `schema_version` table and a per-version `up()` step.
//
// Keys / shape rationale:
//   - PRIMARY KEY (host_id, event_id) per contract §5.8 — `event_id` alone is
//     not stable across `EventUnprocessableLocal` rejections / replays.
//   - `evidence_snapshot` stores the raw JSON of the source event at triage
//     time. The producer's JSONL retention is operator-configurable; without
//     this snapshot a triage row could outlive its source event and become
//     orphaned in the UI (contract §13 retention mitigation).
//   - `triage_status_idx` matches the Alerts queue's primary query
//     (`status IN (...) ORDER BY updated_at DESC`).
const schemaSQL = `
CREATE TABLE IF NOT EXISTS triage (
  host_id           TEXT NOT NULL,
  event_id          TEXT NOT NULL,
  status            TEXT NOT NULL CHECK(status IN
                    ('open','acknowledged','investigating','resolved')),
  assignee          TEXT,
  evidence_snapshot TEXT NOT NULL,
  created_at        TEXT NOT NULL,
  updated_at        TEXT NOT NULL,
  PRIMARY KEY (host_id, event_id)
);

CREATE INDEX IF NOT EXISTS triage_status_idx
  ON triage(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS triage_notes (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  host_id    TEXT NOT NULL,
  event_id   TEXT NOT NULL,
  author     TEXT NOT NULL,
  body       TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (host_id, event_id) REFERENCES triage(host_id, event_id)
);

CREATE INDEX IF NOT EXISTS triage_notes_event_idx
  ON triage_notes(host_id, event_id, created_at);

CREATE TABLE IF NOT EXISTS triage_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  host_id     TEXT NOT NULL,
  event_id    TEXT NOT NULL,
  actor       TEXT NOT NULL,
  from_status TEXT,
  to_status   TEXT,
  at          TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS triage_log_event_idx
  ON triage_log(host_id, event_id, at);
`
