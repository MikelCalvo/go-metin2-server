package worldruntime

import (
	"testing"
	"time"
)

func TestEffectiveStaticActorSpawnReactionDelayDefaultsToBootstrap(t *testing.T) {
	if got := EffectiveStaticActorSpawnReactionDelay(StaticActorCombatProfilePracticeMob); got != DefaultSpawnReactionDelay {
		t.Fatalf("expected practice_mob effective reaction delay %v, got %v", DefaultSpawnReactionDelay, got)
	}
	if got := EffectiveStaticActorSpawnReactionDelay(StaticActorCombatProfileTrainingDummy); got != DefaultSpawnReactionDelay {
		t.Fatalf("expected training_dummy effective reaction delay %v, got %v", DefaultSpawnReactionDelay, got)
	}
	if got := EffectiveStaticActorSpawnReactionDelay("missing_profile"); got != DefaultSpawnReactionDelay {
		t.Fatalf("expected unknown profile effective reaction delay %v, got %v", DefaultSpawnReactionDelay, got)
	}
	if got := EffectiveStaticActorSpawnReactionDelayFromDefaults(StaticActorCombatProfileDefaults{}); got != DefaultSpawnReactionDelay {
		t.Fatalf("expected omitted defaults reaction delay %v, got %v", DefaultSpawnReactionDelay, got)
	}
}

func TestRegisterStaticActorCombatProfileHonorsAuthoredReactionDelay(t *testing.T) {
	const profile = "practice_reaction_delay_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:         24,
		AttackValue:   8,
		DefenseValue:  2,
		RespawnDelay:  PracticeMobBootstrapRespawnDelay,
		ReactionDelay: 2 * time.Second,
	}) {
		t.Fatalf("expected %q profile registration with authored reaction delay to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered profile defaults to resolve")
	}
	if defaults.ReactionDelay != 2*time.Second {
		t.Fatalf("expected registered reaction delay 2s, got %+v", defaults)
	}
	if got := EffectiveStaticActorSpawnReactionDelay(profile); got != 2*time.Second {
		t.Fatalf("expected effective reaction delay 2s, got %v", got)
	}
	snapshot, ok := staticActorCombatProfileSnapshotByName(StaticActorCombatProfileSnapshots(), profile)
	if !ok {
		t.Fatalf("expected registered profile snapshot for %q", profile)
	}
	if snapshot.ReactionDelayMs != 2000 {
		t.Fatalf("expected snapshot to expose authored reaction_delay_ms 2000, got %+v", snapshot)
	}

	actor := StaticEntity{
		Entity:        Entity{ID: 9012},
		SpawnGroupRef: "practice.reaction_delay_wolf",
		CombatProfile: profile,
		Position:      NewPosition(1, 1700, 2800),
		SpawnHome:     NewPosition(1, 1700, 2800),
	}
	if got := EffectiveStaticActorSpawnReactionDelayForActor(actor); got != 2*time.Second {
		t.Fatalf("expected actor effective reaction delay 2s, got %v", got)
	}
}

func TestRegisterStaticActorCombatProfileRejectsReactionDelayBelowBootstrapFloor(t *testing.T) {
	const profile = "practice_reaction_delay_too_fast_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:         24,
		AttackValue:   8,
		DefenseValue:  2,
		RespawnDelay:  PracticeMobBootstrapRespawnDelay,
		ReactionDelay: 249 * time.Millisecond,
	}) {
		t.Fatalf("expected %q profile registration with reaction delay < 250ms to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected too-fast reaction profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileReactionDelay(249 * time.Millisecond) {
		t.Fatalf("expected reaction delay 249ms to be invalid against bootstrap floor")
	}
}

func TestRegisterStaticActorCombatProfileRejectsReactionDelayAboveBootstrapCap(t *testing.T) {
	const profile = "practice_reaction_delay_too_slow_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:         24,
		AttackValue:   8,
		DefenseValue:  2,
		RespawnDelay:  PracticeMobBootstrapRespawnDelay,
		ReactionDelay: 61 * time.Second,
	}) {
		t.Fatalf("expected %q profile registration with reaction delay above 60s to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected over-cap reaction profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileReactionDelay(61 * time.Second) {
		t.Fatalf("expected reaction delay 61s to be invalid against bootstrap upper bound")
	}
}

func TestRegisterStaticActorCombatProfileRejectsNegativeReactionDelay(t *testing.T) {
	const profile = "practice_negative_reaction_delay_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:         24,
		AttackValue:   8,
		DefenseValue:  2,
		RespawnDelay:  PracticeMobBootstrapRespawnDelay,
		ReactionDelay: -time.Millisecond,
	}) {
		t.Fatalf("expected %q profile registration with negative reaction delay to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected negative-reaction profile %q not to become valid", profile)
	}
	if ValidStaticActorCombatProfileReactionDelay(-time.Millisecond) {
		t.Fatalf("expected negative reaction delay to be invalid")
	}
}
