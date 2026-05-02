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

func TestApplyBootstrapStaticActorNormalAttackClampsTrainingDummyCombatHPAtMinimumLiveValue(t *testing.T) {
	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(StaticActorCombatKindTrainingDummy, TrainingDummyBootstrapMinLiveHP)
	if !ok {
		t.Fatal("expected bootstrap training-dummy normal attack to be supported at low HP")
	}
	if nextHP != TrainingDummyBootstrapMinLiveHP {
		t.Fatalf("expected training-dummy HP to clamp at minimum live value %d, got %d", TrainingDummyBootstrapMinLiveHP, nextHP)
	}
	if hpPercent != 10 {
		t.Fatalf("expected training-dummy HP percent 10 at minimum live HP, got %d", hpPercent)
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
