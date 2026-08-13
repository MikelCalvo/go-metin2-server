# Quest State Bootstrap

This document freezes the first clean-room quest-state seam for `go-metin2-server`.

It is intentionally **not** a client-visible quest protocol yet. The current slice only owns a small deterministic state primitive that later NPC/service/quest slices can use without inventing a full script runtime first.

## Scope

The current owned surface is limited to `internal/queststate`:

- a file-backed quest-flag snapshot,
- deterministic snapshot canonicalization,
- strict validation of quest identities and flag names,
- one compare-and-set transition primitive for a single character flag,
- one read-only transition preview plus store validation summary and crash-temp cleanup for the same snapshot format,
- one read-only exact-character quest-state snapshot for local operator QA,
- content-bundle import/export/summary inclusion for the same standalone quest-state snapshot.

This seam is meant to support future content/NPC work such as “talk to an actor once and advance a flag”. The current local operator endpoints can validate, dry-run or mutate through one compare-and-set transition, read back one character's quest flags, and inspect/import/export quest-state rows through authored content bundles, but no static actor, client packet, reward, dialog runtime, or quest script calls it automatically yet.

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

The file-backed store now exposes that operation as `ApplyTransition(...)`. It treats a missing committed snapshot as an empty current quest-state snapshot only for the purpose of evaluating the transition, then persists a canonical snapshot only when `applied = true`. Invalid transitions and `current_value_mismatch` results return the current summary without rewriting the snapshot.

`PreviewTransition(...)` uses the same compare-and-set evaluator and response shape without writing the committed snapshot. If the transition would apply, its response summary describes the hypothetical post-transition snapshot. If the transition would fail, its response summary describes the current committed snapshot. A missing committed snapshot is previewed as an empty snapshot, but no store file is created.

## Runtime configuration and local ops

`gamed` now owns the quest-state store path as a normal bootstrap persistence selection:

