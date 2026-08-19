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
- content-bundle import/export/summary inclusion for the same standalone quest-state snapshot,
- the first static-actor `quest_flag` interaction kind that applies one authored compare-and-set transition for the selected character,
- the first spawn-group kill-quest credit descriptor that applies one authored compare-and-set transition for the selected killer after an accepted non-player death edge.

This seam supports the first content/NPC path of “interact with an actor once and advance or clear a flag”, plus the first combat-adjacent content path of “kill an authored spawn-backed combatant and advance one selected-killer quest flag”. The current local operator endpoints can validate, dry-run or mutate through one compare-and-set transition, read back one character's quest flags, and inspect/import/export quest-state rows through authored content bundles. A visible static actor can now call the same primitive through `INTERACT` when its authored definition uses `interaction_kind = "quest_flag"`, and an authored spawn group may also call the same primitive after the accepted killing hit when it carries kill-quest credit fields. No client quest packet, reward UI, branching dialog runtime, or quest script exists yet.

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

## Static-actor quest flag interaction

The first authored NPC/content trigger is `interaction_kind = "quest_flag"` on the existing `INTERACT (0x0501)` static-actor path. Its interaction definition shape extends the ordinary `kind + ref` catalog entry with the transition fields:

```json
{
  "kind": "quest_flag",
  "ref": "quest:first_steps",
  "text": "Quest updated: first_steps.met_guide = 1.",
  "quest_ref": "quest:first_steps",
  "quest_flag": "met_guide",
  "quest_from": 0,
  "quest_to": 1
}
```

Owned rules:

- `kind` must be exactly `quest_flag`.
- `ref` is still the interaction-definition identity and must use the existing namespaced lower-snake ref rule.
- `text` is required, UTF-8, NUL-free, and is returned as one self-only `CHAT_TYPE_INFO` acknowledgement after a successful transition.
- `quest_ref` must satisfy the quest-state `quest:<name>` identity rule.
- `quest_flag` must satisfy the lower-snake flag-name rule.
- `quest_from` defaults to `0` when omitted and is the compare-and-set expected value.
- `quest_to` must differ from `quest_from`; `quest_to = 0` clears the flag through the same compare-and-set primitive when the current value matches `quest_from`.
- `title`, merchant `catalog`, warp `map_index`, `x`, and `y` are not valid for `quest_flag` definitions.

Runtime behavior:

1. the player must already be in `GAME`, have a live selected character, and target a visible/in-range interactable static actor,
2. the actor's metadata must resolve to a valid `quest_flag` definition,
3. `gamed` applies the transition to the selected character name through the same quest-state store primitive used by `/local/quest-state/transition`,
4. if and only if the transition applies, the client receives one self-only `GC_CHAT` with `type = INFO`, `vid = 0`, `empire = 0`, and `message = definition.text`,
5. when the compare-and-set result is `current_value_mismatch`, the quest-state snapshot remains unchanged and the client now receives one self-only `GC_CHAT` with `type = INFO`, `vid = 0`, `empire = 0`, and `message = "Quest requirements are not met."`,
6. invalid transition definitions, store errors, unsupported content, and other non-CAS failures still fail closed with no frames and no peer fanout.

Loopback interaction visibility now mirrors that player-facing branch without mutation: `GET /local/interaction-visibility` and `GET /local/interaction-visibility/{character}` preview a `quest_flag` actor by dry-running the selected character's compare-and-set transition. A transition that would apply previews `definition.text`; a `current_value_mismatch` previews `Quest requirements are not met.`. Other dry-run failures surface as a fail-closed `resolution_failure` marker rather than mutating the quest-state store.

This is still a bootstrap quest-state trigger, not a client quest UI, reward system, branching dialog tree, or script runtime. The mismatch acknowledgement and its loopback preview exist only so authored-state failures are not silent; they do not expose a quest window, reward, objective tracker, or alternate branch.

## Optional quest gates on non-mutating interactions

`info`, `talk`, `warp`, and `shop_preview` definitions may optionally carry a selected-character quest-flag prerequisite without becoming `quest_flag` mutators:

```json
{
  "kind": "warp",
  "ref": "npc:qa_teleporter",
  "text": "Step through the gate.",
  "map_index": 1,
  "x": 470200,
  "y": 964200,
  "quest_ref": "quest:first_steps",
  "quest_flag": "met_guide",
  "quest_from": 1
}
```

