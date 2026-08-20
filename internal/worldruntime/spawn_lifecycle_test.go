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

func TestPlanStaticActorSpawnLeashReturnStepMovesReturnRequiredActorTowardHomeWithoutMutating(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	current := NewPosition(42, 2301, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 30, Kind: EntityKindStaticActor, Name: "ReturnPlannerMob"},
		Position:      current,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.return_planner",
	}

	plan, ok := PlanStaticActorSpawnLeashReturnStep(actor, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected return-required spawn actor to produce a return-step plan")
	}
	if plan.Evaluation.Status != SpawnLeashStatusReturnRequired || !plan.Evaluation.ReturnRequired {
		t.Fatalf("expected return-required evaluation in return-step plan, got %+v", plan.Evaluation)
	}
	if plan.Next != NewPosition(42, 2201, 2800) || plan.Complete {
		t.Fatalf("expected one 100-unit x-axis return step toward home, got %+v", plan)
	}
	if actor.Position != current || actor.SpawnHome != home {
		t.Fatalf("expected return-step planning not to mutate actor, got actor=%+v", actor)
	}
}

func TestPlanStaticActorSpawnLeashReturnStepCompletesAtHomeWhenWithinOneStep(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 31, Kind: EntityKindStaticActor, Name: "ReturnPlannerNearMob"},
		Position:      NewPosition(42, 2101, 2800),
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.return_planner_near",
	}

	plan, ok := PlanStaticActorSpawnLeashReturnStep(actor, DefaultSpawnLeashRadius, 500)
	if !ok {
		t.Fatal("expected near return-required spawn actor to produce a return-step plan")
	}
	if plan.Next != home || !plan.Complete {
		t.Fatalf("expected one large return step to land exactly at authored home, got %+v", plan)
	}
}

func TestPlanStaticActorSpawnLeashReturnStepUsesHomeForCrossMapReturn(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 32, Kind: EntityKindStaticActor, Name: "ReturnPlannerCrossMapMob"},
		Position:      NewPosition(43, 1700, 2800),
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.return_planner_cross_map",
	}

	plan, ok := PlanStaticActorSpawnLeashReturnStep(actor, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected cross-map return-required spawn actor to produce a return-step plan")
	}
	if plan.Next != home || !plan.Complete {
		t.Fatalf("expected cross-map return step to target authored home directly, got %+v", plan)
	}
}

