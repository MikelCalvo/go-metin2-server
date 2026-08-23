# Portable Legacy Omit-Attack-Value Canonicalize — 2026-08-23

## Objective

Close the authored combat-profile formula seam asymmetry: runtime registration
already expands omitted `attack_value` from legacy `damage_per_normal_attack +
defense_value` (and fails closed on `uint16` overflow), but portable
`combat_profiles` content-bundle canonicalization and file-backed static-actor
snapshots still required a non-zero `attack_value`.

## Contract frozen by this slice

1. Portable `combat_profiles` snapshots that omit `attack_value` but supply
   legacy `damage_per_normal_attack` expand
   `attack_value = damage_per_normal_attack + defense_value` during content-bundle
   canonicalization and static-actor snapshot normalize/save.
2. That same omit-attack path fails closed when the sum cannot fit `uint16`.
3. Formula-first omit-`damage_per_normal_attack` behavior remains unchanged.
4. Spec wording in `combat-normal-attack-bootstrap.md` and
   `content-spawn-groups-bootstrap.md` now names registration, content-bundle,
   and static-actor snapshot seams together for both directions.

## Focused coverage

- `TestCanonicalizeExpandsLegacyDamageCombatProfileOmittedAttackValueFromDefense`
- `TestCanonicalizeRejectsLegacyDamageCombatProfileAttackValueDefenseOverflow`
- `TestFileStoreSaveLoadExpandsLegacyDamageCombatProfileOmittedAttackValue`
- `TestFileStoreRejectsLegacyDamageCombatProfileAttackValueDefenseOverflow`

```bash
go test ./internal/contentbundle ./internal/staticstore -run 'LegacyDamageCombatProfile|FormulaOnlyCombatProfile' -count=1
```

## What this is not yet

- weighted / random loot tables
- full legacy combat math beyond `max(1, attack_value - defense_value)`
- remapping proximity suppress across content-bundle replacement or daemon restart
- skill / ranged / PvP runtime policy
