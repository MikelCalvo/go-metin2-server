# Runtime boundary: equipment eligibility extraction

## Scope

This small compatibility-preserving slice extracts the pure equipment eligibility
rule that `internal/minimal/factory.go` previously assembled from runtime state:

- authored template validity and matching authored wear slot;
- character job, sex/race parity, empire, and minimum-level restrictions; and
- transfer-guard flags (`anti_stack`, `anti_get`, `anti_drop`, `anti_give`, and
  `anti_sell`) that make wearing the item ineligible.

The new `internal/equipment` package accepts only an `itemstore.Template`, an
explicit `equipment.Subject` value, and the requested `inventory.EquipmentSlot`.
It has no runtime, persistence, protocol, session, or factory dependency.

`player.Runtime` translates its persisted immutable character fields into that
explicit subject. The existing public `CanUseTemplate`, template-backed equip,
and occupied-wear replacement paths delegate to the pure rule. The minimal
factory's `runtimeTemplateAllowsEquip` now delegates too, retaining its existing
nil-runtime guard and packet/session orchestration.

## Behavior preserved

No packet encoding, mutation order, persistence behavior, rejection messaging,
or item movement was moved. Invalid templates, mismatched/invalid slots,
character restrictions, and each transfer guard remain fail-closed. The existing
factory helper remains the boundary used by packet and slash flows.

## Evidence

Focused verification from this worktree:

```text
go test ./internal/equipment ./internal/player
ok github.com/MikelCalvo/go-metin2-server/internal/equipment
ok github.com/MikelCalvo/go-metin2-server/internal/player

go test ./internal/minimal -run 'TestNewGameSessionFactoryItemMovePacketEquipsInventoryItem|TestNewGameSessionFactorySlashEquipRejectsTemplateAntiGetWithoutMutation|TestNewGameSessionFactoryItemMovePacketRejectsTemplateMismatchedEquipSlotWithoutMutation' -count=1
ok github.com/MikelCalvo/go-metin2-server/internal/minimal 0.106s
```

The new table tests cover allowed eligibility plus invalid templates, all four
job anti-flags, sex/race parity flags, empire restriction, minimum level, wrong
and invalid destination slots, and each transfer guard.

A combined `go test ./internal/equipment ./internal/player ./internal/minimal`
was started with the requested focused-test timeout budget, but this repository's
full `internal/minimal` package did not complete before the runner's enforced
300-second cap and produced no test failure output. The targeted minimal
regression tests above completed successfully.

## Deliberately out of scope

This is not a framework introduction or a runtime redesign. No mass file moves,
service changes, build, commit, push, or background process action occurred.
Further candidates should be similarly extracted only when their dependencies
can be made explicit and their current behavior can be pinned with focused tests.
