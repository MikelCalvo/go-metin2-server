# Character Point-State Migration — 2026-08-16

## Objective

Freeze the first SQL/backfill contract for the selected-character fixed point vector that the bootstrap runtime already persists in account snapshots and uses for visible gameplay state.

The current playable loop already mutates `loginticket.Character.Points` through item use, equipment effects, combat retaliation/death-floor handling, and the `PLAYER_POINTS` / `PLAYER_POINT_CHANGE` bootstrap packets. The existing `0002_account_character_roster` and `0003_character_item_state` migrations intentionally omit that fixed-width vector, so a future account repository or backfill import needs a separate boundary before any DB-backed runtime write is claimed.

## Contract frozen by this slice

`0011_character_point_state` adds one schema-only table:

- `character_points`

The table captures the current 255-slot signed point vector for every non-empty character row already covered by `0002_account_character_roster`:

- `character_id` — references `characters(id)` from the roster migration;
- `point_index` — fixed vector index, `0..254`;
- `value` — signed `int32` point value stored in a `BIGINT` carrier so the SQL boundary can represent zero, positive, and negative bootstrap point state without truncation;
- database-owned `created_at` / `updated_at` timestamps.

Rows are keyed by `(character_id, point_index)`. The export/backfill contract emits all 255 indices for each non-empty character, including zero values, so a deliberate zero HP floor or empty point slot is not confused with an omitted row.

## Runtime/export boundary

The slice adds a read-only migration-shaped projection:

- programmatic runtime: `ExportCharacterPointState()`
- loopback-only ops: `GET /local/account-store/exports/character-point-state`

The projection reads committed bootstrap account snapshots, validates them through the same roster boundary used by `0002`, and emits deterministic rows ordered by account, select-screen slot, and point index. It does not open a database, emit SQL, apply migrations, mutate account snapshots, or make the account runtime DB-backed.

## What this is not yet

This slice deliberately does not add:

- a DB-backed account or character repository;
- runtime reads from `character_points`;
- DB writes for item-use/equip/combat point mutations;
- import/quarantine execution tooling for the export;
- final legacy stat derivation policy;
- a daemon-local mutating migration endpoint.

## Why this order

Point state is now part of the PvE loop, not just select-screen decoration. Freezing it separately from roster and item instance rows keeps the schema explicit while preserving the existing bootstrap JSON store. It also avoids overloading the roster migration with a sparse or lossy representation of the 255-wide client point packet.

## TDD and validation

Focused coverage for this slice should prove:

- the embedded migration catalog includes `0011_character_point_state` after `0010_bootstrap_ground_item_state`;
- the migration files are manifest-pinned by SHA-256;
- the SQL owns `character_points`, `(character_id, point_index)` uniqueness, `0..254` point indexes, signed `int32` value bounds, and the roster foreign-key boundary;
- accountstore export emits all 255 point rows per non-empty character, including zeros and negative values, in deterministic order;
- `gamed` exposes the projection only through loopback-only read-only ops wiring;
- the broader repo remains green with `go test ./...`, `go vet ./...`, `git diff --check`, and formatted Go files.

## Follow-up options

1. Add import/quarantine tooling that verifies a point-state export against the migration shape without mutating current account snapshots.
2. Extract a repository seam for account/character/player-point state only after export/import preflight proves the boundary reduces file coupling.
3. Add DB-backed point writes only when roster, item-state, point-state, and quest-state repositories can be committed atomically for one selected character.
4. Keep daemon migration surfaces read-only; use CLI-only apply/rollback for mutating migration runs.