func TestPlanStaticActorSpawnLeashReturnStepNoOpsWhenReturnNotRequired(t *testing.T) {
	homeActor := StaticEntity{
		Entity:        Entity{ID: 33, Kind: EntityKindStaticActor, Name: "ReturnPlannerHomeMob"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.return_planner_home",
	}

	plan, ok := PlanStaticActorSpawnLeashReturnStep(homeActor, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected at-home spawn actor to produce a no-op return-step plan")
	}
	if plan.Evaluation.Status != SpawnLeashStatusAtHome || plan.Next != homeActor.Position || !plan.Complete {
		t.Fatalf("expected at-home return-step plan to be complete no-op, got %+v", plan)
	}

	withinActor := StaticEntity{
		Entity:        Entity{ID: 35, Kind: EntityKindStaticActor, Name: "ReturnPlannerWithinMob"},
		Position:      NewPosition(42, 1900, 2900),
		SpawnHome:     NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.return_planner_within",
	}
	plan, ok = PlanStaticActorSpawnLeashReturnStep(withinActor, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected within-radius spawn actor to produce a no-op return-step plan")
	}
	if plan.Evaluation.Status != SpawnLeashStatusWithinRadius || plan.Next != withinActor.Position || !plan.Complete {
		t.Fatalf("expected within-radius return-step plan to be complete no-op, got %+v", plan)
	}
}

func TestPlanStaticActorSpawnLeashReturnStepFailsClosedForInvalidInput(t *testing.T) {
	actor := StaticEntity{Entity: Entity{ID: 34, Kind: EntityKindStaticActor, Name: "ReturnPlannerInvalidMob"}, Position: NewPosition(42, 2301, 2800), RaceNum: 20350}
	if plan, ok := PlanStaticActorSpawnLeashReturnStep(actor, DefaultSpawnLeashRadius, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected non-spawn actor return-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}

	actor.CombatProfile = StaticActorCombatProfilePracticeMob
	actor.CombatKind = StaticActorCombatProfilePracticeMob
	actor.SpawnGroupRef = "practice.return_planner_invalid"
	if plan, ok := PlanStaticActorSpawnLeashReturnStep(actor, 0, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected non-positive leash radius return-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}
	if plan, ok := PlanStaticActorSpawnLeashReturnStep(actor, DefaultSpawnLeashRadius, 0); ok || plan.Next.Valid() {
		t.Fatalf("expected non-positive max step return-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}
}

func TestPlanStaticActorSpawnLeashHomewardStepMovesWithinRadiusTowardHomeWithoutMutating(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	current := NewPosition(42, 1900, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 36, Kind: EntityKindStaticActor, Name: "HomewardPlannerMob"},
		Position:      current,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.homeward_planner",
	}

	plan, ok := PlanStaticActorSpawnLeashHomewardStep(actor, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected within-radius spawn actor to produce a homeward-step plan toward home")
	}
	if plan.Evaluation.Status != SpawnLeashStatusWithinRadius {
		t.Fatalf("expected within-radius evaluation in homeward-step plan, got %+v", plan.Evaluation)
	}
	if plan.Next != NewPosition(42, 1800, 2800) || plan.Complete {
		t.Fatalf("expected one 100-unit x-axis homeward step toward home, got %+v", plan)
	}
	if actor.Position != current || actor.SpawnHome != home {
		t.Fatalf("expected homeward-step planning not to mutate actor, got actor=%+v", actor)
	}
}

func TestPlanStaticActorSpawnLeashHomewardStepCompletesAtHomeWhenWithinOneStep(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	current := NewPosition(42, 1750, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 37, Kind: EntityKindStaticActor, Name: "HomewardPlannerNearHomeMob"},
		Position:      current,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.homeward_planner_near",
	}

	plan, ok := PlanStaticActorSpawnLeashHomewardStep(actor, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected near-home within-radius actor to produce a homeward-step plan")
	}
	if plan.Next != home || !plan.Complete {
		t.Fatalf("expected homeward plan to land exactly on authored home when within one step, got %+v", plan)
	}
}

func TestPlanStaticActorSpawnLeashHomewardStepNoOpsWhenAtHome(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 38, Kind: EntityKindStaticActor, Name: "HomewardPlannerAtHomeMob"},
		Position:      home,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.homeward_planner_home",
	}

	plan, ok := PlanStaticActorSpawnLeashHomewardStep(actor, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected at-home spawn actor to produce a no-op homeward-step plan")
	}
	if plan.Evaluation.Status != SpawnLeashStatusAtHome || plan.Next != home || !plan.Complete {
		t.Fatalf("expected at-home homeward-step plan to be complete no-op, got %+v", plan)
	}
}