Owned gate rules:

- the gate is present only when both `quest_ref` and `quest_flag` are authored
- `quest_from` defaults to `0` and is the exact required current flag value
- `quest_to` must remain absent/`0`; gated non-mutating interactions never mutate quest state
- partial gate fields (`quest_ref` without `quest_flag`, or the reverse) and orphan `quest_from` on ungated non-mutating definitions fail store validation
- when the live selected character's current flag value matches `quest_from`, the ordinary `info` / `talk` / `warp` / `shop_preview` outcome continues unchanged
- when the current value mismatches, the client receives the same self-only `CHAT_TYPE_INFO` text already owned by `quest_flag` mismatch (`Quest requirements are not met.`) and no authored info/talk text, transfer, or merchant window is delivered
- once a gated `shop_preview` merchant window is already open, later packet `SHOP BUY` / `SHOP SELL` / `SHOP SELL2` and the local `/shop_buy` harness must re-resolve that same merchant target through the ordinary interaction path before mutating gold or inventory; if the selected character's live quest flag no longer matches the authored gate, the session receives one self-only `GC::SHOP END`, the active merchant context clears immediately, and gold/inventory remain unchanged
- loopback interaction-visibility previews for gated non-mutating interactions reuse that same mismatch text without mutating quest state
- content-bundle warp destination/route summaries and shop-route summaries now surface the authored gate fields so operators can audit teleporter/merchant prerequisites without opening the live interaction path

The checked-in QA example `docs/examples/bootstrap-npc-service-bundle.json` now gates `npc:qa_guide`, `lore:qa_square`, `npc:qa_teleporter`, and `npc:qa_merchant` on `quest:first_steps.met_guide = 1`, so the owned service unlock loop is: interact with `QuestGuide` once, then use the guide/signpost/teleporter/merchant. The same fixture also closes the first combat-adjacent quest loop: kill `QARewardMob` to advance `quest:first_steps.killed_qa_mob`, then interact with `QuestHunter` (`quest:first_steps_kill_turnin`) to clear that flag through an ordinary `quest_flag` compare-and-set turn-in. A later operator or `quest_flag` reset that clears `met_guide` while a previously opened QA merchant window is still open must therefore auto-close that stale window on the next buy/sell attempt instead of letting the transaction continue under a revoked prerequisite.

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
- `GET /local/quest-state`
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

`GET /local/quest-state` is the no-argument read-only overview for the committed standalone quest-state store. It is loopback-only, accepts only `GET`, returns `409` when the committed snapshot cannot be loaded or validated, and treats a missing committed snapshot as an empty overview. Successful responses include deterministic counts, sorted `quest_refs`, exact per-character flag snapshots, and exact per-quest grouped snapshots. This is the store-backed counterpart to the content-bundle overview reader; it lets local QA inspect live committed quest flags directly without fetching the broader content bundle summary.

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

All readback shapes are local QA/operator inspection aids only; they do not define quest objectives, NPC dialog state, reward state, or client quest packets.

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

Import-preview and summary responses include `quest_state_flag_count`, `quest_state_character_count`, `quest_state_quest_count`, deterministic `quest_state_quest_refs`, per-character `quest_state_characters` rows, and per-quest `quest_state_quests` rows so operators can inspect candidate quest-state content without fetching the full bundle. They also include the authored `quest_flag` trigger catalog (`quest_flag_trigger_count`, `quest_flag_triggers`) and actor route rows (`quest_flag_route_count`, `quest_flag_routes`) so local QA can distinguish portable seed state from the visible NPCs that can advance or clear selected-character flags. Broad import-preview responses expose matching `deltas.quest_flag_triggers`, `deltas.quest_flag_routes`, and map-local `deltas.maps[].quest_flag_routes` rows. `gamed` exposes loopback-only focused readers for authored quest triggers through `GET /local/content-bundle/quest-flag-triggers/{kind}/{ref}`, authored quest-trigger placement through `GET /local/content-bundle/quest-flag-routes/{actor_name}`, map-local placement through `GET /local/content-bundle/maps/{map_index}/quest-flag-routes`, no-mutation exact trigger deltas through `POST /local/content-bundle/import-preview/quest-flag-triggers/{kind}/{ref}`, and no-mutation route deltas through `POST /local/content-bundle/import-preview/quest-flag-routes/{actor_name}`. The `{kind}` segment must be `quest_flag`; the `{ref}` segment must be a path-safe `interactionstore` ref such as `quest:first_steps`. The quest count is derived from the canonical distinct `quest_ref` set, not from static actor metadata or future quest definitions. `gamed` also exposes loopback-only read-only focused readers for the live exported bundle:

