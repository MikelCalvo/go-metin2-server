package worldruntime

import (
	"testing"
	"time"
)

func TestEffectiveStaticActorSpawnHomewardDelayDefaultsToBootstrap(t *testing.T) {
	if got := EffectiveStaticActorSpawnHomewardDelay(StaticActorCombatProfilePracticeMob); got != DefaultSpawnHomewardDelay {
		t.Fatalf("expected practice_mob effective homeward delay %v, got %v", DefaultSpawnHomewardDelay, got)
	}
	if got := EffectiveStaticActorSpawnHomewardDelay(StaticActorCombatProfileTrainingDummy); got != DefaultSpawnHomewardDelay {
		t.Fatalf("expected training_dummy effective homeward delay %v, got %v", DefaultSpawnHomewardDelay, got)
	}
	if got := EffectiveStaticActorSpawnHomewardDelay("missing_profile"); got != DefaultSpawnHomewardDelay {
		t.Fatalf("expected unknown profile effective homeward delay %v, got %v", DefaultSpawnHomewardDelay, got)
	}
	if got := EffectiveStaticActorSpawnHomewardDelayFromDefaults(StaticActorCombatProfileDefaults{}); got != DefaultSpawnHomewardDelay {
		t.Fatalf("expected omitted defaults homeward delay %v, got %v", DefaultSpawnHomewardDelay, got)
	}
}

func TestRegisterStaticActorCombatProfileHonorsAuthoredHomewardDelay(t *testing.T) {
	const profile = "practice_homeward_delay_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:         24,
		AttackValue:   8,
		DefenseValue:  2,
		RespawnDelay:  PracticeMobBootstrapRespawnDelay,
		HomewardDelay: 2 * time.Second,
	}) {
		t.Fatalf("expected %q profile registration with authored homeward delay to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered profile defaults to resolve")
	}
	if defaults.HomewardDelay != 2*time.Second {
		t.Fatalf("expected registered homeward delay 2s, got %+v", defaults)
	}
	if got := EffectiveStaticActorSpawnHomewardDelay(profile); got != 2*time.Second {
		t.Fatalf("expected effective homeward delay 2s, got %v", got)
	}
	snapshot, ok := staticActorCombatProfileSnapshotByName(StaticActorCombatProfileSnapshots(), profile)
	if !ok {
		t.Fatalf("expected registered profile snapshot for %q", profile)
	}
	if snapshot.HomewardDelayMs != 2000 {
		t.Fatalf("expected snapshot to expose authored homeward_delay_ms 2000, got %+v", snapshot)
	}

	actor := StaticEntity{
		Entity:        Entity{ID: 9003},
		SpawnGroupRef: "practice.homeward_delay_wolf",
		CombatProfile: profile,
		Position:      NewPosition(1, 1700, 2800),
		SpawnHome:     NewPosition(1, 1700, 2800),
	}
	if got := EffectiveStaticActorSpawnHomewardDelayForActor(actor); got != 2*time.Second {
		t.Fatalf("expected actor effective homeward delay 2s, got %v", got)
	}
}

func TestRegisterStaticActorCombatProfileRejectsHomewardDelayBelowBootstrapFloor(t *testing.T) {
	const profile = "practice_homeward_delay_too_fast_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:         24,
		AttackValue:   8,
		DefenseValue:  2,
		RespawnDelay:  PracticeMobBootstrapRespawnDelay,
		HomewardDelay: 249 * time.Millisecond,
	}) {
		t.Fatalf("expected %q profile registration with homeward delay < 250ms to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected too-fast homeward profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileHomewardDelay(249 * time.Millisecond) {
		t.Fatalf("expected homeward delay 249ms to be invalid against bootstrap floor")
	}
}

func TestRegisterStaticActorCombatProfileRejectsHomewardDelayAboveBootstrapCap(t *testing.T) {
	const profile = "practice_homeward_delay_too_slow_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:         24,
		AttackValue:   8,
		DefenseValue:  2,
		RespawnDelay:  PracticeMobBootstrapRespawnDelay,
		HomewardDelay: 61 * time.Second,
	}) {
		t.Fatalf("expected %q profile registration with homeward delay above 60s to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected over-cap homeward profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileHomewardDelay(61 * time.Second) {
		t.Fatalf("expected homeward delay 61s to be invalid against bootstrap upper bound")
	}
}

func TestRegisterStaticActorCombatProfileRejectsNegativeHomewardDelay(t *testing.T) {
	const profile = "practice_negative_homeward_delay_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:         24,
		AttackValue:   8,
		DefenseValue:  2,
		RespawnDelay:  PracticeMobBootstrapRespawnDelay,
		HomewardDelay: -time.Millisecond,
	}) {
		t.Fatalf("expected %q profile registration with negative homeward delay to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected negative-homeward profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileHomewardDelay(-time.Millisecond) {
		t.Fatalf("expected negative homeward delay to be invalid")
	}
}
