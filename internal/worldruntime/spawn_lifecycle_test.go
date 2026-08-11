package worldruntime

import "testing"

func TestEvaluateStaticActorSpawnLeashTreatsAuthoredPositionAsHome(t *testing.T) {
	actor := StaticEntity{
		Entity:        Entity{ID: 7, Kind: EntityKindStaticActor, Name: "PracticeMob"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.leash_home",
	}

	evaluation, ok := EvaluateStaticActorSpawnLeash(actor, actor.Position, 400)
	if !ok {
		t.Fatal("expected spawn-backed practice mob leash evaluation to resolve")
	}
	if evaluation.Status != SpawnLeashStatusAtHome || evaluation.ReturnRequired {
		t.Fatalf("expected stationary actor to be at home without return requirement, got %+v", evaluation)
	}
	if evaluation.Home != actor.Position || evaluation.Current != actor.Position || evaluation.ReturnTarget != actor.Position || evaluation.Radius != 400 {
		t.Fatalf("expected leash evaluation to preserve authored home/current/radius, got %+v", evaluation)
	}
}

func TestEvaluateSpawnLeashClassifiesInsideAndOutsideRadius(t *testing.T) {
	home := NewPosition(42, 1700, 2800)

	inside, ok := EvaluateSpawnLeash(home, NewPosition(42, 1900, 2900), 400)
	if !ok {
		t.Fatal("expected inside-radius leash evaluation to resolve")
	}
	if inside.Status != SpawnLeashStatusWithinRadius || inside.ReturnRequired {
		t.Fatalf("expected inside-radius position to stay within leash, got %+v", inside)
	}

	outside, ok := EvaluateSpawnLeash(home, NewPosition(42, 2301, 2800), 400)
	if !ok {
		t.Fatal("expected outside-radius leash evaluation to resolve")
	}
	if outside.Status != SpawnLeashStatusReturnRequired || !outside.ReturnRequired || outside.ReturnTarget != home {
		t.Fatalf("expected outside-radius position to require return home, got %+v", outside)
	}
}

func TestEvaluateSpawnLeashRequiresReturnAcrossMaps(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	evaluation, ok := EvaluateSpawnLeash(home, NewPosition(43, 1700, 2800), 400)
	if !ok {
		t.Fatal("expected cross-map leash evaluation to resolve")
	}
	if evaluation.Status != SpawnLeashStatusReturnRequired || !evaluation.ReturnRequired || evaluation.ReturnTarget != home {
		t.Fatalf("expected cross-map position to require return to authored home, got %+v", evaluation)
	}
}

func TestEvaluateStaticActorSpawnLeashFailsClosedForNonSpawnActorsAndInvalidInput(t *testing.T) {
	plainActor := StaticEntity{Entity: Entity{ID: 8, Kind: EntityKindStaticActor, Name: "VillageGuide"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if evaluation, ok := EvaluateStaticActorSpawnLeash(plainActor, plainActor.Position, 400); ok || evaluation.Status != "" {
		t.Fatalf("expected non-spawn static actor leash evaluation to fail closed, got ok=%v evaluation=%+v", ok, evaluation)
	}

	spawnActor := plainActor
	spawnActor.Entity.Name = "PracticeMob"
	spawnActor.CombatProfile = StaticActorCombatProfilePracticeMob
	spawnActor.CombatKind = StaticActorCombatProfilePracticeMob
	spawnActor.SpawnGroupRef = "practice.invalid_leash"
	if evaluation, ok := EvaluateStaticActorSpawnLeash(spawnActor, spawnActor.Position, 0); ok || evaluation.Status != "" {
		t.Fatalf("expected non-positive leash radius to fail closed, got ok=%v evaluation=%+v", ok, evaluation)
	}
	if evaluation, ok := EvaluateStaticActorSpawnLeash(spawnActor, NewPosition(0, 1700, 2800), 400); ok || evaluation.Status != "" {
		t.Fatalf("expected invalid current position to fail closed, got ok=%v evaluation=%+v", ok, evaluation)
	}
}