- `GET /local/content-bundle/quest-state` returns a compact quest-state overview with the quest-state counts, quest refs, per-character rows, and per-quest rows from the live exported bundle summary.
- `GET /local/content-bundle/quest-state/characters/{character}` returns one exact `quest_state_characters[]` summary row.
- `GET /local/content-bundle/quest-state/quests/{quest_ref}` returns one exact `quest_state_quests[]` summary row keyed by a valid `quest:<name>` ref.
- `GET /local/content-bundle/quest-state/flags/{character}/{quest_ref}/{flag}` returns one exact persisted non-zero quest flag row from the exported bundle summary.
- `POST /local/content-bundle/import-preview/quest-state` runs the ordinary content-bundle import preview for a candidate bundle, then returns a compact quest-state-only import-preview object with current/candidate quest-state overviews plus the `flag_count`, `character_count`, `quest_count`, and `flags[]` deltas.
- `POST /local/content-bundle/import-preview/quest-state/characters/{character}` runs the ordinary content-bundle import preview for a candidate bundle, then returns only changed `quest_state_flags[]` delta rows for that character.
- `POST /local/content-bundle/import-preview/quest-state/quests/{quest_ref}` runs the ordinary content-bundle import preview for a candidate bundle, then returns only changed `quest_state_flags[]` delta rows for that quest ref.
- `POST /local/content-bundle/import-preview/quest-state/flags/{character}/{quest_ref}/{flag}` runs the ordinary content-bundle import preview for a candidate bundle, then returns only the exact `quest_state_flags[]` delta for that flag identity.

The overview reader is a no-argument focused projection for local QA when the full authored content-bundle summary is too broad but character/quest/flag-specific endpoints are too narrow. The per-flag reader validates the same path-safe `character`, `quest_ref`, and lower-snake flag-name identities as the store primitive, then returns the canonical row shape (`character`, `quest_ref`, `name`, `value`) or `404` when that exact flag is absent. The per-quest summary groups the already-canonical quest-state rows by `quest_ref`, preserves deterministic character ordering within each quest, and includes each matching character's deterministic flag summaries. These are operator inspection shapes only; they do not define quest objectives or make quest refs executable.

The focused import-preview readers are no-mutation projections over the existing `POST /local/content-bundle/import-preview` result. They use the same candidate-bundle decoding, canonicalization, validation, and live-export comparison as the broad preview endpoint, but return either a compact quest-state preview object, a character-scoped list of `QuestStateDelta` rows, a quest-scoped list of `QuestStateDelta` rows, or one exact `QuestStateDelta` row instead of the full preview. The compact quest-state preview intentionally omits static actors, shops, rewards, items, and interaction-definition deltas, but keeps both current and candidate quest-state overviews so operators can audit the authored flag-state change without manual JSON filtering. These endpoints return `404` when the candidate import has no quest-state added/removed/changed flag deltas, or when the requested character, quest, or flag has no matching added/removed/changed delta; `400` for malformed path identities or candidate bundles; `403` for non-loopback callers; and `405` for wrong methods. This lets local QA inspect authored quest-state changes at the right granularity without manually filtering a full import-preview response.

The content-bundle boundary is still authored-content plumbing only: it does not define quest objectives, transition triggers, NPC dialogs, rewards, or client quest packets.

## Spawn-group kill-quest credit

Authored `spawn_groups`, authoring-only `regen_spawns`, and authoring-only `drop_tables` may optionally carry one kill-quest credit descriptor that reuses the same compare-and-set primitive after an accepted non-player death edge:

```json
{
  "ref": "practice.qa_kill_quest_mob",
  "name": "QAKillQuestMob",
  "map_index": 1,
  "x": 469800,
  "y": 964200,
  "race_num": 20350,
  "combat_profile": "training_dummy",
  "reward_quest_ref": "quest:first_steps",
  "reward_quest_flag": "killed_qa_mob",
  "reward_quest_from": 0,
  "reward_quest_to": 1,
  "reward_quest_text": "Quest updated: first_steps.killed_qa_mob = 1."
}
```

