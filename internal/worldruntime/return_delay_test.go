package worldruntime

import (
	"testing"
	"time"
)

func TestEffectiveStaticActorSpawnReturnDelayDefaultsToBootstrap(t *testing.T) {
	if got := EffectiveStaticActorSpawnReturnDelay(StaticActorCombatProfilePracticeMob); got != DefaultSpawnReturnDelay {
		t.Fatalf("expected practice_mob effective return delay %v, got %v", DefaultSpawnReturnDelay, got)
	}
	if got := EffectiveStaticActorSpawnReturnDelay(StaticActorCombatProfileTrainingDummy); got != DefaultSpawnReturnDelay {
		t.Fatalf("expected training_dummy effective return delay %v, got %v", DefaultSpawnReturnDelay, got)
	}
	if got := EffectiveStaticActorSpawnReturnDelay("missing_profile"); got != DefaultSpawnReturnDelay {
		t.Fatalf("expected unknown profile effective return delay %v, got %v", DefaultSpawnReturnDelay, got)
	}
	if got := EffectiveStaticActorSpawnReturnDelayFromDefaults(StaticActorCombatProfileDefaults{}); got != DefaultSpawnReturnDelay {
		t.Fatalf("expected omitted defaults return delay %v, got %v", DefaultSpawnReturnDelay, got)
	}
}

func TestRegisterStaticActorCombatProfileHonorsAuthoredReturnDelay(t *testing.T) {
	const profile = "practice_return_delay_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		ReturnDelay:  2 * time.Second,
	}) {
		t.Fatalf("expected %q profile registration with authored return delay to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered profile defaults to resolve")
	}
	if defaults.ReturnDelay != 2*time.Second {
		t.Fatalf("expected registered return delay 2s, got %+v", defaults)
	}
	if got := EffectiveStaticActorSpawnReturnDelay(profile); got != 2*time.Second {
		t.Fatalf("expected effective return delay 2s, got %v", got)
	}
	snapshot, ok := staticActorCombatProfileSnapshotByName(StaticActorCombatProfileSnapshots(), profile)
	if !ok {
		t.Fatalf("expected registered profile snapshot for %q", profile)
	}
	if snapshot.ReturnDelayMs != 2000 {
		t.Fatalf("expected snapshot to expose authored return_delay_ms 2000, got %+v", snapshot)
	}

	actor := StaticEntity{
		Entity:        Entity{ID: 9002},
		SpawnGroupRef: "practice.return_delay_wolf",
		CombatProfile: profile,
		Position:      NewPosition(1, 1700, 2800),
		SpawnHome:     NewPosition(1, 1700, 2800),
	}
	if got := EffectiveStaticActorSpawnReturnDelayForActor(actor); got != 2*time.Second {
		t.Fatalf("expected actor effective return delay 2s, got %v", got)
	}
}

func TestRegisterStaticActorCombatProfileRejectsReturnDelayBelowBootstrapFloor(t *testing.T) {
	const profile = "practice_return_delay_too_fast_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		ReturnDelay:  249 * time.Millisecond,
	}) {
		t.Fatalf("expected %q profile registration with return delay < 250ms to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected too-fast return profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileReturnDelay(249 * time.Millisecond) {
		t.Fatalf("expected return delay 249ms to be invalid against bootstrap floor")
	}
}

func TestRegisterStaticActorCombatProfileRejectsReturnDelayAboveBootstrapCap(t *testing.T) {
	const profile = "practice_return_delay_too_slow_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		ReturnDelay:  61 * time.Second,
	}) {
		t.Fatalf("expected %q profile registration with return delay above 60s to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected over-cap return profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileReturnDelay(61 * time.Second) {
		t.Fatalf("expected return delay 61s to be invalid against bootstrap upper bound")
	}
}

func TestRegisterStaticActorCombatProfileRejectsNegativeReturnDelay(t *testing.T) {
	const profile = "practice_negative_return_delay_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		ReturnDelay:  -time.Millisecond,
	}) {
		t.Fatalf("expected %q profile registration with negative return delay to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected negative-return profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileReturnDelay(-time.Millisecond) {
		t.Fatalf("expected negative return delay to be invalid")
	}
}