func TestPlanStaticActorSpawnLeashHomewardStepFailsClosedForReturnRequiredOrInvalidInput(t *testing.T) {
	outside := StaticEntity{
		Entity:        Entity{ID: 39, Kind: EntityKindStaticActor, Name: "HomewardPlannerOutsideMob"},
		Position:      NewPosition(42, 2301, 2800),
		SpawnHome:     NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.homeward_planner_outside",
	}
	if plan, ok := PlanStaticActorSpawnLeashHomewardStep(outside, DefaultSpawnLeashRadius, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected return_required homeward plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}

	invalid := StaticEntity{Entity: Entity{ID: 391, Kind: EntityKindStaticActor, Name: "HomewardPlannerInvalidMob"}, Position: NewPosition(42, 1800, 2800), RaceNum: 20350}
	if plan, ok := PlanStaticActorSpawnLeashHomewardStep(invalid, DefaultSpawnLeashRadius, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected non-spawn actor homeward plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}

	outside.CombatProfile = StaticActorCombatProfilePracticeMob
	outside.CombatKind = StaticActorCombatProfilePracticeMob
	if plan, ok := PlanStaticActorSpawnLeashHomewardStep(outside, 0, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected non-positive leash radius homeward plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}
	if plan, ok := PlanStaticActorSpawnLeashHomewardStep(outside, DefaultSpawnLeashRadius, 0); ok || plan.Next.Valid() {
		t.Fatalf("expected non-positive max step homeward plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}
}

func TestPlanStaticActorSpawnChaseStepMovesTowardOwnerWithoutMutating(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	current := NewPosition(42, 1700, 2800)
	owner := NewPosition(42, 1900, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 40, Kind: EntityKindStaticActor, Name: "ChasePlannerMob"},
		Position:      current,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.chase_planner",
	}

	plan, ok := PlanStaticActorSpawnChaseStep(actor, owner, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected at-home spawn actor to produce a chase-step plan toward owner")
	}
	if plan.Evaluation.Status != SpawnLeashStatusAtHome {
		t.Fatalf("expected at-home evaluation in chase-step plan, got %+v", plan.Evaluation)
	}
	if plan.Next != NewPosition(42, 1800, 2800) || plan.Complete {
		t.Fatalf("expected one 100-unit x-axis chase step toward owner, got %+v", plan)
	}
	if actor.Position != current || actor.SpawnHome != home {
		t.Fatalf("expected chase-step planning not to mutate actor, got actor=%+v", actor)
	}
}

func TestPlanStaticActorSpawnChaseStepCompletesWhenAlreadyOnOwner(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	owner := NewPosition(42, 1700, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 41, Kind: EntityKindStaticActor, Name: "ChasePlannerOnOwnerMob"},
		Position:      owner,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.chase_planner_on_owner",
	}

	plan, ok := PlanStaticActorSpawnChaseStep(actor, owner, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected chase-step plan when actor already occupies owner position")
	}
	if plan.Next != owner || !plan.Complete {
		t.Fatalf("expected complete no-move chase plan on owner, got %+v", plan)
	}
}

func TestPlanStaticActorSpawnChaseStepCompletesWhenWithinOneStep(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	current := NewPosition(42, 1750, 2800)
	owner := NewPosition(42, 1800, 2800)
	actor := StaticEntity{
		Entity:        Entity{ID: 42, Kind: EntityKindStaticActor, Name: "ChasePlannerNearOwnerMob"},
		Position:      current,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.chase_planner_near",
	}

	plan, ok := PlanStaticActorSpawnChaseStep(actor, owner, DefaultSpawnLeashRadius, 100)
	if !ok {
		t.Fatal("expected near-owner chase-step plan")
	}
	if plan.Next != owner || !plan.Complete {
		t.Fatalf("expected one large chase step to land exactly on owner, got %+v", plan)
	}
}

func TestPlanStaticActorSpawnChaseStepClampsToLeashBoundary(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	current := NewPosition(42, 2000, 2800) // within default radius 400
	owner := NewPosition(42, 2300, 2800)   // would leave leash if chased uncapped
	actor := StaticEntity{
		Entity:        Entity{ID: 43, Kind: EntityKindStaticActor, Name: "ChasePlannerLeashClampMob"},
		Position:      current,
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.chase_planner_clamp",
	}

	plan, ok := PlanStaticActorSpawnChaseStep(actor, owner, DefaultSpawnLeashRadius, 500)
	if !ok {
		t.Fatal("expected leash-clamped chase-step plan")
	}
	if plan.Evaluation.Status != SpawnLeashStatusWithinRadius {
		t.Fatalf("expected within-radius evaluation before chase clamp, got %+v", plan.Evaluation)
	}
	if plan.Next != NewPosition(42, 2100, 2800) || !plan.Complete {
		t.Fatalf("expected chase step clamped to farthest on-segment point inside leash, got %+v", plan)
	}
	if evaluation, ok := EvaluateSpawnLeash(home, plan.Next, DefaultSpawnLeashRadius); !ok || evaluation.ReturnRequired {
		t.Fatalf("expected clamped chase next to remain inside leash, got ok=%v evaluation=%+v", ok, evaluation)
	}
}

