# Quest State Bootstrap

This document freezes the first clean-room quest-state seam for `go-metin2-server`.

It is intentionally **not** a client-visible quest protocol yet. The current slice only owns a small deterministic state primitive that later NPC/service/quest slices can use without inventing a full script runtime first.

## Scope

The current owned surface is limited to `internal/queststate`:

- a file-backed quest-flag snapshot,
- deterministic snapshot canonicalization,
- strict validation of quest identities and flag names,
- one compare-and-set transition primitive for a single character flag,
- a read-only store validation summary plus crash-temp cleanup for the same snapshot format.

This seam is meant to support future content/NPC work such as “talk to an actor once and advance a flag”, but no static actor, packet, reward, dialog runtime, or loopback operator endpoint calls it yet.

## Snapshot shape

The file-backed snapshot shape is:

```json
{
  "flags": [
    {
      "character": "QuestHero",
      "quest_ref": "quest:first_steps",
      "name": "step",
      "value": 1
    }
  ]
}
```

Owned rules:

- missing snapshot file = store-level `not found`, not an implicit live quest runtime,
- missing `flags` collection in an existing JSON object = empty snapshot,
- JSON `null` root or `null` `flags` collection is invalid,
- unknown JSON fields are invalid,
- trailing JSON values are invalid,
- invalid UTF-8 or embedded NUL bytes in authored identifiers are invalid,
- snapshots are saved deterministically sorted by `character`, then `quest_ref`, then `name`,
- duplicate `(character, quest_ref, name)` rows are invalid after trimming.

## Identity rules

This first seam deliberately uses narrow bootstrap identities:

- `character`: non-empty ASCII bootstrap character name (`[A-Za-z0-9_]`), matching the conservative early character-name posture already used elsewhere in the repo,
- `quest_ref`: `quest:<name>` where `<name>` is lower-snake-ish (`[a-z][a-z0-9_]*`),
- `name`: lower-snake-ish quest flag name (`[a-z][a-z0-9_]*`).

The project does not yet own broader quest namespaces, localized quest labels, or legacy quest file names. Those can be added later only with their own tests/docs.

## Flag value rule

A persisted flag row must have `value != 0`.

For transitions, value `0` means “absent / not set”. A transition whose `to` value is `0` deletes the flag row when it is currently present.

This keeps the first snapshot compact and avoids two different representations for the same false/initial state.

## Transition contract

A transition request is:

```json
{
  "character": "QuestHero",
  "quest_ref": "quest:first_steps",
  "flag": "step",
  "from": 0,
  "to": 1
}
```

The owned operation is compare-and-set:

1. normalize and validate the transition identity,
2. normalize and validate the current snapshot,
3. read the current flag value, treating a missing row as `0`,
4. apply the update only when `current == from`,
5. return the canonical next snapshot plus a transition result.

A successful transition returns `applied = true` and preserves `current_value` as the value that matched `from`.

Known failure reasons:

| Reason | Meaning | Mutation |
| --- | --- | --- |
| `invalid_transition` | transition identity or current snapshot is invalid | none |
| `current_value_mismatch` | current flag value did not match `from` | none |

Failed transitions return no mutated snapshot.

## Runtime configuration and local ops

`gamed` now owns the quest-state store path as a normal bootstrap persistence selection:

- default path: `${TMPDIR}/go-metin2-server-quest-state.json` (using Go's `os.TempDir()`),
- global override: `METIN2_QUEST_STATE_STORE_PATH`,
- service-specific override: `METIN2_GAMED_QUEST_STATE_STORE_PATH`.

The quest-state file is included in the same persistence overlap preflight as the authored content stores. It must not share a path with account, login-ticket, static-actor, interaction, or item-template stores, must not resolve inside either directory-backed store, and must not be an existing directory.

The first local-only operator surfaces are also frozen on `gamed`:

- `POST /local/quest-state/validate`
- `POST /local/quest-state/crash-temps/cleanup`

Both endpoints are loopback-only, reject non-`POST` methods with `405`, reject non-empty bodies with `400`, reject oversized bodies with `413` through the existing local mutation body guard, and return `409` on validation/cleanup errors. They are persistence preflights for the server-side quest-state primitive, not a client-visible quest protocol or a quest mutation API.

## Store validation and crash-temp cleanup

`FileStore.Validate()` is the read-only primitive behind `/local/quest-state/validate` and `/local/persistence/status`. It does not mutate the committed snapshot. It returns a deterministic summary:

- `flag_count`,
- sorted distinct `characters`,
- sorted distinct `quest_refs`,
- sorted `flag_keys` using `character:quest_ref:name`,
- `crash_temp_count`,
- sorted `crash_temp_files`.

Validation treats a missing committed snapshot as an empty valid summary, matching the store-level “not found is not an implicit runtime” rule. Malformed committed snapshots, symlinked committed snapshots, malformed crash-temp directory reads, and symlinked crash-temp candidates fail closed.

`FileStore.CleanupCrashTempFiles()` removes only crash remnants that match the owned quest-state temp-file pattern:

```text
.quest-state-*.json
```

It validates the store before removal, leaves the committed `quest-state.json` and unrelated files alone, syncs the store directory after cleanup, and returns the post-cleanup validation summary.

These endpoints reuse the store primitives directly rather than duplicating snapshot parsing or temp-file matching rules.

## Current non-goals

This seam does **not** yet freeze:

- client quest packets,
- NPC dialog windows or option selection,
- quest acceptance/completion UI,
- quest rewards,
- quest item hooks,
- party/guild/account-wide quest state,
- timers or daily reset policy,
- script VM compatibility,
- content-bundle quest definitions,
- loopback operator endpoints for quest mutation beyond validation and crash-temp cleanup.

## Success definition

The current repository can now say:

- there is a tested, deterministic file-backed quest-flag primitive,
- one single-flag transition can initialize, advance, or clear a flag only when the caller-provided current value matches,
- the same store can be validated and cleaned of owned crash-temp files without mutating committed quest flags,
- bad identities, duplicate rows, malformed JSON, symlinked committed snapshots, symlinked crash-temp candidates, and mismatched current values fail closed,
- broader client-visible quest runtime remains future work.
