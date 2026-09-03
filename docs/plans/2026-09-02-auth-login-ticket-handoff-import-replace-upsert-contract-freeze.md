# Auth-login-ticket-handoff SQL import replace/upsert contract freeze — 2026-09-02

## Objective

Freeze the next fail-closed **scoped replace** policy for quarantined tip-`0007`
(`auth_login_ticket_handoff` / `auth_login_tickets`) before opening RED, so
operators can re-backfill a retained authd→gamed login-ticket handoff export
without hitting insert-only primary-key or active-login-key unique-index
conflicts.

This freeze does **not** invent a stock production driver, live DB ticket
repository replacing FileStore issue/load/consume, catalog tip `0030`, remote
admin, silent row-merge semantics, or daemon mutation routes.

## Why docs-first

Track E character-domain, roster, pending-ground, item-template, and
static-actor scoped replace paths are already GREEN on `main`:

- [character item-state import scoped replace GREEN](2026-08-31-character-item-state-import-scoped-replace-green.md)
- [character safebox-state import scoped replace GREEN](2026-09-01-character-safebox-state-import-scoped-replace-green.md)
- [character quest-state import scoped replace GREEN](2026-09-01-character-quest-state-import-scoped-replace-green.md)
- [character point-state import scoped replace GREEN](2026-09-01-character-point-state-import-scoped-replace-green.md)
- [character myshop unit-prices import scoped replace GREEN](2026-09-01-character-myshop-unit-prices-import-scoped-replace-green.md)
- [account/character roster import scoped replace GREEN](2026-09-02-account-character-roster-import-scoped-replace-green.md)
- [bootstrap ground-item-state import scoped replace GREEN](2026-09-02-bootstrap-ground-item-state-import-scoped-replace-green.md)
- [item-template-state import scoped replace GREEN](2026-09-02-item-template-state-import-scoped-replace-green.md)
- [static-actor content-state import scoped replace GREEN](2026-09-02-static-actor-content-state-import-scoped-replace-green.md)

Catalog tip remains `0029_bootstrap_ground_item_instance_attributes`.

`loginticket.ImportAuthLoginTicketHandoff` is still insert-only and fails closed
on a second import of the same `(login_key, issued_at)` primary keys or a second
active (unconsumed) row for the same `login_key` (explicitly “does not invent
upsert / merge policy”). That is safe for first-time backfill of a drained
handoff set, but it blocks the honest operator re-backfill path after a retained
tip-`0007` export is corrected or a lab DB is rebuilt to tip and re-seeded.

Auth login tickets sit on the playable login → select → map vertical already
owned by FileStore issue/load/consume. Owning the re-backfill contract next
keeps that durable projection operable without waiting on production-engine
selection, and it closes the last remaining tip-kind scoped-replace gap called
out after tip-`0013` GREEN.

Opening RED without freezing:

- default vs opt-in confirmation,
- replace scope (`login_key` vs `(login_key, issued_at)`),
- declared login-key scope vs row-derived keys,
- multi-row history under one login key (active + consumed),
- wipe-to-empty / empty no-op semantics,
- transaction / fail-closed semantics,
- CLI flag shape,

would invent policy mid-implementation. Freeze first; GREEN stays follow-on.

## Contract to freeze (before RED)

### A. Default remains insert-only

1. `ImportAuthLoginTicketHandoff(...)` without an explicit replace option keeps
   today's insert-only behavior: duplicate primary keys / active-login-key
   unique-index collisions fail closed and roll the transaction back.
2. `metin2-migrate import-export` without an explicit replace confirmation keeps
   today's insert-only path for every kind, including
   `auth-login-ticket-handoff`.
3. No silent upgrade of insert-only into replace.

### B. Opt-in scoped replace for tip-`0007` only (this freeze)

1. A new programmatic option (name TBD in GREEN; treat as
   `ImportAuthLoginTicketHandoffOptions{Replace: true}` mirroring tip-`0002` /
   tip-`0003` / tip-`0004` / tip-`0009` / tip-`0010` / tip-`0011` / tip-`0013` /
   tip-`0015` / tip-`0023`) performs **scoped replace** for the login keys
   present in the quarantined export summary / declared scope list.
2. Scope is exactly the tip-`0007` table already owned by
   `ImportAuthLoginTicketHandoff`:
   - `auth_login_tickets`
3. Tip-`0007` identity for replace scope is **`login_key`**, not the composite
   primary key `(login_key, issued_at)`. One login key may own multiple historical
   rows (consumed history plus at most one active/unconsumed row). Replace of a
   listed login key therefore deletes **all** existing `auth_login_tickets` rows
   for that login key before inserting the export's rows for that key.
4. For each `login_key` listed in the canonicalized export summary / declared
   `login_keys`:
   - delete existing `auth_login_tickets` rows for that `login_key` (if any),
   - insert the canonicalized export rows for that `login_key` when present,
   - all inside **one** transaction after the existing quarantine + schema
     preflight (ledger entry for version `7` / `auth_login_ticket_handoff`).
5. Login keys **not** listed in the export are left untouched (no global
   truncate).
6. Export tip identity stays `0007_auth_login_ticket_handoff`; do **not** retip
   export / quarantine / import-result identity.
7. GREEN delete/insert order for a listed key is: delete-by-`login_key`, then
   insert the export rows for that key (order among inserted rows may stay the
   already-owned quarantine canonical order).

### C. Declared login-key scope + empty / wipe semantics

