# Import-export-status scoped-replace identity slices — 2026-09-03

## Objective

Amend `metin2-migrate import-export-status` so retained tip-`0007` / tip-`0009`
/ tip-`0010` / tip-`0013` `import-result.json` artifacts from scoped-replace
**wipe-to-empty** (and tip-`0007` multi-history) remain inspectable offline.

The original status contract required identity-slice lengths to equal their
corresponding row-count fields. That matched insert-only results where each
scope id mapped 1:1 onto inserted rows. Opt-in scoped replace changed that:

- tip-`0007`: `login_keys` is the **unique scope**; `ticket_count` is the
  inserted **row** count (history rows can share a login key; wipe lists a key
  with `ticket_count = 0`)
- tip-`0009`: `vnums` is wipe/replace **scope**; `template_count` is inserted
  template rows (`vnums = [N]`, `template_count = 0` on wipe)
- tip-`0010`: `vids` is wipe/replace **scope**; `ground_item_count` is inserted
  rows (`vids = [N]`, `ground_item_count = 0` on wipe)
- tip-`0013`: `entity_ids` / `combat_profiles` are wipe/replace **scope**;
  `static_actor_count` / `combat_profile_count` are inserted rows

Without this amend, `import-export-drill`'s printed
`import-export-status` redirect rejects valid retained evidence after a
successful scoped replace.

## Contract frozen by this slice

1. Tip-`0002` / tip-`0003` / tip-`0004` / tip-`0011` / tip-`0015` / tip-`0023`
   keep the existing rule that the character/account identity-slice length
   equals the corresponding `*_count` field (those Import* seams already set
   count = scope length, including wipe-to-empty).
2. Tip-`0007` status accepts:
   - non-negative `ticket_count` / `active_ticket_count`
   - `active_ticket_count <= ticket_count`
   - present `login_keys` (empty array allowed)
   - **no** `len(login_keys) == ticket_count` requirement
   - optional `replaced: true`
3. Tip-`0009` status accepts present `vnums` without requiring
   `len(vnums) == template_count` (wipe: non-empty `vnums`, zero template /
   child counts). Child counts stay non-negative.
4. Tip-`0010` status accepts present `vids` without requiring
   `len(vids) == ground_item_count`, while keeping
   `item_shaped_count + gold_shaped_count == ground_item_count`.
5. Tip-`0013` status accepts present `entity_ids` / `combat_profiles` without
   requiring equality to `static_actor_count` / `combat_profile_count`
   (wipe: scoped ids/names with zero row counts). Keep the existing
   `interaction_kinds` upper-bound vs `interaction_definition_count`.
6. Still fail closed on wrong tip identity, unknown/trailing JSON, negatives,
   symlink / oversized / empty / invalid UTF-8, and tip-`0002`-family
   account/character slice mismatches.
7. No stock production driver, no drill auto-enable of `--i-confirm-scoped-replace`,
   no daemon mutation route, no FileStore→SQL runtime repository, no README churn.

## Why now

- Tip vocabulary scoped-replace GREEN is complete through tip-`0007`
  ([auth-login-ticket-handoff import scoped replace GREEN](2026-09-03-auth-login-ticket-handoff-import-scoped-replace-green.md)).
- SQLite harness already emits wipe results the status checker rejects.
- Offline import-evidence inspection is Track E priority #2/#3 surface area.

## Likely files to change

- `docs/plans/2026-08-28-cli-import-export-status.md` (amend rule 5)
- `docs/development.md` (status wording + tip-0007 tip-sync punctuation cleanup)
- `docs/workflow/migration-apply-runbook.md` (pointer if needed)
- `docs/plans/2026-08-08-playable-vertical-roadmap.md` / `2026-08-09-db-migration-contract.md` (next-slice pointers)
- `internal/migratecli/import_export_status.go`
- `internal/migratecli/import_export_status_test.go`
- this plan

## TDD and validation

```bash
go test ./internal/migratecli -run 'ImportExportStatus' -count=1
gofmt -l internal/migratecli/import_export_status.go internal/migratecli/import_export_status_test.go
git diff --check
```

## Exit criteria

- Status accepts tip-`0007` wipe (`ticket_count=0`, non-empty `login_keys`, `replaced:true`)
- Status accepts tip-`0007` multi-history (`ticket_count > len(login_keys)`)
- Status accepts tip-`0009` / tip-`0010` / tip-`0013` wipe results
- Roster-style count/slice mismatches still exit `1`
- Docs tip-sync no longer mangles the tip-`0007` GREEN / freeze links
- Production-engine selection / drill auto-replace remain deferred

## Status

GREEN on `lane/persistence`. `metin2-migrate import-export-status` now treats
tip-`0007` / tip-`0009` / tip-`0010` / tip-`0013` identity slices as replace
scope rather than row-count twins. Production-engine selection remains deferred.

## Anti-goals

- Do not change Import* result JSON shapes.
- Do not auto-enable scoped replace in `import-export-drill` by default (opt-in print flag is a separate owned slice).
- Do not register a stock production driver.
- Do not push `origin/main`.