Owned rules:

- absent / all-empty kill-quest fields remain valid and are a no-op
- when any kill-quest field is present, `reward_quest_ref`, `reward_quest_flag`, and non-blank `reward_quest_text` are all required
- `reward_quest_ref` and `reward_quest_flag` use the same identity rules as the standalone quest-state store
- `reward_quest_from` / `reward_quest_to` must differ when credit is present; omitted `reward_quest_from` means the ordinary absent-current-value / `0` transition case
- kill-quest credit is spawn-group-only at runtime: standalone static actors without `spawn_group_ref` may not carry these fields
- authoring-only `drop_tables` may carry the same kill-quest fields; canonicalization expands them onto the referencing spawn group / regen spawn together with EXP/gold/drop-vnum channels, then strips `drop_tables` and `reward_drop_table_ref`
- a spawn group that already authors any kill-quest field may not also expand a table that carries kill-quest credit; that conflict fails closed before import
- the credit lives beside, not inside, the EXP/gold/drop death-reward descriptor; empty combat rewards may still apply kill-quest credit
- on the accepted killing hit, after death/clear and any independent EXP/gold/drop reward handling, the selected killer session attempts one `ApplyTransition` for `(reward_quest_ref, reward_quest_flag, reward_quest_from, reward_quest_to)`
- when the transition applies, the killer receives one self-only `CHAT_TYPE_INFO` frame with `reward_quest_text`
- `current_value_mismatch` and other fail-closed transition results stay silent for this combat path: no quest chat is emitted and combat rewards are not rolled back
- the narrow checked-in QA example remains `docs/examples/bootstrap-kill-quest-credit-bundle.json`; the combined NPC service fixture `docs/examples/bootstrap-npc-service-bundle.json` now also authors the same credit fields on `practice.qa_reward_mob` and closes that credit with `QuestHunter` / `quest:first_steps_kill_turnin`; `docs/examples/bootstrap-drop-table-authoring-bundle.json` shows the same kill-quest fields authored once on a shared `drop_tables` row and expanded into the referencing spawn group before runtime import

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
- static-actor/NPC interaction hooks that call `/local/quest-state/transition` or the store transition primitive automatically beyond the owned `quest_flag` interaction kind and spawn-group kill-quest credit seam.

## Success definition

The current repository can now say:

- there is a tested, deterministic file-backed quest-flag primitive,
- one single-flag transition can initialize, advance, or clear a flag only when the caller-provided current value matches,
- `gamed` exposes a loopback-only `POST /local/quest-state/transition-preview` harness for dry-running the exact same primitive without writing the committed snapshot,
- `gamed` exposes a loopback-only `POST /local/quest-state/transition` harness for applying that primitive without inventing client quest packets or NPC dialog semantics,
- `gamed` exposes loopback-only readback harnesses for inspecting the whole committed quest-state overview, one persisted character flag set, all persisted flags for one quest ref, or one exact flag row without mutating quest state,
- content-bundle import/export now includes the configured quest-state snapshot and exposes focused `GET /local/content-bundle/quest-state`, `GET /local/content-bundle/quest-state/characters/{character}`, `GET /local/content-bundle/quest-state/quests/{quest_ref}`, `GET /local/content-bundle/quest-state/flags/{character}/{quest_ref}/{flag}`, `GET /local/content-bundle/quest-flag-triggers/{kind}/{ref}`, `POST /local/content-bundle/import-preview/quest-flag-triggers/{kind}/{ref}`, `POST /local/content-bundle/import-preview/quest-state/characters/{character}`, `POST /local/content-bundle/import-preview/quest-state/quests/{quest_ref}`, and `POST /local/content-bundle/import-preview/quest-state/flags/{character}/{quest_ref}/{flag}` readers for bundle-summary rows and scoped import-preview quest/quest-trigger deltas,
- the same store can be validated and cleaned of owned crash-temp files without mutating committed quest flags,
- bad identities, duplicate rows, malformed JSON, symlinked committed snapshots, symlinked crash-temp candidates, and mismatched current values fail closed,
- the first owned combat-adjacent content trigger can apply that same primitive for the selected killer after an accepted spawn-backed death edge when the spawn group authors kill-quest credit fields,
- the combined QA fixture can close that kill credit through an authored `quest_flag` turn-in NPC without inventing a second quest runtime,
- broader client-visible quest runtime remains future work.
