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

func TestApplyBootstrapStaticActorNormalAttackSupportsPracticeMobProfile(t *testing.T) {
	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(StaticActorCombatProfilePracticeMob, PracticeMobBootstrapMaxHP)
	if !ok {
		t.Fatal("expected bootstrap practice-mob normal attack to be supported")
	}
	if nextHP != PracticeMobBootstrapMaxHP-PracticeMobBootstrapDamagePerNormalAttack {
		t.Fatalf("expected practice-mob HP to decrement by one bootstrap hit, got %d", nextHP)
	}
	if hpPercent != 90 {
		t.Fatalf("expected practice-mob HP percent 90 after one bootstrap hit, got %d", hpPercent)
	}
}

func TestApplyBootstrapStaticActorNormalAttackUsesRegisteredAttackDefenseFormula(t *testing.T) {
	const profile = "practice_formula_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        20,
		AttackValue:  7,
		DefenseValue: 3,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with formula stats to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(profile, 20)
	if !ok {
		t.Fatal("expected registered formula profile normal attack to be supported")
	}
	if nextHP != 16 {
		t.Fatalf("expected attack 7 minus defense 3 to deal 4 damage, got next HP %d", nextHP)
	}
	if hpPercent != 80 {
		t.Fatalf("expected formula profile HP percent 80 after one hit, got %d", hpPercent)
	}
}

