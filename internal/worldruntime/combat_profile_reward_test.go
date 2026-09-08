package worldruntime

import (
	"testing"
	"time"
)

func TestStaticActorCombatProfileSnapshotRoundTripsDeathReward(t *testing.T) {
	const profile = "practice_profile_reward_round_trip"
	defaults := StaticActorCombatProfileDefaults{
		MaxHP:                 12,
		DamagePerNormalAttack: 3,
		AttackValue:           9,
		DefenseValue:          6,
		Level:                 5,
		Rank:                  1,
		RespawnDelay:          1500 * time.Millisecond,
		DeathReward: StaticActorDeathReward{
			Experience: 75,
			Gold:       60,
			DropVnums:  []uint32{27001, 27002},
		},
	}
	if !RegisterStaticActorCombatProfile(profile, defaults) {
		t.Fatalf("expected %q profile registration with an authored death reward to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	var snapshot StaticActorCombatProfileSnapshot
	found := false
	for _, candidate := range StaticActorCombatProfileSnapshots() {
		if candidate.Profile != profile {
			continue
		}
		snapshot = candidate
		found = true
		break
	}
	if !found {
		t.Fatalf("expected registered profile %q in exported snapshots", profile)
	}
	if got := snapshot.DeathReward; got.Experience != defaults.DeathReward.Experience || got.Gold != defaults.DeathReward.Gold || len(got.DropVnums) != len(defaults.DeathReward.DropVnums) {
		t.Fatalf("expected exported profile death reward %#v, got %#v", defaults.DeathReward, got)
	}
	for index, wantVnum := range defaults.DeathReward.DropVnums {
		if snapshot.DeathReward.DropVnums[index] != wantVnum {
			t.Fatalf("expected exported profile drop vnum %d at index %d, got %#v", wantVnum, index, snapshot.DeathReward.DropVnums)
		}
	}
}

func TestStaticActorCombatProfileSnapshotRoundTripClonesDeathRewardDropVnums(t *testing.T) {
	const profile = "practice_profile_reward_snapshot_clone"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 12,
		DamagePerNormalAttack: 3,
		AttackValue:           9,
		DefenseValue:          6,
		Level:                 5,
		Rank:                  1,
		RespawnDelay:          time.Second,
		DeathReward: StaticActorDeathReward{
			DropVnums: []uint32{27001},
		},
	}) {
		t.Fatalf("expected %q profile registration to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	var exported StaticActorCombatProfileSnapshot
	for _, candidate := range StaticActorCombatProfileSnapshots() {
		if candidate.Profile == profile {
			exported = candidate
			break
		}
	}
	if len(exported.DeathReward.DropVnums) != 1 {
		t.Fatalf("expected exported profile reward drop vnum, got %#v", exported.DeathReward)
	}
	defaults := StaticActorCombatProfileDefaultsFromSnapshot(exported)
	exported.DeathReward.DropVnums[0] = 29999
	if len(defaults.DeathReward.DropVnums) != 1 || defaults.DeathReward.DropVnums[0] != 27001 {
		t.Fatalf("expected snapshot-to-default conversion to clone reward drops, got %#v", defaults.DeathReward)
	}

	for _, candidate := range StaticActorCombatProfileSnapshots() {
		if candidate.Profile == profile && len(candidate.DeathReward.DropVnums) == 1 && candidate.DeathReward.DropVnums[0] == 27001 {
			return
		}
	}
	t.Fatalf("expected exported profile mutation not to alter registered death reward")
}

func TestStaticActorCombatProfileSnapshotRoundTripPreservesRetaliationAndChaseFields(t *testing.T) {
	const profile = "practice_profile_formula_round_trip"
	defaults := StaticActorCombatProfileDefaults{
		MaxHP:                 12,
		DamagePerNormalAttack: 3,
		AttackValue:           9,
		DefenseValue:          6,
		Level:                 5,
		Rank:                  1,
		RespawnDelay:          1500 * time.Millisecond,
		AggroRadius:           320,
		LeashRadius:           500,
		ChaseDelay:            2 * time.Second,
		ReturnDelay:           1500 * time.Millisecond,
		HomewardDelay:         2 * time.Second,
		MaxStep:               50,
		ReactionDelay:         2 * time.Second,
		RetaliationPointDelta: -2,
	}
	if !RegisterStaticActorCombatProfile(profile, defaults) {
		t.Fatalf("expected %q formula profile registration to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	var snapshot StaticActorCombatProfileSnapshot
	for _, candidate := range StaticActorCombatProfileSnapshots() {
		if candidate.Profile == profile {
			snapshot = candidate
			break
		}
	}
	if snapshot.Profile == "" {
		t.Fatalf("expected formula profile %q in exported snapshots", profile)
	}

	UnregisterStaticActorCombatProfileForTest(profile)
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaultsFromSnapshot(snapshot)) {
		t.Fatalf("expected exported formula profile snapshot %q to re-register", profile)
	}

	got, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok || got.MaxHP != defaults.MaxHP || got.DamagePerNormalAttack != defaults.DamagePerNormalAttack || got.AttackValue != defaults.AttackValue || got.DefenseValue != defaults.DefenseValue || got.Level != defaults.Level || got.Rank != defaults.Rank || got.RespawnDelay != defaults.RespawnDelay || got.AggroRadius != defaults.AggroRadius || got.LeashRadius != defaults.LeashRadius || got.ChaseDelay != defaults.ChaseDelay || got.ReturnDelay != defaults.ReturnDelay || got.HomewardDelay != defaults.HomewardDelay || got.MaxStep != defaults.MaxStep || got.ReactionDelay != defaults.ReactionDelay || got.RetaliationPointDelta != defaults.RetaliationPointDelta || got.DeathReward.Experience != defaults.DeathReward.Experience || got.DeathReward.Gold != defaults.DeathReward.Gold || len(got.DeathReward.DropVnums) != len(defaults.DeathReward.DropVnums) {
		t.Fatalf("expected snapshot re-registration to preserve formula defaults %#v, got %#v ok=%v", defaults, got, ok)
	}
	for index, wantVnum := range defaults.DeathReward.DropVnums {
		if got.DeathReward.DropVnums[index] != wantVnum {
			t.Fatalf("expected re-registered reward drop vnum %d at index %d, got %#v", wantVnum, index, got.DeathReward.DropVnums)
		}
	}
}
