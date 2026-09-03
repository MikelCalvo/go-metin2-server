# Refine success / fail_result_vnum downgrade preserve instance sockets/attributes — 2026-09-03

## Objective

Freeze the next Track C honesty seam after destination-wins compatible
stack-merge landed: confirm-after-preview refine paths that keep the source
carried identity and only rewrite `vnum` must preserve presence-aware
per-instance sockets and attributes through the result cell, result `ITEM_SET`,
and account snapshot.

Owned paths in scope:

- `probability = 100` success (`ApplyRefineSuccess`) — replace source `vnum`
  with `result_vnum` in the same cell while preserving instance id
- `probability` in `1..99` + authored `fail_result_vnum` downgrade
  (`ApplyRefineDowngradeFailure`) — replace source `vnum` with
  `fail_result_vnum` in the same cell while preserving instance id
- injected-roll success / downgrade outcomes that reuse those helpers

Today those helpers clone live inventory (including independent
sockets/attributes) and only rewrite `Vnum`. Spec language still names this as
"preserving the existing instance id" without an explicit presence contract, so
a later RED could invent clear-on-success / template-reset rules mid-slice.

## Why docs-first

This is priority-queue #1 (item-state consistency) on the owned refine confirm
seam. Opening RED without freezing:

- that success / downgrade keep source presence (active / explicit-zero / omit),
- that result `ITEM_SET` projects preserved instance presence through ordinary
  `EffectiveSockets` / `EffectiveAttributes`,
- that destroy / keep-on-fail paths stay out of this contract,

would invent policy mid-implementation. Keep refine catalysts, mall, and party
ownership notices deferred.

## Contract to freeze (before RED)

1. **In-place vnum rewrite preserves presence**: when refine success or
   `fail_result_vnum` downgrade replaces the source carried `vnum` in the same
   cell while keeping the existing instance id / count / cell:
   - if the pre-confirm source `HasSockets()`, those sockets remain on the
     result cell (including explicit `{0,0,0}`);
   - if the pre-confirm source `HasAttributes()`, those attributes remain on the
     result cell (including explicit all-zero / type-zero);
   - if the pre-confirm source omits sockets and/or attributes, the result cell
     likewise omits them (later encode may fall back to the **result**
     template's display sockets/attributes through already-owned
     `EffectiveSockets` / `EffectiveAttributes`).
2. **Wire honesty**: confirm-burst result `ITEM_SET` must project the preserved
   instance presence the same way ordinary inventory encode already prefers
   instance over template when present.
3. **Persistence**: the selected-character account snapshot after successful
   confirm must round-trip those presence-aware fields on the rewritten cell.
4. **Independence from material consume**: material `ITEM_UPDATE` / `ITEM_DEL`
   paths stay count-only / removal and must not clear or overwrite the source
   cell's presence as a side effect of planning materials.
5. **Out of scope here**:
   - whole-source destroy (`probability = 0` / failed injected roll without
     keep/downgrade) — source cell is removed
   - `keep_on_fail` — source cell is unchanged in place (already preserves by
     non-mutation of the source item)
   - scroll / hyuniron / musin / black-dragon catalyst consumption
   - inventing socket/attribute gameplay formulas or clearing presence on
     success because the result template authors different display metadata

## Proof shape (for later RED → GREEN)

1. Unit: seed a refineable source cell with authoritative instance presence
   (active + explicit-zero + omitted cases) → `ApplyRefineSuccess` /
   `ApplyRefineDowngradeFailure` → result item keeps the same presence pointers
   independently cloned from the pre-mutation source; omitted stays omitted.
2. Session: preview + matching confirm for `probability = 100` (and one
   `fail_result_vnum` injected-fail twin) → confirm-burst result `ITEM_SET` and
   persisted inventory carry preserved instance sockets/attributes; omitted
   regression keeps omit→result-template encode fallback.
3. Negatives: destroy / keep-on-fail / busy / insufficient gold-materials stay
   as already owned; catalysts remain deferred.

## Likely files to change (later GREEN, not this freeze)

- focused proofs under `internal/player` / `internal/minimal`
- `spec/protocol/item-refine-bootstrap.md`
- `docs/qa/manual-client-checklist.md`
- `docs/plans/2026-08-08-playable-vertical-roadmap.md`

## Validation (later GREEN)

```bash
go test ./internal/player -run 'ApplyRefine(Success|Downgrade).*Instance|PreservesInstance' -count=1
go test ./internal/minimal -run 'RefineConfirm.*PreservesInstance|FailResultVnum.*PreservesInstance' -count=1
go test ./internal/minimal ./internal/player -count=1
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
```

## Status

GREEN on `lane/items`: in-place refine success (`probability = 100`) and
`fail_result_vnum` downgrade preserve presence-aware instance sockets/attributes
(including explicit zero; omit→omit with result-template encode fallback) through
result `ITEM_SET` and the account snapshot
(`TestRuntimeApplyRefineSuccessPreservesInstanceSocketsAndAttributes`,
`TestRuntimeApplyRefineDowngradeFailurePreservesInstanceSocketsAndAttributes`,
`TestGameRuntimeItemRefineConfirmAfterPreviewProbability100PreservesInstanceSocketsAndAttributes`,
`TestGameRuntimeItemRefineConfirmAfterPreviewFailResultVnumPreservesInstanceSocketsAndAttributes`).
Destroy / keep-on-fail / catalysts / mall / party ownership notices remain
deferred.
