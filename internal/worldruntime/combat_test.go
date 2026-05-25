package worldruntime

import "testing"

func TestApplyBootstrapStaticActorNormalAttackDecrementsTrainingDummyCombatHP(t *testing.T) {
	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(StaticActorCombatKindTrainingDummy, TrainingDummyBootstrapMaxHP)
	if !ok {
		t.Fatal("expected bootstrap training-dummy normal attack to be supported")
	}
	if nextHP != TrainingDummyBootstrapMaxHP-TrainingDummyBootstrapDamagePerNormalAttack {
		t.Fatalf("expected training-dummy HP to decrement by one bootstrap hit, got %d", nextHP)
	}
	if hpPercent != 90 {
		t.Fatalf("expected training-dummy HP percent 90 after one bootstrap hit, got %d", hpPercent)
	}
}

func TestApplyBootstrapStaticActorNormalAttackTransitionsTrainingDummyCombatHPToDeadValue(t *testing.T) {
	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(StaticActorCombatKindTrainingDummy, TrainingDummyBootstrapMinLiveHP)
	if !ok {
		t.Fatal("expected bootstrap training-dummy normal attack to be supported at low HP")
	}
	if nextHP != 0 {
		t.Fatalf("expected training-dummy HP to reach dead value 0 from minimum live HP %d, got %d", TrainingDummyBootstrapMinLiveHP, nextHP)
	}
	if hpPercent != 0 {
		t.Fatalf("expected training-dummy HP percent 0 at dead value, got %d", hpPercent)
	}
}

func TestApplyBootstrapStaticActorNormalAttackRejectsDeadTrainingDummyCombatHP(t *testing.T) {
	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(StaticActorCombatKindTrainingDummy, 0)
	if ok {
		t.Fatalf("expected dead bootstrap training-dummy attack to fail, got nextHP=%d hpPercent=%d", nextHP, hpPercent)
	}
	if nextHP != 0 || hpPercent != 0 {
		t.Fatalf("expected dead bootstrap training-dummy attack to return zero values, got nextHP=%d hpPercent=%d", nextHP, hpPercent)
	}
}

func TestBootstrapStaticActorRespawnDelayReturnsTrainingDummyBootstrapDelay(t *testing.T) {
	delay, ok := BootstrapStaticActorRespawnDelay(StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected bootstrap training-dummy respawn delay to be supported")
	}
	if delay != TrainingDummyBootstrapRespawnDelay {
		t.Fatalf("expected training-dummy respawn delay %v, got %v", TrainingDummyBootstrapRespawnDelay, delay)
	}
}

func TestBootstrapStaticActorCombatProfileDefaultsSupportsTrainingDummyProfile(t *testing.T) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(StaticActorCombatProfileTrainingDummy)
	if !ok {
		t.Fatal("expected bootstrap training-dummy combat profile defaults to be supported")
	}
	if defaults.MaxHP != TrainingDummyBootstrapMaxHP {
		t.Fatalf("expected training-dummy max HP %d, got %d", TrainingDummyBootstrapMaxHP, defaults.MaxHP)
	}
	if defaults.DamagePerNormalAttack != TrainingDummyBootstrapDamagePerNormalAttack {
		t.Fatalf("expected training-dummy normal attack damage %d, got %d", TrainingDummyBootstrapDamagePerNormalAttack, defaults.DamagePerNormalAttack)
	}
	if defaults.RespawnDelay != TrainingDummyBootstrapRespawnDelay {
		t.Fatalf("expected training-dummy respawn delay %v, got %v", TrainingDummyBootstrapRespawnDelay, defaults.RespawnDelay)
	}
	if defaults.DeathReward.Experience != 0 || defaults.DeathReward.Gold != 0 || len(defaults.DeathReward.DropVnums) != 0 {
		t.Fatalf("expected rewardless training-dummy profile defaults, got %+v", defaults.DeathReward)
	}
}

func TestBootstrapStaticActorCombatProfileDefaultsRejectsUnknownProfile(t *testing.T) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults("boss")
	if ok {
		t.Fatalf("expected unknown combat profile defaults to fail closed, got %+v", defaults)
	}
	if defaults.MaxHP != 0 || defaults.DamagePerNormalAttack != 0 || defaults.RespawnDelay != 0 || defaults.DeathReward.Experience != 0 || defaults.DeathReward.Gold != 0 || len(defaults.DeathReward.DropVnums) != 0 {
		t.Fatalf("expected zero defaults on failure, got %+v", defaults)
	}
}

func TestBootstrapStaticActorCurrentHPSupportsTrainingDummyCombatProfile(t *testing.T) {
	currentHP, ok := BootstrapStaticActorCurrentHP(StaticActorCombatProfileTrainingDummy)
	if !ok {
		t.Fatal("expected bootstrap training-dummy combat profile to be supported")
	}
	if currentHP != TrainingDummyBootstrapMaxHP {
		t.Fatalf("expected training-dummy combat profile bootstrap HP %d, got %d", TrainingDummyBootstrapMaxHP, currentHP)
	}
}

func TestBootstrapStaticActorDeathRewardKeepsTrainingDummyRewardless(t *testing.T) {
	reward, ok := BootstrapStaticActorDeathReward(StaticActorCombatKindTrainingDummy)
	if !ok {
		t.Fatal("expected bootstrap training-dummy death reward to be supported")
	}
	if !reward.Empty() {
		t.Fatalf("expected rewardless training-dummy death reward, got %+v", reward)
	}
}

func TestStaticActorDeathRewardEmptyDetectsAnyRewardChannel(t *testing.T) {
	if !((StaticActorDeathReward{}).Empty()) {
		t.Fatal("expected zero-value death reward to be empty")
	}
	if (StaticActorDeathReward{Experience: 1}).Empty() {
		t.Fatal("expected EXP-bearing death reward to be non-empty")
	}
	if (StaticActorDeathReward{Gold: 1}).Empty() {
		t.Fatal("expected gold-bearing death reward to be non-empty")
	}
	if (StaticActorDeathReward{DropVnums: []uint32{1}}).Empty() {
		t.Fatal("expected drop-bearing death reward to be non-empty")
	}
}

func TestBootstrapStaticActorDeathRewardRejectsUnknownCombatKind(t *testing.T) {
	reward, ok := BootstrapStaticActorDeathReward("boss")
	if ok {
		t.Fatalf("expected unknown death reward to fail closed, got %+v", reward)
	}
	if reward.Experience != 0 || reward.Gold != 0 || len(reward.DropVnums) != 0 {
		t.Fatalf("expected zero reward on failure, got %+v", reward)
	}
}

func TestValidStaticActorCombatProfileRejectsUnknownProfile(t *testing.T) {
	if !ValidStaticActorCombatProfile("") {
		t.Fatal("expected empty combat profile to remain valid for non-combat actors")
	}
	if !ValidStaticActorCombatProfile(StaticActorCombatProfileTrainingDummy) {
		t.Fatal("expected training-dummy combat profile to be valid")
	}
	if ValidStaticActorCombatProfile("boss") {
		t.Fatal("expected unknown combat profile to fail closed")
	}
}