1. Tip-`0007` exports gain an optional `login_keys` field (same merge idea as
   tip-`0002` `account_ids`, tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` /
   tip-`0023` `character_ids`, tip-`0009` `vnums`, and tip-`0010` `vids`):
   declared keys merge with ticket-row-derived login keys so a listed login key
   with zero ticket rows can wipe-to-empty.
2. Quarantine continues to reject zero `login_key`, empty/whitespace-padded
   logins, `login_normalized` mismatches, duplicate `(login_key, issued_at)`,
   duplicate active (unconsumed) `login_key`, `consumed_at` before `issued_at`,
   empty / invalid `characters_snapshot_json`, and other already-owned tip-`0007`
   shape failures. Those fail closed before any DELETE / INSERT.
3. A listed login key with zero ticket rows is allowed only as an explicit
   wipe-to-empty scope entry (via declared `login_keys`).
4. An export with an empty `login_keys` list and empty `tickets` is a no-op
   mutation after quarantine/schema preflight (commit allowed; result counts stay
   zero).
5. Malformed / non-quarantinable exports continue to fail before any DELETE /
   INSERT.
6. Replace of a listed login key replaces that key's entire tip-`0007` row set
   for the export (every `(login_key, issued_at)` history row present in the
   export, including optional consumed history). There is no per-issued-at
   half-replace / silent merge / keep-missing-history mode in this freeze.
7. Historical rows for a listed login key that are absent from the export are
   removed by the scoped DELETE; this freeze does **not** invent a
   keep-missing-history merge mode.

### D. CLI confirmation shape (tip-`0007` added beside owned replace kinds)

1. Replace stays off unless the operator passes an explicit confirmation flag
   in addition to `--i-confirm-sql-import`. GREEN should reuse the existing
   `--i-confirm-scoped-replace` flag and widen it to also accept
   `--kind auth-login-ticket-handoff` (still reject every other kind that has
   not frozen+GREEN'd its own replace path — after this freeze lands, tip-`0007`
   is the remaining kind awaiting GREEN among the current tip vocabulary).
2. Successful stdout remains metadata-only
   `AuthLoginTicketHandoffImportResult` JSON (no DSN, no SQL text, no ticket
   payloads / character-snapshot bytes). GREEN should add `replaced: true`
   (omitempty) mirroring tip-`0002` / tip-`0003` / tip-`0004` / tip-`0009` /
   tip-`0010` / tip-`0011` / tip-`0013` / tip-`0015` / tip-`0023`.
3. Print-only `import-export-drill` does **not** auto-enable replace by default; opt-in `--i-confirm-print-scoped-replace` is owned by
   [import-export-drill opt-in scoped-replace printer](2026-09-03-import-export-drill-opt-in-scoped-replace.md).

### E. Explicit non-goals

- stock production DB driver registration in `gamed` / `authd` / `metin2-migrate`
- live DB ticket repository replacing FileStore issue/load/consume
- catalog tip `0030` / retip of tip-`0007` / `0002` / `0003` / `0004` / `0009` /
  `0010` / `0011` / `0013` / `0015` / `0023` export identities
- inventing cascade rewrite of account/character/child domains from a ticket
  replace path
- silent merge / per-row `ON CONFLICT DO UPDATE` without scoped DELETE
- online mutation while authd/gamed handoff is live (CLI remains an offline
  operator tool; no daemon mutation route; FileStore / MemoryStore / live
  issue/consume stay untouched)
- remote admin, secrets in git, README churn, gameplay changes
- another crash/restart rematerialize twin (FileStore ticket durability /
  backup/restore is already owned)

## Proof shape (for later RED → GREEN)

1. SQLite harness: first insert-only import succeeds; second insert-only import
   of the same export fails closed; opt-in replace of the same export succeeds
   and leaves exactly the canonical ticket rows (including optional consumed
   history + active uniqueness).
2. Scoped wipe: login key A in export, login key B only in DB beforehand →
   replace updates A and leaves B untouched.
3. Empty wipe: listed login key with zero ticket rows via declared `login_keys`
   → that key's `auth_login_tickets` rows become absent inside the transaction.
4. Multi-row history: replace of a login key that previously had consumed +
   active rows leaves only the export's rows for that key (no keep-missing-
   history merge).
5. CLI: insert-only confirmation alone cannot replace;
   `--i-confirm-scoped-replace` is accepted for `auth-login-ticket-handoff`
   (and still for already-owned replace kinds); any still-unfrozen kind continues
   to reject it.
6. Negatives: missing schema tip `7`, bad quarantine (duplicate primary key /
   duplicate active login key / invalid snapshot JSON), nil executor, and
   active-unique-index mistakes still fail closed before commit.

## Likely files to change (later GREEN, not this freeze)

- `internal/loginticket/migration_export.go` /
  `migration_export_quarantine.go` (optional `login_keys` merge + wipe-to-empty
  exception)
- `internal/loginticket/auth_login_ticket_handoff_import.go` (+ unit / sqlite
  harness tests)
- `internal/migratecli` import-export flag wiring (+ tests)
- `docs/development.md` / `docs/workflow/migration-apply-runbook.md` (GREEN
  wording once the kind is accepted)
- Track E / migration-contract next-slice pointers (flip freeze → Done on GREEN)

## Status

Docs/spec freeze landed. Tip-`0007` scoped replace GREEN is owned by
[auth-login-ticket-handoff import scoped replace GREEN](2026-09-03-auth-login-ticket-handoff-import-scoped-replace-green.md).
Insert-only remains the default without the replace option / CLI confirmation.
Production-engine selection remains deferred.
