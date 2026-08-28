package worldruntime

import (
	"testing"
	"time"
)

func TestEffectiveStaticActorSpawnChaseDelayDefaultsToBootstrap(t *testing.T) {
	if got := EffectiveStaticActorSpawnChaseDelay(StaticActorCombatProfilePracticeMob); got != DefaultSpawnChaseDelay {
		t.Fatalf("expected practice_mob effective chase delay %v, got %v", DefaultSpawnChaseDelay, got)
	}
	if got := EffectiveStaticActorSpawnChaseDelay(StaticActorCombatProfileTrainingDummy); got != DefaultSpawnChaseDelay {
		t.Fatalf("expected training_dummy effective chase delay %v, got %v", DefaultSpawnChaseDelay, got)
	}
	if got := EffectiveStaticActorSpawnChaseDelay("missing_profile"); got != DefaultSpawnChaseDelay {
		t.Fatalf("expected unknown profile effective chase delay %v, got %v", DefaultSpawnChaseDelay, got)
	}
	if got := EffectiveStaticActorSpawnChaseDelayFromDefaults(StaticActorCombatProfileDefaults{}); got != DefaultSpawnChaseDelay {
		t.Fatalf("expected omitted defaults chase delay %v, got %v", DefaultSpawnChaseDelay, got)
	}
}

func TestRegisterStaticActorCombatProfileHonorsAuthoredChaseDelay(t *testing.T) {
	const profile = "practice_chase_delay_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		ChaseDelay:   2 * time.Second,
	}) {
		t.Fatalf("expected %q profile registration with authored chase delay to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered profile defaults to resolve")
	}
	if defaults.ChaseDelay != 2*time.Second {
		t.Fatalf("expected registered chase delay 2s, got %+v", defaults)
	}
	if got := EffectiveStaticActorSpawnChaseDelay(profile); got != 2*time.Second {
		t.Fatalf("expected effective chase delay 2s, got %v", got)
	}
	snapshot, ok := staticActorCombatProfileSnapshotByName(StaticActorCombatProfileSnapshots(), profile)
	if !ok {
		t.Fatalf("expected registered profile snapshot for %q", profile)
	}
	if snapshot.ChaseDelayMs != 2000 {
		t.Fatalf("expected snapshot to expose authored chase_delay_ms 2000, got %+v", snapshot)
	}

	actor := StaticEntity{
		Entity:        Entity{ID: 9001},
		SpawnGroupRef: "practice.chase_delay_wolf",
		CombatProfile: profile,
		Position:      NewPosition(1, 1700, 2800),
		SpawnHome:     NewPosition(1, 1700, 2800),
	}
	if got := EffectiveStaticActorSpawnChaseDelayForActor(actor); got != 2*time.Second {
		t.Fatalf("expected actor effective chase delay 2s, got %v", got)
	}
}

func TestRegisterStaticActorCombatProfileRejectsChaseDelayAtOrBelowRetaliationBeat(t *testing.T) {
	const profile = "practice_chase_delay_too_fast_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		ChaseDelay:   time.Second,
	}) {
		t.Fatalf("expected %q profile registration with chase delay <= 1s to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected too-fast chase profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileChaseDelay(time.Second) {
		t.Fatalf("expected chase delay 1s to be invalid against retaliation beat floor")
	}
}

func TestRegisterStaticActorCombatProfileRejectsChaseDelayAboveBootstrapCap(t *testing.T) {
	const profile = "practice_chase_delay_too_slow_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		ChaseDelay:   61 * time.Second,
	}) {
		t.Fatalf("expected %q profile registration with chase delay above 60s to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected over-cap chase profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileChaseDelay(61 * time.Second) {
		t.Fatalf("expected chase delay 61s to be invalid against bootstrap upper bound")
	}
}

func TestRegisterStaticActorCombatProfileRejectsNegativeChaseDelay(t *testing.T) {
	const profile = "practice_negative_chase_delay_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		ChaseDelay:   -time.Millisecond,
	}) {
		t.Fatalf("expected %q profile registration with negative chase delay to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected negative-chase profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileChaseDelay(-time.Millisecond) {
		t.Fatalf("expected negative chase delay to be invalid")
	}
}
