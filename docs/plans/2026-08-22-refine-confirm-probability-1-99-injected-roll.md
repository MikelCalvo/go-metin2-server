# Refine Confirm Probability 1..99 Injected Roll — 2026-08-22

## Objective

Freeze the first deterministic/testable confirm path for remembered `refine_info.probability` values in `1..99`: one injected roll in `1..100` decides between the already-owned success burst and the already-owned destroy + `RefineFailed <type>` burst, without inventing keep-grade, catalysts, or production RNG opacity in tests.

## Contract to own

1. Preview may still open for authored `refine_info.probability` in `0..100` (already owned).
2. Matching confirm while a remembered dialog has `probability` in `1..99` may mutate only when the same busy / zero-HP / source-identity / template / gold / material / result-template guards already owned by the success and destroy-failure paths pass.
3. Confirm supplies one roll integer in `1..100` through a narrow player helper / runtime seam (session wiring may later draw that roll from a default RNG; tests inject fixed rolls). Rolls outside `1..100` fail closed with no frames/mutation and leave the open dialog untouched.
4. Outcome:
   - if `roll <= remembered.probability` → apply the already-owned success mutation and emit the owned success burst ending in `CHAT_TYPE_COMMAND` `RefineSuceeded <type>`;
   - if `roll > remembered.probability` → apply the already-owned destroy-failure mutation and emit the owned destroy burst ending in `CHAT_TYPE_COMMAND` `RefineFailed <type>`.
5. `probability = 0` and `probability = 100` keep their already-owned deterministic paths and do not require a roll.
6. Keep-grade / downgrade / safe-refine / catalyst / guild / money-only variants remain deferred; failed `1..99` rolls use whole-source destroy exactly like `probability = 0`.

## What this is not yet

- opaque production-only RNG without a test injection seam
- keep-grade / downgrade / safe-refine failure variants
- catalyst / scroll / hyuniron / musin / black-dragon semantics
- peer-facing refine notifications

## TDD and validation (implementation follow-up)

Focused coverage:

- `go test ./internal/player -run 'ApplyRefineWithRoll|ApplyRefineRoll' -count=1`
- `go test ./internal/minimal -run 'ItemRefineConfirmAfterPreviewProbability[0-9]+Roll' -count=1`
- `gofmt` on touched Go files
- `git diff --check`

## Follow-up options

1. Implement the GREEN path after this contract freeze (player roll helper + minimal confirm wiring + session proofs for success and destroy rolls).
2. Keep keep-grade / catalyst variants deferred.
3. Optional later: document the default production RNG source once operators pick one.
