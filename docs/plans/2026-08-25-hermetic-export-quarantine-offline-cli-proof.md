# Hermetic Export → Offline Quarantine-Export CLI Proof — 2026-08-25

## Objective

Prove the already-shipped loopback migration-shaped export routes and the
offline `metin2-migrate quarantine-export` CLI form one operator-safe handoff:
`GET /local/.../exports/<kind>` from a drained `gamed` ops mux retains a tip-
shaped JSON artifact that `quarantine-export --kind ... --export <file>`
accepts and canonicalizes — without inventing a printer, remote admin, SQL
import, or changing export/quarantine contracts.

The shared registration helper
[`RegisterGamedMigrationQuarantineExportOps`](2026-08-24-gamed-migration-quarantine-export-ops-registration-helper.md)
and the offline CLI already landed. Operators still lacked a hermetic
end-to-end proof that a live loopback export file is accepted by the offline
quarantine path used in runbooks (`curl ... | quarantine-export --export -`
or a retained file).

## Why now

- The migration/quarantine registration helper plan explicitly deferred
  "hermetic HTTP drill that drives export → offline `quarantine-export`".
- Per-kind unit tests and helper registration tests do not exercise the
  retained-file handoff across the HTTP → CLI boundary.
- The PvE reconnect / migration-window vertical needs confidence that tip-
  `0015` safebox money exports and roster exports survive that handoff before
  any future printer or SQL backfill work.

## Contract frozen by this slice

1. Focused `internal/minimal` coverage materializes dedicated parent FileStores,
   seeds a drained runtime with:
   - one account/character roster row
   - one durable safebox snapshot including warehouse `money` (tip `0015`)
2. The test serves only
   `RegisterGamedMigrationQuarantineExportOps(ops.NewPprofMux("gamed"), runtime)`
   on a loopback `httptest.Server` (no file-store backup routes required).
3. The test GETs:
   - `/local/account-store/exports/account-character-roster`
   - `/local/safebox-store/exports/character-safebox-state`
4. Each response body is written to a regular non-symlink temp file.
5. `migratecli.Run(["quarantine-export", "--kind", <kind>, "--export", <path>])`
   returns exit `0` for both kinds, with stdout JSON containing:
   - roster: `account_count` / `character_count` and the seeded login/name
   - safebox: `password_count` / `item_count`, seeded login, and tip
     `migration_version` / `migration_name` for `0015_character_safebox_money`
     plus the seeded warehouse money on the canonicalized export
6. Offline stdout omits SQL / DSN markers (`CREATE TABLE`, `postgres://`, etc.).
7. Optional parity check: loopback `POST .../quarantine` for the roster export
   body returns `200` with the same summary counters the offline CLI printed.
8. Docs mark the migration-helper follow-up done and name this proof as the
   hermetic owner for the export → offline quarantine-export handoff.

## What this is not yet

- a `metin2-migrate export-quarantine-drill` printer that emits curl scripts
- automatic / scheduled execution of printed triage scripts
- covering every empty export kind in the hermetic HTTP proof (roster + tip
  safebox are the seeded non-empty gate; other kinds remain unit-covered)
- SQL import/backfill from quarantined exports
- `rm` of aside-renamed trees
- FreeBSD port / `pkg` enable defaults
- remote log shipping / metrics exporters
- remote admin authentication
- changing export schemas, quarantine validators, or migration tip `0015`

## Likely files to change

- `internal/minimal/export_quarantine_offline_cli_http_test.go` (new)
- `docs/plans/2026-08-24-gamed-migration-quarantine-export-ops-registration-helper.md`
- `docs/development.md` (brief pointer beside `quarantine-export`)
- this plan

## TDD and validation

Focused coverage in `internal/minimal`:

- loopback roster export file → offline `quarantine-export` succeeds
- loopback tip-`0015` safebox export file (with money) → offline
  `quarantine-export` succeeds and retains money
- offline stdout omits SQL/DSN markers
- loopback roster POST quarantine summary counters match offline summary

Validation for this slice:

- `go test ./internal/minimal -run 'ExportQuarantineOfflineCLIHTTP' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Exit criteria

- hermetic HTTP → retained-file → offline CLI proof is green
- migration-helper plan marks the deferred follow-up done
- docs point operators at the proof for the handoff

## Anti-goals / ordering constraints

- Do not add a printer in this slice.
- Do not widen registration helpers or change endpoint paths/bodies.
- Do not auto-run printed scripts from CLI.
- Do not push `origin/main`; push only `origin/lane/persistence`.
