# CLI Export Quarantine Inspection — 2026-08-19

## Objective

Add a read-only `metin2-migrate quarantine-export` command so operators can validate and canonicalize retained migration-shaped export JSON offline, without starting `gamed` or opening a database.

Loopback `POST .../quarantine` endpoints already exist for every shipped export shape. This slice closes the offline/runbook gap: retained artifacts can be inspected from a file or stdin beside the existing migration CLI.

## Contract frozen by this slice

```bash
metin2-migrate quarantine-export \
  --kind <kind> \
  --export <path|->
```

Supported `--kind` values:

| kind | migration boundary |
| --- | --- |
| `account-character-roster` | `0002_account_character_roster` |
| `character-item-state` | `0003_character_item_state` |
| `character-point-state` | `0011_character_point_state` |
| `character-quest-state` | `0004_character_quest_state` |
| `auth-login-ticket-handoff` | `0007_auth_login_ticket_handoff` |
| `item-template-state` | `0009_item_template_refine_info` |
| `static-actor-content-state` | `0008_static_actor_content_state` |
| `bootstrap-ground-item-state` | `0010_bootstrap_ground_item_state` |

Behavior:

1. `--kind` and `--export` are required.
2. `--export -` reads stdin; any other value opens a regular non-symlink file.
3. Input is capped at 1 MiB (same bound as the loopback quarantine POST bodies).
4. Input must be valid UTF-8, non-empty after trim, not literal JSON `null`, and must decode with `DisallowUnknownFields` plus no trailing JSON.
5. On success, stdout is indented JSON with the same `{ "summary": ..., "export": ... }` shape already returned by the loopback quarantine endpoints.
6. On contract failure (wrong migration boundary, invalid rows, malformed JSON, oversized body, symlink/non-regular file), exit `1` and write a short stderr reason with **no** stdout JSON.
7. Missing/unknown flags or unsupported kind → usage exit `2`.
8. The command never opens a database, never writes store snapshots, never emits SQL/DSN text, and never mutates live runtime state.

## What this is not yet

- DB INSERT / backfill execution
- repository seams
- automatic restore from quarantined exports
- daemon startup auto-quarantine
- remote admin auth

## TDD and validation

Focused coverage in `internal/migratecli`:

- successful quarantine for each kind (empty valid export is enough for most; roster uses a tiny valid payload)
- wrong migration boundary → exit `1`, no stdout
- malformed / invalid UTF-8 / oversized export → exit `1`
- missing flags / unknown kind → exit `2`
- usage text lists `quarantine-export`
- stdout omits SQL / DSN markers

Validation:

- `go test ./internal/migratecli -run 'QuarantineExport|RejectsUnknownCommand' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Add a dry-run helper that prints backup/restore drill commands from `/local/runtime-config`.
2. Extract repository seams only after offline quarantine + loopback quarantine both prove the export boundary.
3. Keep ground-item restart durability deferred until a real world-state repository exists.