- default path: `${TMPDIR}/go-metin2-server-quest-state.json` (using Go's `os.TempDir()`),
- global override: `METIN2_QUEST_STATE_STORE_PATH`,
- service-specific override: `METIN2_GAMED_QUEST_STATE_STORE_PATH`.

The quest-state file is included in the same persistence overlap preflight as the authored content stores. It must not share a path with account, login-ticket, static-actor, interaction, or item-template stores, must not resolve inside either directory-backed store, and must not be an existing directory.

The first local-only operator surfaces are also frozen on `gamed`:

- `POST /local/quest-state/validate`
- `POST /local/quest-state/crash-temps/cleanup`
- `POST /local/quest-state/transition-preview`
- `POST /local/quest-state/transition`
- `GET /local/quest-state/characters/{character}`
- `GET /local/quest-state/quests/{quest_ref}`
- `GET /local/quest-state/flags/{character}/{quest_ref}/{flag}`

The validation and cleanup endpoints are loopback-only, reject non-`POST` methods with `405`, reject non-empty bodies with `400`, reject oversized bodies with `413` through the existing local mutation body guard, and return `409` on validation/cleanup errors. They are persistence preflights for the server-side quest-state primitive, not a client-visible quest protocol.

`/local/quest-state/transition-preview` is the read-only dry-run harness for the same primitive. It accepts the transition JSON shape above, rejects invalid JSON, unknown fields, trailing JSON, invalid UTF-8, JSON `null`, oversized bodies, wrong methods, and non-loopback callers before invoking the runtime. It returns the same result JSON shape as the mutating endpoint, but the summary is hypothetical on `applied = true` and committed-current on `applied = false`; it never persists the hypothetical snapshot.

`/local/quest-state/transition` is the local-only mutation harness for this primitive. It accepts the transition JSON shape above, rejects invalid JSON, unknown fields, trailing JSON, invalid UTF-8, JSON `null`, oversized bodies, wrong methods, and non-loopback callers before invoking the runtime. It returns the store result as JSON with:

- the canonical `transition`,
- the compare-and-set `result`,
- the post-attempt `summary`.

Compare-and-set failures such as `current_value_mismatch` return `200 OK` with `applied = false` and the failure `reason`; they are expected authored-state outcomes, not transport errors. Runtime/store failures that prevent evaluating or persisting the transition return `409`. This endpoint is an operator/bootstrap harness for testing authored quest-state progression and recovery. It is still not a client-visible quest packet, NPC dialog path, reward path, or remote admin API.

`GET /local/quest-state/characters/{character}` is the first read-only exact-character inspection endpoint for the same store. It is loopback-only, rejects non-`GET` methods with `405`, rejects blank or slash-containing character path values with `400`, returns `404` when the store has no flags for that character, and returns `409` when the committed quest-state snapshot cannot be loaded or validated. Successful responses use this deterministic JSON shape:

```json
{
  "character": "QuestHero",
  "flags": [
    {"quest_ref": "quest:first_steps", "name": "met_guard", "value": 1},
    {"quest_ref": "quest:first_steps", "name": "step", "value": 2}
  ]
}
```

The `flags` array is already in store-canonical order (`quest_ref`, then `name`) because the file-backed store normalizes the underlying snapshot by `character`, then `quest_ref`, then `name`. This endpoint does not infer account rosters, connected sessions, quest availability, or zero-valued flags. A character with no persisted non-zero quest flags is therefore indistinguishable from an unknown character at this seam and returns `404`.

`GET /local/quest-state/quests/{quest_ref}` and `GET /local/quest-state/flags/{character}/{quest_ref}/{flag}` are narrower read-only readers over the same committed snapshot. They are loopback-only, accept only `GET`, reject malformed path identities before loading the store, return `404` when the requested non-zero quest/flag row is absent, and return `409` when the committed snapshot cannot be loaded or validated.

The exact-quest response groups canonical rows by character:

```json
{
  "quest_ref": "quest:first_steps",
  "flag_count": 2,
  "characters": [
    {"character":"AnotherHero","flags":[{"quest_ref":"quest:first_steps","name":"met_guard","value":1}]},
    {"character":"QuestHero","flags":[{"quest_ref":"quest:first_steps","name":"step","value":2}]}
  ]
}
```

The exact-flag response uses the persisted row shape:

```json
{"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":2}
```

Both shapes are local QA/operator inspection aids only; they do not define quest objectives, NPC dialog state, reward state, or client quest packets.

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

## Content-bundle boundary

The content-bundle layer accepts an optional `quest_state` collection with the same row shape as the standalone file store:

```json
"quest_state": [
  {"character":"QuestHero","quest_ref":"quest:first_steps","name":"step","value":1}
]
```

Bundle canonicalization normalizes and sorts this collection with the same `queststate.NormalizeSnapshot(...)` ordering as the file store. Bundle validation rejects invalid or duplicate quest-state rows through `queststate.ValidSnapshot(...)`. Runtime `GET /local/content-bundle` exports the configured quest-state store into this collection, and runtime `POST /local/content-bundle` replaces the configured quest-state snapshot with the canonical bundle rows. Omitting `quest_state` imports an empty quest-state snapshot for this bootstrap content-bundle path. The checked-in `docs/examples/bootstrap-npc-service-bundle.json` fixture now carries one canonical `QuestHero / quest:first_steps / step = 1` row so local QA can validate the portable quest-state path together with authored NPC service content.

Import-preview and summary responses include `quest_state_flag_count`, `quest_state_character_count`, `quest_state_quest_count`, deterministic `quest_state_quest_refs`, per-character `quest_state_characters` rows, and per-quest `quest_state_quests` rows so operators can inspect candidate quest-state content without fetching the full bundle. The quest count is derived from the canonical distinct `quest_ref` set, not from static actor metadata or future quest definitions. `gamed` also exposes loopback-only read-only focused readers for the live exported bundle:

- `GET /local/content-bundle/quest-state/characters/{character}` returns one exact `quest_state_characters[]` summary row.
- `GET /local/content-bundle/quest-state/quests/{quest_ref}` returns one exact `quest_state_quests[]` summary row keyed by a valid `quest:<name>` ref.
- `GET /local/content-bundle/quest-state/flags/{character}/{quest_ref}/{flag}` returns one exact persisted non-zero quest flag row from the exported bundle summary.

The per-flag reader validates the same path-safe `character`, `quest_ref`, and lower-snake flag-name identities as the store primitive, then returns the canonical row shape (`character`, `quest_ref`, `name`, `value`) or `404` when that exact flag is absent. The per-quest summary groups the already-canonical quest-state rows by `quest_ref`, preserves deterministic character ordering within each quest, and includes each matching character's deterministic flag summaries. These are operator inspection shapes only; they do not define quest objectives or make quest refs executable.

The content-bundle boundary is still authored-content plumbing only: it does not define quest objectives, transition triggers, NPC dialogs, rewards, or client quest packets.

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
- content-bundle quest definitions beyond portable flag rows,
- static-actor/NPC interaction hooks that call `/local/quest-state/transition` or the store transition primitive automatically.

## Success definition

The current repository can now say:

- there is a tested, deterministic file-backed quest-flag primitive,
- one single-flag transition can initialize, advance, or clear a flag only when the caller-provided current value matches,
- `gamed` exposes a loopback-only `POST /local/quest-state/transition-preview` harness for dry-running the exact same primitive without writing the committed snapshot,
- `gamed` exposes a loopback-only `POST /local/quest-state/transition` harness for applying that primitive without inventing client quest packets or NPC dialog semantics,
- `gamed` exposes loopback-only readback harnesses for inspecting one persisted character flag set, all persisted flags for one quest ref, or one exact flag row without mutating quest state,
- content-bundle import/export now includes the configured quest-state snapshot and exposes focused `GET /local/content-bundle/quest-state/characters/{character}`, `GET /local/content-bundle/quest-state/quests/{quest_ref}`, and `GET /local/content-bundle/quest-state/flags/{character}/{quest_ref}/{flag}` readers for bundle-summary rows,
- the same store can be validated and cleaned of owned crash-temp files without mutating committed quest flags,
- bad identities, duplicate rows, malformed JSON, symlinked committed snapshots, symlinked crash-temp candidates, and mismatched current values fail closed,
- broader client-visible quest runtime remains future work.
