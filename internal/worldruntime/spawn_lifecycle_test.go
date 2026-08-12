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

func TestEvaluateStaticActorCurrentSpawnLeashUsesAuthoredHomeSeparateFromCurrentPosition(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	current := NewPosition(42, 2301, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 8, Kind: EntityKindStaticActor, Name: "PracticeMob"},
		Position:      current,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.leash_current",
	}

	evaluation, ok := EvaluateStaticActorCurrentSpawnLeash(actor, 400)
	if !ok {
		t.Fatal("expected spawn-backed practice mob current leash evaluation to resolve")
	}
	if evaluation.Home != home || evaluation.Current != current || evaluation.Status != SpawnLeashStatusReturnRequired || !evaluation.ReturnRequired || evaluation.ReturnTarget != home {
		t.Fatalf("expected current position outside authored home radius to require return home, got %+v", evaluation)
	}

	snapshot := staticActorSnapshot(NewBootstrapTopology(1), actor)
	if snapshot.SpawnLeash == nil {
		t.Fatalf("expected spawn-group snapshot to expose leash classification")
	}
	if snapshot.SpawnLeash.Home.MapIndex != 42 || snapshot.SpawnLeash.Home.X != 1700 || snapshot.SpawnLeash.Current.X != 2301 || snapshot.SpawnLeash.Status != SpawnLeashStatusReturnRequired || !snapshot.SpawnLeash.ReturnRequired {
		t.Fatalf("unexpected spawn leash snapshot: %+v", snapshot.SpawnLeash)
	}
	if snapshot.SpawnHome == nil || snapshot.SpawnHome.MapIndex != 42 || snapshot.SpawnHome.X != 1700 || snapshot.SpawnHome.Y != 2800 {
		t.Fatalf("expected spawn snapshot to expose authored home separately for persistence, got %+v", snapshot.SpawnHome)
	}
}

func TestEntityRegistryPreservesSpawnHomeAcrossStaticActorPositionUpdate(t *testing.T) {
	registry := NewEntityRegistry()
	home := NewPosition(42, 1700, 2800)
	actor, ok := registry.RegisterStaticActor(StaticEntity{
		Entity:        Entity{Name: "PracticeMob"},
		Position:      home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.leash_registry",
	})
	if !ok {
		t.Fatal("expected spawn actor registration to succeed")
	}
	if actor.SpawnHome != home {
		t.Fatalf("expected registration to default spawn home to authored position, got %+v", actor.SpawnHome)
	}

	updated := actor
	updated.Position = NewPosition(42, 2301, 2800)
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected live static actor position update to succeed")
	}
	if result.SpawnHome != home || result.Position != updated.Position {
		t.Fatalf("expected update to preserve spawn home while changing current position, got %+v", result)
	}
	evaluation, ok := EvaluateStaticActorCurrentSpawnLeash(result, 400)
	if !ok || !evaluation.ReturnRequired || evaluation.ReturnTarget != home {
		t.Fatalf("expected moved registry actor to require return home, ok=%v evaluation=%+v", ok, evaluation)
	}
}

func TestEntityRegistryPreservesSpawnHomeWhenUpdateOmitsSpawnHome(t *testing.T) {
	registry := NewEntityRegistry()
	home := NewPosition(42, 1700, 2800)
	actor, ok := registry.RegisterStaticActor(StaticEntity{
		Entity:        Entity{Name: "PracticeMob"},
		Position:      home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.leash_registry_omitted_home",
	})
	if !ok {
		t.Fatal("expected spawn actor registration to succeed")
	}

	updated := StaticEntity{
		Entity:        Entity{ID: actor.Entity.ID, Name: "PracticeMobMoved"},
		Position:      NewPosition(42, 2301, 2800),
		RaceNum:       actor.RaceNum,
		CombatProfile: actor.CombatProfile,
		SpawnGroupRef: actor.SpawnGroupRef,
	}
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected spawn actor update that omits spawn home to succeed")
	}
	if result.SpawnHome != home || result.Position != updated.Position {
		t.Fatalf("expected omitted-home update to preserve authored home and change current position, got %+v", result)
	}
	evaluation, ok := EvaluateStaticActorCurrentSpawnLeash(result, 400)
	if !ok || !evaluation.ReturnRequired || evaluation.ReturnTarget != home {
		t.Fatalf("expected moved omitted-home update to require return home, ok=%v evaluation=%+v", ok, evaluation)
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

func TestSpawnLeashSnapshotFromEvaluationExposesJSONFriendlyShape(t *testing.T) {
	evaluation, ok := EvaluateSpawnLeash(NewPosition(42, 1700, 2800), NewPosition(42, 2301, 2800), 400)
	if !ok {
		t.Fatal("expected return-required leash evaluation to resolve")
	}

	snapshot := SpawnLeashSnapshotFromEvaluation(evaluation)
	if snapshot.Status != SpawnLeashStatusReturnRequired || !snapshot.ReturnRequired || snapshot.Radius != 400 {
		t.Fatalf("unexpected leash snapshot status/radius: %+v", snapshot)
	}
	if snapshot.Home.MapIndex != 42 || snapshot.Home.X != 1700 || snapshot.Home.Y != 2800 {
		t.Fatalf("unexpected leash home snapshot: %+v", snapshot.Home)
	}
	if snapshot.Current.MapIndex != 42 || snapshot.Current.X != 2301 || snapshot.Current.Y != 2800 {
		t.Fatalf("unexpected leash current snapshot: %+v", snapshot.Current)
	}
	if snapshot.ReturnTarget.MapIndex != 42 || snapshot.ReturnTarget.X != 1700 || snapshot.ReturnTarget.Y != 2800 {
		t.Fatalf("unexpected leash return target snapshot: %+v", snapshot.ReturnTarget)
	}
}

func TestSpawnLeashSnapshotFromEvaluationOmitsReturnTargetWhenReturnIsNotRequired(t *testing.T) {
	evaluation, ok := EvaluateSpawnLeash(NewPosition(42, 1700, 2800), NewPosition(42, 1800, 2800), 400)
	if !ok {
		t.Fatal("expected within-radius leash evaluation to resolve")
	}

	snapshot := SpawnLeashSnapshotFromEvaluation(evaluation)
	if snapshot.Status != SpawnLeashStatusWithinRadius || snapshot.ReturnRequired {
		t.Fatalf("expected within-radius leash snapshot, got %+v", snapshot)
	}
	if snapshot.ReturnTarget != nil {
		t.Fatalf("expected within-radius leash snapshot to omit return target, got %+v", snapshot.ReturnTarget)
	}
}