func TestApplyBootstrapStaticActorNormalAttackClampsFormulaDamageToMinimumOne(t *testing.T) {
	const profile = "practice_armored_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        20,
		AttackValue:  2,
		DefenseValue: 9,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with armored formula stats to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(profile, 20)
	if !ok {
		t.Fatal("expected registered armored profile normal attack to be supported")
	}
	if nextHP != 19 {
		t.Fatalf("expected attack lower than defense to clamp to 1 damage, got next HP %d", nextHP)
	}
	if hpPercent != 95 {
		t.Fatalf("expected formula profile HP percent 95 after minimum hit, got %d", hpPercent)
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

func TestBootstrapStaticActorRespawnDelayReturnsPracticeMobBootstrapDelay(t *testing.T) {
	delay, ok := BootstrapStaticActorRespawnDelay(StaticActorCombatProfilePracticeMob)
	if !ok {
		t.Fatal("expected bootstrap practice-mob respawn delay to be supported")
	}
	if delay != PracticeMobBootstrapRespawnDelay {
		t.Fatalf("expected practice-mob respawn delay %v, got %v", PracticeMobBootstrapRespawnDelay, delay)
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
	if defaults.AttackValue != TrainingDummyBootstrapAttackValue {
		t.Fatalf("expected training-dummy attack value %d, got %d", TrainingDummyBootstrapAttackValue, defaults.AttackValue)
	}
	if defaults.DefenseValue != TrainingDummyBootstrapDefenseValue {
		t.Fatalf("expected training-dummy defense value %d, got %d", TrainingDummyBootstrapDefenseValue, defaults.DefenseValue)
	}
	if defaults.RespawnDelay != TrainingDummyBootstrapRespawnDelay {
		t.Fatalf("expected training-dummy respawn delay %v, got %v", TrainingDummyBootstrapRespawnDelay, defaults.RespawnDelay)
	}
	if defaults.Level != TrainingDummyBootstrapLevel {
		t.Fatalf("expected training-dummy level %d, got %d", TrainingDummyBootstrapLevel, defaults.Level)
	}
	if defaults.Rank != TrainingDummyBootstrapRank {
		t.Fatalf("expected training-dummy rank %d, got %d", TrainingDummyBootstrapRank, defaults.Rank)
	}
	if defaults.DeathReward.Experience != 0 || defaults.DeathReward.Gold != 0 || len(defaults.DeathReward.DropVnums) != 0 {
		t.Fatalf("expected rewardless training-dummy profile defaults, got %+v", defaults.DeathReward)
	}
}

func TestBootstrapStaticActorCombatProfileDefaultsSupportsPracticeMobProfile(t *testing.T) {
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(StaticActorCombatProfilePracticeMob)
	if !ok {
		t.Fatal("expected bootstrap practice-mob combat profile defaults to be supported")
	}
	if defaults.MaxHP != PracticeMobBootstrapMaxHP {
		t.Fatalf("expected practice-mob max HP %d, got %d", PracticeMobBootstrapMaxHP, defaults.MaxHP)
	}
	if defaults.DamagePerNormalAttack != PracticeMobBootstrapDamagePerNormalAttack {
		t.Fatalf("expected practice-mob normal attack damage %d, got %d", PracticeMobBootstrapDamagePerNormalAttack, defaults.DamagePerNormalAttack)
	}
	if defaults.AttackValue != PracticeMobBootstrapAttackValue {
		t.Fatalf("expected practice-mob attack value %d, got %d", PracticeMobBootstrapAttackValue, defaults.AttackValue)
	}
	if defaults.DefenseValue != PracticeMobBootstrapDefenseValue {
		t.Fatalf("expected practice-mob defense value %d, got %d", PracticeMobBootstrapDefenseValue, defaults.DefenseValue)
	}
	if defaults.RespawnDelay != PracticeMobBootstrapRespawnDelay {
		t.Fatalf("expected practice-mob respawn delay %v, got %v", PracticeMobBootstrapRespawnDelay, defaults.RespawnDelay)
	}
	if defaults.Level != PracticeMobBootstrapLevel {
		t.Fatalf("expected practice-mob level %d, got %d", PracticeMobBootstrapLevel, defaults.Level)
	}
	if defaults.Rank != PracticeMobBootstrapRank {
		t.Fatalf("expected practice-mob rank %d, got %d", PracticeMobBootstrapRank, defaults.Rank)
	}
	if !defaults.DeathReward.Empty() {
		t.Fatalf("expected rewardless practice-mob profile defaults, got %+v", defaults.DeathReward)
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

func TestRegisterStaticActorCombatProfileAddsProfileDefaults(t *testing.T) {
	const profile = "practice_wolf"
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected %q to start unregistered", profile)
	}
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		Level:        12,
		Rank:         3,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	if !ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected %q to become a valid static actor combat profile", profile)
	}
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered profile defaults to resolve")
	}
	if defaults.MaxHP != 24 || defaults.DamagePerNormalAttack != 6 || defaults.AttackValue != 8 || defaults.DefenseValue != 2 || defaults.Level != 12 || defaults.Rank != 3 || defaults.RespawnDelay != PracticeMobBootstrapRespawnDelay {
		t.Fatalf("unexpected registered profile defaults: %+v", defaults)
	}
}

func TestRegisterStaticActorCombatProfileCanonicalizesOmittedLevelToBootstrapDefault(t *testing.T) {
	const profile = "practice_level_default_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with omitted level to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered profile defaults to resolve")
	}
	if defaults.Level != TrainingDummyBootstrapLevel {
		t.Fatalf("expected omitted registered profile level to canonicalize to bootstrap level %d, got %d", TrainingDummyBootstrapLevel, defaults.Level)
	}
	if defaults.Rank != 0 {
		t.Fatalf("expected omitted registered profile rank to remain bootstrap rank 0, got %d", defaults.Rank)
	}
}

func TestRegisterStaticActorCombatProfileCanonicalizesOmittedAttackValueWithDefenseFromLegacyDamage(t *testing.T) {
	const profile = "practice_legacy_damage_armored_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 5,
		DefenseValue:          3,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with legacy damage plus defense to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered profile defaults to resolve")
	}
	if defaults.AttackValue != 8 || defaults.DefenseValue != 3 || defaults.DamagePerNormalAttack != 5 {
		t.Fatalf("expected omitted attack value to preserve legacy damage through attack-defense formula, got %+v", defaults)
	}

	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(profile, 20)
	if !ok {
		t.Fatal("expected legacy-damage armored profile normal attack to be supported")
	}
	if nextHP != 15 || hpPercent != 62 {
		t.Fatalf("expected legacy damage 5 to be preserved through defense canonicalization, got nextHP=%d hpPercent=%d", nextHP, hpPercent)
	}
}

func TestRegisterStaticActorCombatProfileRejectsLegacyDamageDefenseOverflow(t *testing.T) {
	const profile = "practice_legacy_damage_overflow_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 5,
		DefenseValue:          ^uint16(0),
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with legacy damage plus overflowing defense canonicalization to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected overflow profile %q not to become valid", profile)
	}
}

