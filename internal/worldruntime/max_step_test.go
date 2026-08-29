package worldruntime

import (
	"testing"
)

func TestEffectiveStaticActorSpawnMaxStepDefaultsToBootstrap(t *testing.T) {
	if got := EffectiveStaticActorSpawnMaxStep(StaticActorCombatProfilePracticeMob); got != DefaultSpawnMaxStep {
		t.Fatalf("expected practice_mob effective max step %d, got %d", DefaultSpawnMaxStep, got)
	}
	if got := EffectiveStaticActorSpawnMaxStep(StaticActorCombatProfileTrainingDummy); got != DefaultSpawnMaxStep {
		t.Fatalf("expected training_dummy effective max step %d, got %d", DefaultSpawnMaxStep, got)
	}
	if got := EffectiveStaticActorSpawnMaxStep("missing_profile"); got != DefaultSpawnMaxStep {
		t.Fatalf("expected unknown profile effective max step %d, got %d", DefaultSpawnMaxStep, got)
	}
	if got := EffectiveStaticActorSpawnMaxStepFromDefaults(StaticActorCombatProfileDefaults{}); got != DefaultSpawnMaxStep {
		t.Fatalf("expected omitted defaults max step %d, got %d", DefaultSpawnMaxStep, got)
	}
}

func TestRegisterStaticActorCombatProfileHonorsAuthoredMaxStep(t *testing.T) {
	const profile = "practice_max_step_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		MaxStep:      50,
	}) {
		t.Fatalf("expected %q profile registration with authored max step to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered profile defaults to resolve")
	}
	if defaults.MaxStep != 50 {
		t.Fatalf("expected registered max step 50, got %+v", defaults)
	}
	if got := EffectiveStaticActorSpawnMaxStep(profile); got != 50 {
		t.Fatalf("expected effective max step 50, got %d", got)
	}
	snapshot, ok := staticActorCombatProfileSnapshotByName(StaticActorCombatProfileSnapshots(), profile)
	if !ok {
		t.Fatalf("expected registered profile snapshot for %q", profile)
	}
	if snapshot.MaxStep != 50 {
		t.Fatalf("expected snapshot to expose authored max_step 50, got %+v", snapshot)
	}

	actor := StaticEntity{
		Entity:        Entity{ID: 9004},
		SpawnGroupRef: "practice.max_step_wolf",
		CombatProfile: profile,
		Position:      NewPosition(1, 1700, 2800),
		SpawnHome:     NewPosition(1, 1700, 2800),
	}
	if got := EffectiveStaticActorSpawnMaxStepForActor(actor); got != 50 {
		t.Fatalf("expected actor effective max step 50, got %d", got)
	}
}

func TestRegisterStaticActorCombatProfileRejectsMaxStepAboveBootstrapCap(t *testing.T) {
	const profile = "practice_max_step_too_large_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		MaxStep:      1001,
	}) {
		t.Fatalf("expected %q profile registration with max step above 1000 to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected over-cap max-step profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileMaxStep(1001) {
		t.Fatalf("expected max step 1001 to be invalid against bootstrap upper bound")
	}
}

func TestRegisterStaticActorCombatProfileRejectsNegativeMaxStep(t *testing.T) {
	const profile = "practice_negative_max_step_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		MaxStep:      -1,
	}) {
		t.Fatalf("expected %q profile registration with negative max step to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected negative-max-step profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileMaxStep(-1) {
		t.Fatalf("expected negative max step to be invalid")
	}
}
