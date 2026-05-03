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