func TestRegisterStaticActorCombatProfileRejectsContradictoryLegacyDamageAndFormula(t *testing.T) {
	const profile = "practice_contradictory_damage_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 5,
		AttackValue:           2,
		DefenseValue:          9,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with contradictory legacy damage and attack/defense formula to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected contradictory profile %q not to become valid", profile)
	}
}

func TestRegisterStaticActorCombatProfileAcceptsConsistentLegacyDamageAndFormula(t *testing.T) {
	const profile = "practice_consistent_damage_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 5,
		AttackValue:           8,
		DefenseValue:          3,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with consistent legacy damage and attack/defense formula to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected consistent registered profile defaults to resolve")
	}
	if defaults.DamagePerNormalAttack != 5 || defaults.AttackValue != 8 || defaults.DefenseValue != 3 {
		t.Fatalf("expected consistent profile defaults to preserve both damage surfaces, got %+v", defaults)
	}
}

func TestRegisterStaticActorCombatProfileAcceptsFormulaWithoutLegacyDamage(t *testing.T) {
	const profile = "practice_formula_only_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  9,
		DefenseValue: 4,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with formula-only damage to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected formula-only registered profile defaults to resolve")
	}
	if defaults.DamagePerNormalAttack != 5 || defaults.AttackValue != 9 || defaults.DefenseValue != 4 {
		t.Fatalf("expected formula-only profile to canonicalize legacy damage from attack-defense stats, got %+v", defaults)
	}

	nextHP, hpPercent, ok := ApplyBootstrapStaticActorNormalAttack(profile, 24)
	if !ok {
		t.Fatal("expected formula-only profile normal attack to be supported")
	}
	if nextHP != 19 || hpPercent != 79 {
		t.Fatalf("expected formula-only profile to deal 5 damage, got nextHP=%d hpPercent=%d", nextHP, hpPercent)
	}
}

func TestRegisterStaticActorCombatProfileAddsDeathRewardDefaults(t *testing.T) {
	const profile = "practice_reward_wolf"
	reward := StaticActorDeathReward{Experience: 25, Gold: 7, DropVnums: []uint32{27001, 27002}}
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
		DeathReward:           reward,
	}) {
		t.Fatalf("expected %q profile registration with death reward to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	reward.DropVnums[0] = 99999
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected registered reward profile defaults to resolve")
	}
	if defaults.DeathReward.Experience != 25 || defaults.DeathReward.Gold != 7 || len(defaults.DeathReward.DropVnums) != 2 || defaults.DeathReward.DropVnums[0] != 27001 || defaults.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected registered profile to clone death reward defaults, got %+v", defaults.DeathReward)
	}

	defaults.DeathReward.DropVnums[0] = 11111
	resolvedReward, ok := BootstrapStaticActorDeathReward(profile)
	if !ok {
		t.Fatalf("expected registered reward profile death reward to resolve")
	}
	if resolvedReward.Experience != 25 || resolvedReward.Gold != 7 || len(resolvedReward.DropVnums) != 2 || resolvedReward.DropVnums[0] != 27001 || resolvedReward.DropVnums[1] != 27002 {
		t.Fatalf("expected death reward lookup to return an isolated clone, got %+v", resolvedReward)
	}
}

func TestRegisterStaticActorCombatProfileRejectsDamageAboveMaxHP(t *testing.T) {
	const profile = "practice_overkill_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 2,
		DamagePerNormalAttack: 3,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with damage above max HP to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected over-damage profile %q not to become valid", profile)
	}
}

func TestRegisterStaticActorCombatProfileRejectsExplicitFormulaDamageAboveMaxHP(t *testing.T) {
	const profile = "practice_formula_overkill_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        2,
		AttackValue:  5,
		DefenseValue: 1,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration with explicit formula damage above max HP to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected explicit over-damage profile %q not to become valid", profile)
	}
}

func TestRegisterStaticActorCombatProfileRejectsMissingDamageAndFormula(t *testing.T) {
	const profile = "practice_missing_damage_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected %q profile registration without legacy damage or explicit attack formula to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected missing-damage profile %q not to become valid", profile)
	}
}