func TestPlanStaticActorSpawnChaseStepFailsClosedForReturnRequiredOrCrossMap(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	returnRequired := StaticEntity{
		Entity:        Entity{ID: 44, Kind: EntityKindStaticActor, Name: "ChasePlannerReturnRequiredMob"},
		Position:      NewPosition(42, 2301, 2800),
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.chase_planner_return_required",
	}
	if plan, ok := PlanStaticActorSpawnChaseStep(returnRequired, NewPosition(42, 2400, 2800), DefaultSpawnLeashRadius, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected return-required chase-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}

	crossMap := StaticEntity{
		Entity:        Entity{ID: 45, Kind: EntityKindStaticActor, Name: "ChasePlannerCrossMapMob"},
		Position:      NewPosition(42, 1700, 2800),
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.chase_planner_cross_map",
	}
	if plan, ok := PlanStaticActorSpawnChaseStep(crossMap, NewPosition(43, 1700, 2800), DefaultSpawnLeashRadius, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected cross-map chase-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}
}

func TestPlanStaticActorSpawnChaseStepFailsClosedForInvalidInput(t *testing.T) {
	actor := StaticEntity{Entity: Entity{ID: 46, Kind: EntityKindStaticActor, Name: "ChasePlannerInvalidMob"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20350}
	owner := NewPosition(42, 1800, 2800)
	if plan, ok := PlanStaticActorSpawnChaseStep(actor, owner, DefaultSpawnLeashRadius, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected non-spawn actor chase-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}

	actor.CombatProfile = StaticActorCombatProfilePracticeMob
	actor.CombatKind = StaticActorCombatProfilePracticeMob
	actor.SpawnGroupRef = "practice.chase_planner_invalid"
	if plan, ok := PlanStaticActorSpawnChaseStep(actor, owner, 0, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected non-positive leash radius chase-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}
	if plan, ok := PlanStaticActorSpawnChaseStep(actor, owner, DefaultSpawnLeashRadius, 0); ok || plan.Next.Valid() {
		t.Fatalf("expected non-positive max step chase-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}
	if plan, ok := PlanStaticActorSpawnChaseStep(actor, Position{}, DefaultSpawnLeashRadius, 100); ok || plan.Next.Valid() {
		t.Fatalf("expected invalid owner position chase-step plan to fail closed, got ok=%v plan=%+v", ok, plan)
	}
}

func TestEvaluateStaticActorSpawnAggroAcquisitionAcquiresInsideDefaultRadius(t *testing.T) {
	actor := StaticEntity{
		Entity:        Entity{ID: 50, Kind: EntityKindStaticActor, Name: "AggroPlannerHomeMob"},
		Position:      NewPosition(42, 1700, 2800),
		SpawnHome:     NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.aggro_planner_home",
	}
	evaluation, ok := EvaluateStaticActorSpawnAggroAcquisition(actor, NewPosition(42, 1850, 2800), DefaultSpawnAggroRadius)
	if !ok {
		t.Fatal("expected in-radius same-map candidate to evaluate")
	}
	if !evaluation.Acquired || evaluation.Radius != DefaultSpawnAggroRadius || evaluation.Current != actor.Position {
		t.Fatalf("expected acquired aggro evaluation inside default radius, got %+v", evaluation)
	}
}

func TestEvaluateStaticActorSpawnAggroAcquisitionRejectsOutsideRadius(t *testing.T) {
	actor := StaticEntity{
		Entity:        Entity{ID: 51, Kind: EntityKindStaticActor, Name: "AggroPlannerOutsideMob"},
		Position:      NewPosition(42, 1700, 2800),
		SpawnHome:     NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.aggro_planner_outside",
	}
	evaluation, ok := EvaluateStaticActorSpawnAggroAcquisition(actor, NewPosition(42, 1901, 2800), DefaultSpawnAggroRadius)
	if !ok {
		t.Fatal("expected outside-radius candidate to evaluate")
	}
	if evaluation.Acquired {
		t.Fatalf("expected outside-radius candidate not to acquire, got %+v", evaluation)
	}
}

func TestEvaluateStaticActorSpawnAggroAcquisitionFailsClosedForReturnRequiredOrCrossMap(t *testing.T) {
	home := NewPosition(42, 1700, 2800)
	returnRequired := StaticEntity{
		Entity:        Entity{ID: 52, Kind: EntityKindStaticActor, Name: "AggroPlannerReturnRequiredMob"},
		Position:      NewPosition(42, 2301, 2800),
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.aggro_planner_return_required",
	}
	evaluation, ok := EvaluateStaticActorSpawnAggroAcquisition(returnRequired, NewPosition(42, 2301, 2800), DefaultSpawnAggroRadius)
	if !ok {
		t.Fatal("expected return-required actor to evaluate without mutating")
	}
	if evaluation.Acquired {
		t.Fatalf("expected return-required actor not to acquire, got %+v", evaluation)
	}

	crossMap := StaticEntity{
		Entity:        Entity{ID: 53, Kind: EntityKindStaticActor, Name: "AggroPlannerCrossMapMob"},
		Position:      NewPosition(42, 1700, 2800),
		SpawnHome:     home,
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.aggro_planner_cross_map",
	}
	evaluation, ok = EvaluateStaticActorSpawnAggroAcquisition(crossMap, NewPosition(43, 1700, 2800), DefaultSpawnAggroRadius)
	if !ok {
		t.Fatal("expected cross-map candidate to evaluate without mutating")
	}
	if evaluation.Acquired {
		t.Fatalf("expected cross-map candidate not to acquire, got %+v", evaluation)
	}
}

func TestEvaluateStaticActorSpawnAggroAcquisitionFailsClosedForInvalidInput(t *testing.T) {
	actor := StaticEntity{Entity: Entity{ID: 54, Kind: EntityKindStaticActor, Name: "AggroPlannerInvalidMob"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20350}
	if evaluation, ok := EvaluateStaticActorSpawnAggroAcquisition(actor, NewPosition(42, 1750, 2800), DefaultSpawnAggroRadius); ok || evaluation.Acquired {
		t.Fatalf("expected non-spawn actor aggro evaluation to fail closed, got ok=%v evaluation=%+v", ok, evaluation)
	}

	actor.CombatProfile = StaticActorCombatProfilePracticeMob
	actor.CombatKind = StaticActorCombatProfilePracticeMob
	actor.SpawnGroupRef = "practice.aggro_planner_invalid"
	if evaluation, ok := EvaluateStaticActorSpawnAggroAcquisition(actor, NewPosition(42, 1750, 2800), 0); ok || evaluation.Acquired {
		t.Fatalf("expected non-positive aggro radius to fail closed, got ok=%v evaluation=%+v", ok, evaluation)
	}
	if evaluation, ok := EvaluateStaticActorSpawnAggroAcquisition(actor, Position{}, DefaultSpawnAggroRadius); ok || evaluation.Acquired {
		t.Fatalf("expected invalid candidate position to fail closed, got ok=%v evaluation=%+v", ok, evaluation)
	}
}

func TestSelectStaticActorSpawnAggroCandidateChoosesNearestThenLowestEntityID(t *testing.T) {
	actor := StaticEntity{
		Entity:        Entity{ID: 55, Kind: EntityKindStaticActor, Name: "AggroPlannerSelectMob"},
		Position:      NewPosition(42, 1700, 2800),
		SpawnHome:     NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		CombatKind:    StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.aggro_planner_select",
	}
	selected, ok := SelectStaticActorSpawnAggroCandidate(actor, []SpawnAggroCandidate{
		{EntityID: 30, Position: NewPosition(42, 1850, 2800)},
		{EntityID: 20, Position: NewPosition(42, 1800, 2800)},
		{EntityID: 10, Position: NewPosition(42, 1800, 2800)},
		{EntityID: 40, Position: NewPosition(42, 2100, 2800)},
		{EntityID: 0, Position: NewPosition(42, 1750, 2800)},
	}, DefaultSpawnAggroRadius)
	if !ok {
		t.Fatal("expected nearest in-radius candidate to be selected")
	}
	if selected.EntityID != 10 || selected.Position != NewPosition(42, 1800, 2800) {
		t.Fatalf("expected ascending entity-id tie-break on nearest candidate, got %+v", selected)
	}
}

func TestEffectiveStaticActorSpawnAggroRadiusForActorUsesRegisteredProfile(t *testing.T) {
	const profile = "practice_spawn_aggro_actor_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		AggroRadius:  300,
	}) {
		t.Fatalf("expected %q profile registration to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	actor := StaticEntity{
		Entity:        Entity{ID: 77, Kind: EntityKindStaticActor, Name: "AuthoredAggroMob"},
		Position:      NewPosition(42, 1700, 2800),
		SpawnHome:     NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: profile,
		CombatKind:    profile,
		SpawnGroupRef: "practice.authored_aggro_actor",
	}
	if got := EffectiveStaticActorSpawnAggroRadiusForActor(actor); got != 300 {
		t.Fatalf("expected actor effective aggro radius 300, got %d", got)
	}

	inside, ok := EvaluateStaticActorSpawnAggroAcquisition(actor, NewPosition(42, 1950, 2800), EffectiveStaticActorSpawnAggroRadiusForActor(actor))
	if !ok || !inside.Acquired || inside.Radius != 300 {
		t.Fatalf("expected authored radius 300 to acquire at distance 250, got ok=%v evaluation=%+v", ok, inside)
	}
	outside, ok := EvaluateStaticActorSpawnAggroAcquisition(actor, NewPosition(42, 2050, 2800), EffectiveStaticActorSpawnAggroRadiusForActor(actor))
	if !ok || outside.Acquired || outside.Radius != 300 {
		t.Fatalf("expected authored radius 300 to reject distance 350, got ok=%v evaluation=%+v", ok, outside)
	}
}

func TestEffectiveStaticActorSpawnLeashRadiusForActorUsesRegisteredProfile(t *testing.T) {
	const profile = "practice_spawn_leash_actor_wolf"
	if !RegisterStaticActorCombatProfile(profile, StaticActorCombatProfileDefaults{
		MaxHP:        24,
		AttackValue:  8,
		DefenseValue: 2,
		RespawnDelay: PracticeMobBootstrapRespawnDelay,
		AggroRadius:  200,
		LeashRadius:  300,
	}) {
		t.Fatalf("expected %q profile registration to succeed", profile)
	}
	t.Cleanup(func() { UnregisterStaticActorCombatProfileForTest(profile) })

	actor := StaticEntity{
		Entity:        Entity{ID: 78, Kind: EntityKindStaticActor, Name: "AuthoredLeashMob"},
		Position:      NewPosition(42, 2001, 2800),
		SpawnHome:     NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: profile,
		CombatKind:    profile,
		SpawnGroupRef: "practice.authored_leash_actor",
	}
	if got := EffectiveStaticActorSpawnLeashRadiusForActor(actor); got != 300 {
		t.Fatalf("expected actor effective leash radius 300, got %d", got)
	}

	// Distance from home (1700,2800) to current (2001,2800) is 301: outside authored 300
	// but still inside bootstrap DefaultSpawnLeashRadius=400.
	evaluation, ok := EvaluateStaticActorCurrentSpawnLeash(actor, EffectiveStaticActorSpawnLeashRadiusForActor(actor))
	if !ok || !evaluation.ReturnRequired || evaluation.Radius != 300 {
		t.Fatalf("expected authored leash radius 300 to require return at distance 301, got ok=%v evaluation=%+v", ok, evaluation)
	}
	bootstrap, ok := EvaluateStaticActorCurrentSpawnLeash(actor, DefaultSpawnLeashRadius)
	if !ok || bootstrap.ReturnRequired {
		t.Fatalf("expected bootstrap leash 400 to keep distance 301 within radius, got ok=%v evaluation=%+v", ok, bootstrap)
	}
}