func TestRegisterStaticActorCombatProfileRejectsNonCanonicalName(t *testing.T) {
	for _, profile := range []string{
		" practice_whitespace_wolf ",
		"PracticeUppercaseWolf",
		"practice-hyphen-wolf",
		"practice.dot.wolf",
		"1practice_digit_wolf",
	} {
		t.Run(profile, func(t *testing.T) {
			if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
				MaxHP:                 24,
				DamagePerNormalAttack: 3,
				RespawnDelay:          PracticeMobBootstrapRespawnDelay,
			}) {
				t.Fatalf("expected non-canonical profile name %q to fail closed", profile)
			}
		})
	}
	if ValidStaticActorCombatProfile("practice_whitespace_wolf") {
		t.Fatalf("expected trimmed form of rejected whitespace-padded profile not to become valid")
	}
	if ValidStaticActorCombatProfile("practice.dot.wolf") {
		t.Fatalf("expected dotted rejected profile not to become valid")
	}
}

func TestRegisterStaticActorCombatProfileRejectsInvalidDeathReward(t *testing.T) {
	const profile = "practice_invalid_reward_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
		DeathReward:           StaticActorDeathReward{DropVnums: []uint32{27001, 0}},
	}) {
		t.Fatalf("expected %q profile registration with invalid death reward to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected invalid reward profile %q not to become valid", profile)
	}
}

func TestRegisterStaticActorCombatProfileRejectsDuplicateDeathRewardDropsBeforeCloneNormalization(t *testing.T) {
	const profile = "practice_duplicate_reward_wolf"
	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
		DeathReward:           StaticActorDeathReward{DropVnums: []uint32{27002, 27001, 27002}},
	}) {
		t.Fatalf("expected %q profile registration with duplicate reward drop vnums to fail closed", profile)
	}
	if ValidStaticActorCombatProfile(profile) {
		t.Fatalf("expected duplicate reward profile %q not to become valid", profile)
	}
}

func TestRegisterStaticActorCombatProfileRejectsDuplicateName(t *testing.T) {
	const profile = "practice_boar"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 24,
		DamagePerNormalAttack: 3,
		RespawnDelay:          PracticeMobBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected initial %q profile registration to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	if RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:                 99,
		DamagePerNormalAttack: 9,
		RespawnDelay:          TrainingDummyBootstrapRespawnDelay,
	}) {
		t.Fatalf("expected duplicate %q profile registration to fail closed", profile)
	}
	defaults, ok := BootstrapStaticActorCombatProfileDefaults(profile)
	if !ok {
		t.Fatalf("expected original registered profile defaults to remain available")
	}
	if defaults.MaxHP != 24 || defaults.DamagePerNormalAttack != 3 || defaults.RespawnDelay != PracticeMobBootstrapRespawnDelay {
		t.Fatalf("expected duplicate registration to preserve original defaults, got %+v", defaults)
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

func TestBootstrapStaticActorHPPercentSupportsPracticeMobCombatProfile(t *testing.T) {
	hpPercent, ok := BootstrapStaticActorHPPercent(StaticActorCombatProfilePracticeMob, PracticeMobBootstrapMaxHP-PracticeMobBootstrapDamagePerNormalAttack)
	if !ok {
		t.Fatal("expected bootstrap practice-mob combat profile HP percent to be supported")
	}
	if hpPercent != 90 {
		t.Fatalf("expected practice-mob HP percent 90 after one bootstrap hit, got %d", hpPercent)
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

func TestBootstrapStaticActorDeathRewardKeepsPracticeMobRewardless(t *testing.T) {
	reward, ok := BootstrapStaticActorDeathReward(StaticActorCombatProfilePracticeMob)
	if !ok {
		t.Fatal("expected bootstrap practice-mob death reward to be supported")
	}
	if !reward.Empty() {
		t.Fatalf("expected rewardless practice-mob death reward, got %+v", reward)
	}
}

func TestValidStaticActorDeathRewardRejectsScalarPointCarrierOverflow(t *testing.T) {
	maxPointCarrier := uint64(^uint32(0) >> 1)
	if !ValidStaticActorDeathReward(StaticActorDeathReward{Experience: maxPointCarrier, Gold: maxPointCarrier}) {
		t.Fatal("expected signed 32-bit scalar reward carrier maximum to be valid")
	}
	if ValidStaticActorDeathReward(StaticActorDeathReward{Experience: maxPointCarrier + 1}) {
		t.Fatal("expected experience reward above signed 32-bit point-change carrier to be rejected")
	}
	if ValidStaticActorDeathReward(StaticActorDeathReward{Gold: maxPointCarrier + 1}) {
		t.Fatal("expected gold reward above signed 32-bit point-change carrier to be rejected")
	}
}

func TestValidStaticActorDeathRewardRejectsZeroDropVnum(t *testing.T) {
	if ValidStaticActorDeathReward(StaticActorDeathReward{DropVnums: []uint32{27001, 0, 27002}}) {
		t.Fatal("expected zero-valued reward drop vnum to be rejected")
	}
}

func TestValidStaticActorDeathRewardRejectsDuplicateDropVnum(t *testing.T) {
	if ValidStaticActorDeathReward(StaticActorDeathReward{DropVnums: []uint32{27001, 27002, 27001}}) {
		t.Fatal("expected duplicate reward drop vnum to be rejected")
	}
}

func TestValidStaticActorDeathRewardAcceptsMultipleDistinctDropVnums(t *testing.T) {
	reward := StaticActorDeathReward{DropVnums: []uint32{27001, 27002, 27003}}
	if !ValidStaticActorDeathReward(reward) {
		t.Fatalf("expected multiple distinct reward drop vnums to be accepted, got %+v", reward)
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

func TestStaticActorDeathRewardCloneCopiesDropVnums(t *testing.T) {
	reward := StaticActorDeathReward{Experience: 7, Gold: 11, DropVnums: []uint32{101, 202}}
	cloned := reward.Clone()

	if cloned.Experience != reward.Experience || cloned.Gold != reward.Gold {
		t.Fatalf("expected clone to preserve scalar reward fields, got %+v from %+v", cloned, reward)
	}
	if len(cloned.DropVnums) != len(reward.DropVnums) || cloned.DropVnums[0] != 101 || cloned.DropVnums[1] != 202 {
		t.Fatalf("expected clone to preserve drop vnums, got %+v from %+v", cloned, reward)
	}

	reward.DropVnums[0] = 999
	if cloned.DropVnums[0] != 101 {
		t.Fatalf("expected cloned drop list to be isolated from source mutation, got %+v", cloned.DropVnums)
	}
}

func TestStaticActorDeathRewardCloneSortsAndDeduplicatesDropVnumsDeterministically(t *testing.T) {
	reward := StaticActorDeathReward{DropVnums: []uint32{27003, 27001, 27002, 27001, 27003}}
	cloned := reward.Clone()

	if len(cloned.DropVnums) != 3 || cloned.DropVnums[0] != 27001 || cloned.DropVnums[1] != 27002 || cloned.DropVnums[2] != 27003 {
		t.Fatalf("expected cloned drop list to be sorted and deduplicated deterministically, got %+v", cloned.DropVnums)
	}
	if reward.DropVnums[0] != 27003 || reward.DropVnums[1] != 27001 || reward.DropVnums[2] != 27002 || reward.DropVnums[3] != 27001 || reward.DropVnums[4] != 27003 {
		t.Fatalf("expected normalizing clone to leave source order unchanged, got %+v", reward.DropVnums)
	}
}

func TestStaticActorDeathRewardCloneNormalizesEmptyDropVnums(t *testing.T) {
	cloned := (StaticActorDeathReward{DropVnums: []uint32{}}).Clone()
	if cloned.DropVnums != nil {
		t.Fatalf("expected empty drop list clone to normalize to nil, got %#v", cloned.DropVnums)
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
	if !ValidStaticActorCombatProfile(StaticActorCombatProfilePracticeMob) {
		t.Fatal("expected practice-mob combat profile to be valid")
	}
	if ValidStaticActorCombatProfile("boss") {
		t.Fatal("expected unknown combat profile to fail closed")
	}
}
