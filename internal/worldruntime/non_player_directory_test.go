package worldruntime

import "testing"

func TestNonPlayerDirectoryRegistersLooksUpAndRemovesStaticActors(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 1, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}

	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}
	lookup, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok || lookup.Entity.Name != "VillageGuard" || lookup.RaceNum != 20300 {
		t.Fatalf("expected static actor lookup to return VillageGuard, got actor=%+v ok=%v", lookup, ok)
	}
	removed, ok := directory.Remove(actor.Entity.ID)
	if !ok || removed.Entity.ID != actor.Entity.ID {
		t.Fatalf("expected static actor remove to return VillageGuard, got actor=%+v ok=%v", removed, ok)
	}
	if _, ok := directory.ByEntityID(actor.Entity.ID); ok {
		t.Fatal("expected static actor lookup to be cleared after removal")
	}
}

func TestNonPlayerDirectoryLooksUpStaticActorsByVisibilityVID(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}

	lookup, ok := directory.ByVID(7)
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != actor.Entity.Name {
		t.Fatalf("expected static actor VID lookup to return VillageGuard, got actor=%+v ok=%v", lookup, ok)
	}
	if _, ok := directory.ByVID(999); ok {
		t.Fatal("expected missing static actor VID lookup to fail")
	}
}

func TestNonPlayerDirectoryRegisterClearsStaleVisibilityVIDsForSameEntityID(t *testing.T) {
	directory := NewNonPlayerDirectory()
	directory.entityIDByVID[7] = 13

	actor := StaticEntity{
		Entity:   Entity{ID: 13, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to repair stale VID ownership")
	}

	if _, ok := directory.ByVID(7); ok {
		t.Fatal("expected stale visibility VID lookup to be cleared after registration")
	}
	lookup, ok := directory.ByVID(13)
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != actor.Entity.Name {
		t.Fatalf("expected current visibility VID lookup to return VillageGuard, got actor=%+v ok=%v", lookup, ok)
	}
}

func TestNonPlayerDirectoryLookupPrunesStaleVisibilityVID(t *testing.T) {
	directory := NewNonPlayerDirectory()
	directory.entityIDByVID[7] = 13

	if actor, ok := directory.ByVID(7); ok {
		t.Fatalf("expected stale static actor VID lookup to fail, got %+v", actor)
	}
	if _, exists := directory.entityIDByVID[7]; exists {
		t.Fatal("expected stale static actor VID index to be pruned after lookup")
	}
}

func TestNonPlayerDirectoryLookupPrunesStaleVisibilityAliasesForSurvivingActor(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 13, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}
	directory.entityIDByVID[99] = actor.Entity.ID

	if lookup, ok := directory.ByVID(99); ok {
		t.Fatalf("expected stale non-canonical visibility VID alias to fail, got %+v", lookup)
	}
	if _, exists := directory.entityIDByVID[99]; exists {
		t.Fatal("expected stale non-canonical visibility VID alias to be pruned after lookup")
	}
	lookup, ok := directory.ByVID(uint32(actor.Entity.ID))
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != actor.Entity.Name {
		t.Fatalf("expected canonical visibility VID lookup to remain intact, got actor=%+v ok=%v", lookup, ok)
	}
}

func TestNonPlayerDirectoryRemovePrunesStaleVisibilityVIDWhenActorEntryMissing(t *testing.T) {
	directory := NewNonPlayerDirectory()
	directory.entityIDByVID[7] = 13
	directory.entityIDByVID[99] = 13

	if actor, ok := directory.Remove(13); ok {
		t.Fatalf("expected remove to report missing actor entry, got %+v", actor)
	}
	if _, exists := directory.entityIDByVID[7]; exists {
		t.Fatal("expected stale visibility VID 7 to be pruned during remove")
	}
	if _, exists := directory.entityIDByVID[99]; exists {
		t.Fatal("expected stale visibility VID 99 to be pruned during remove")
	}
}

func TestNonPlayerDirectoryRegisterPrunesOrphanedVisibilityVIDConflict(t *testing.T) {
	directory := NewNonPlayerDirectory()
	directory.entityIDByVID[7] = 999

	actor := StaticEntity{
		Entity:   Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to prune orphaned VID conflict")
	}
	lookup, ok := directory.ByVID(7)
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != actor.Entity.Name {
		t.Fatalf("expected current visibility VID lookup after orphan prune, got actor=%+v ok=%v", lookup, ok)
	}
}

func TestNonPlayerDirectoryRegisterPrunesNonCanonicalVisibilityVIDConflictForSurvivingActor(t *testing.T) {
	directory := NewNonPlayerDirectory()
	guard := StaticEntity{
		Entity:   Entity{ID: 13, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(guard) {
		t.Fatal("expected guard registration to succeed")
	}
	directory.entityIDByVID[99] = guard.Entity.ID

	blacksmith := StaticEntity{
		Entity:   Entity{ID: 99, Kind: EntityKindStaticActor, Name: "Blacksmith"},
		Position: NewPosition(42, 1900, 3000),
		RaceNum:  20301,
	}
	if !directory.Register(blacksmith) {
		t.Fatal("expected static actor registration to prune non-canonical VID conflict")
	}
	lookup, ok := directory.ByVID(99)
	if !ok || lookup.Entity.ID != blacksmith.Entity.ID || lookup.Entity.Name != blacksmith.Entity.Name {
		t.Fatalf("expected current visibility VID lookup after non-canonical prune, got actor=%+v ok=%v", lookup, ok)
	}
	guardLookup, ok := directory.ByVID(uint32(guard.Entity.ID))
	if !ok || guardLookup.Entity.ID != guard.Entity.ID || guardLookup.Entity.Name != guard.Entity.Name {
		t.Fatalf("expected guard canonical VID lookup to remain intact, got actor=%+v ok=%v", guardLookup, ok)
	}
}

func TestNonPlayerDirectoryUpdatePrunesOrphanedVisibilityVIDConflict(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}
	directory.entityIDByVID[7] = 999

	updated := actor
	updated.Entity.Name = "VillageGuardRenamed"
	if !directory.Update(updated) {
		t.Fatal("expected static actor update to prune orphaned VID conflict")
	}
	lookup, ok := directory.ByVID(7)
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != "VillageGuardRenamed" {
		t.Fatalf("expected current visibility VID lookup after update orphan prune, got actor=%+v ok=%v", lookup, ok)
	}
}

func TestNonPlayerDirectoryUpdatePrunesNonCanonicalVisibilityVIDConflictForSurvivingActor(t *testing.T) {
	directory := NewNonPlayerDirectory()
	guard := StaticEntity{
		Entity:   Entity{ID: 13, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	blacksmith := StaticEntity{
		Entity:   Entity{ID: 99, Kind: EntityKindStaticActor, Name: "Blacksmith"},
		Position: NewPosition(42, 1900, 3000),
		RaceNum:  20301,
	}
	if !directory.Register(guard) || !directory.Register(blacksmith) {
		t.Fatal("expected static actor registrations to succeed")
	}
	directory.entityIDByVID[99] = guard.Entity.ID

	updated := blacksmith
	updated.Entity.Name = "BlacksmithRenamed"
	if !directory.Update(updated) {
		t.Fatal("expected static actor update to prune non-canonical VID conflict")
	}
	lookup, ok := directory.ByVID(99)
	if !ok || lookup.Entity.ID != blacksmith.Entity.ID || lookup.Entity.Name != "BlacksmithRenamed" {
		t.Fatalf("expected current visibility VID lookup after update non-canonical prune, got actor=%+v ok=%v", lookup, ok)
	}
	guardLookup, ok := directory.ByVID(uint32(guard.Entity.ID))
	if !ok || guardLookup.Entity.ID != guard.Entity.ID || guardLookup.Entity.Name != guard.Entity.Name {
		t.Fatalf("expected guard canonical VID lookup to remain intact, got actor=%+v ok=%v", guardLookup, ok)
	}
}

func TestNonPlayerDirectoryUpdateReplacesStaticActorByEntityID(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}

	updated := actor
	updated.Entity.Name = "Blacksmith"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	if !directory.Update(updated) {
		t.Fatal("expected static actor update to succeed")
	}

	lookup, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected static actor lookup to succeed after update")
	}
	if lookup.Entity.Name != "Blacksmith" || lookup.Position != NewPosition(99, 900, 1200) || lookup.RaceNum != 20016 {
		t.Fatalf("unexpected static actor after update: %+v", lookup)
	}
}

func TestNonPlayerDirectoryUpdateRejectsActorThatStopsBeingVisibilityEncodable(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}
	if _, ok := directory.ByVID(7); !ok {
		t.Fatal("expected static actor VID lookup to exist before update")
	}

	updated := actor
	updated.RaceNum = uint32(^uint16(0)) + 1
	if directory.Update(updated) {
		t.Fatal("expected static actor update that stops being visibility-encodable to fail closed")
	}
	lookup, ok := directory.ByVID(7)
	if !ok || lookup.RaceNum != 20300 {
		t.Fatalf("expected original static actor VID lookup to remain after rejected update, got actor=%+v ok=%v", lookup, ok)
	}
}

func TestNonPlayerDirectoryUpdateClearsStaleVisibilityVIDsForSameEntityID(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 13, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}
	directory.entityIDByVID[99] = actor.Entity.ID

	updated := actor
	updated.Entity.Name = "Blacksmith"
	if !directory.Update(updated) {
		t.Fatal("expected static actor update to repair stale VID ownership")
	}

	if _, ok := directory.ByVID(99); ok {
		t.Fatal("expected stale visibility VID lookup to be cleared after update")
	}
	lookup, ok := directory.ByVID(13)
	if !ok || lookup.Entity.ID != actor.Entity.ID || lookup.Entity.Name != "Blacksmith" {
		t.Fatalf("expected current visibility VID lookup to return Blacksmith, got actor=%+v ok=%v", lookup, ok)
	}
}

func TestNonPlayerDirectoryRegisterDeepClonesDeathRewardDrops(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:        Entity{ID: 15, Kind: EntityKindStaticActor, Name: "RewardGuard"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.reward_guard",
		DeathReward:   StaticActorDeathReward{DropVnums: []uint32{27001, 27002}},
	}
	if !directory.Register(actor) {
		t.Fatal("expected reward actor registration to succeed")
	}
	actor.DeathReward.DropVnums[0] = 11111

	stored, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected stored reward actor to remain present")
	}
	if len(stored.DeathReward.DropVnums) != 2 || stored.DeathReward.DropVnums[0] != 27001 || stored.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected register to clone caller reward drops, got %+v", stored.DeathReward.DropVnums)
	}
}

func TestNonPlayerDirectoryUpdateDeepClonesDeathRewardDrops(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:        Entity{ID: 16, Kind: EntityKindStaticActor, Name: "RewardGuard"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.reward_guard",
	}
	if !directory.Register(actor) {
		t.Fatal("expected reward actor registration to succeed")
	}
	updated := actor
	updated.DeathReward = StaticActorDeathReward{DropVnums: []uint32{27001, 27002}}
	if !directory.Update(updated) {
		t.Fatal("expected reward actor update to succeed")
	}
	updated.DeathReward.DropVnums[1] = 22222

	stored, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected stored reward actor to remain present")
	}
	if len(stored.DeathReward.DropVnums) != 2 || stored.DeathReward.DropVnums[0] != 27001 || stored.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected update to clone caller reward drops, got %+v", stored.DeathReward.DropVnums)
	}
}

func TestNonPlayerDirectoryLookupsDeepCloneDeathRewardDrops(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:        Entity{ID: 17, Kind: EntityKindStaticActor, Name: "RewardGuard"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.reward_guard",
		DeathReward:   StaticActorDeathReward{DropVnums: []uint32{27001, 27002}},
	}
	if !directory.Register(actor) {
		t.Fatal("expected reward actor registration to succeed")
	}

	byEntityID, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected entity-id lookup to succeed")
	}
	byEntityID.DeathReward.DropVnums[0] = 11111

	byVID, ok := directory.ByVID(uint32(actor.Entity.ID))
	if !ok {
		t.Fatal("expected visibility-VID lookup to succeed")
	}
	byVID.DeathReward.DropVnums[1] = 22222

	actors := directory.StaticActors()
	actors[0].DeathReward.DropVnums[0] = 33333

	stored, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected stored reward actor to remain present")
	}
	if len(stored.DeathReward.DropVnums) != 2 || stored.DeathReward.DropVnums[0] != 27001 || stored.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected stored reward drops to stay cloned, got %+v", stored.DeathReward.DropVnums)
	}
}

func TestNonPlayerDirectoryRemoveClearsStaleVisibilityVIDsForSameEntityID(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:   Entity{ID: 14, Kind: EntityKindStaticActor, Name: "VillageGuard"},
		Position: NewPosition(42, 1700, 2800),
		RaceNum:  20300,
	}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}
	directory.entityIDByVID[99] = actor.Entity.ID

	removed, ok := directory.Remove(actor.Entity.ID)
	if !ok || removed.Entity.ID != actor.Entity.ID {
		t.Fatalf("expected static actor remove to return VillageGuard, got actor=%+v ok=%v", removed, ok)
	}
	if _, exists := directory.entityIDByVID[99]; exists {
		t.Fatal("expected stale visibility VID index to be cleared immediately after remove")
	}
	if _, exists := directory.entityIDByVID[14]; exists {
		t.Fatal("expected current visibility VID index to be cleared immediately after remove")
	}
}

func TestNonPlayerDirectoryPreservesCombatProfileOnRegisterAndUpdate(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:        Entity{ID: 9, Kind: EntityKindStaticActor, Name: "TrainingDummy"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfileTrainingDummy,
	}
	if !directory.Register(actor) {
		t.Fatal("expected combat-targetable static actor registration to succeed")
	}
	lookup, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected registered combat-targetable static actor lookup to succeed")
	}
	if lookup.CombatProfile != StaticActorCombatProfileTrainingDummy {
		t.Fatalf("expected training-dummy combat profile after register, got %+v", lookup)
	}

	updated := actor
	updated.CombatProfile = ""
	if !directory.Update(updated) {
		t.Fatal("expected combat-targetable static actor update to succeed")
	}
	lookup, ok = directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected updated combat-targetable static actor lookup to succeed")
	}
	if lookup.CombatProfile != "" {
		t.Fatalf("expected empty combat profile after update, got %+v", lookup)
	}
}

func TestNonPlayerDirectoryRejectsInvalidCombatProfile(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:        Entity{ID: 11, Kind: EntityKindStaticActor, Name: "BrokenDummy"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20350,
		CombatProfile: "boss",
	}
	if directory.Register(actor) {
		t.Fatal("expected invalid combat profile registration to fail closed")
	}
}

func TestNonPlayerDirectoryNormalizesLegacyCombatKindIntoCombatProfile(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:     Entity{ID: 12, Kind: EntityKindStaticActor, Name: "TrainingDummy"},
		Position:   NewPosition(42, 1700, 2800),
		RaceNum:    20350,
		CombatKind: StaticActorCombatKindTrainingDummy,
	}
	if !directory.Register(actor) {
		t.Fatal("expected legacy combat-kind registration to succeed")
	}
	lookup, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected legacy combat-kind lookup to succeed")
	}
	if lookup.CombatProfile != StaticActorCombatProfileTrainingDummy || lookup.CombatKind != StaticActorCombatKindTrainingDummy {
		t.Fatalf("expected legacy combat kind to normalize into combat profile, got %+v", lookup)
	}
}

func TestNonPlayerDirectoryPreservesSpawnGroupRefOnRegisterAndUpdate(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:        Entity{ID: 13, Kind: EntityKindStaticActor, Name: "PracticeMobAlpha"},
		Position:      NewPosition(42, 1800, 2900),
		RaceNum:       101,
		CombatProfile: StaticActorCombatProfileTrainingDummy,
		SpawnGroupRef: "practice.mob_alpha",
	}
	if !directory.Register(actor) {
		t.Fatal("expected spawn-group static actor registration to succeed")
	}
	lookup, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected spawn-group static actor lookup to succeed after register")
	}
	if lookup.SpawnGroupRef != "practice.mob_alpha" || lookup.CombatProfile != StaticActorCombatProfileTrainingDummy {
		t.Fatalf("expected spawn-group metadata after register, got %+v", lookup)
	}

	updated := actor
	updated.SpawnGroupRef = "practice.mob_beta"
	if !directory.Update(updated) {
		t.Fatal("expected spawn-group static actor update to succeed")
	}
	lookup, ok = directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected spawn-group static actor lookup to succeed after update")
	}
	if lookup.SpawnGroupRef != "practice.mob_beta" {
		t.Fatalf("expected updated spawn-group ref after update, got %+v", lookup)
	}
}

func TestNonPlayerDirectoryRejectsNonCanonicalSpawnGroupRef(t *testing.T) {
	for name, ref := range map[string]string{
		"blank segment":      "practice..mob",
		"uppercase segment":  "practice.Mob",
		"hyphenated segment": "practice-mob",
		"leading digit":      "1practice.mob",
		"missing namespace":  "practice_mob",
	} {
		t.Run(name, func(t *testing.T) {
			directory := NewNonPlayerDirectory()
			actor := StaticEntity{
				Entity:        Entity{ID: 14, Kind: EntityKindStaticActor, Name: "PracticeMobAlpha"},
				Position:      NewPosition(42, 1800, 2900),
				RaceNum:       101,
				CombatProfile: StaticActorCombatProfilePracticeMob,
				SpawnGroupRef: ref,
			}
			if directory.Register(actor) {
				t.Fatalf("expected spawn-backed actor with ref %q to be rejected", ref)
			}
		})
	}
}

func TestNonPlayerDirectoryRejectsStandaloneStaticActorDeathReward(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{
		Entity:        Entity{ID: 14, Kind: EntityKindStaticActor, Name: "RewardedStandaloneDummy"},
		Position:      NewPosition(42, 1800, 2900),
		RaceNum:       20350,
		CombatProfile: StaticActorCombatProfileTrainingDummy,
		DeathReward:   StaticActorDeathReward{Experience: 75},
	}
	if directory.Register(actor) {
		t.Fatal("expected standalone static actor reward metadata to be rejected")
	}
}

func TestNonPlayerDirectoryUpdateClonesDeathRewardDropVnums(t *testing.T) {
	directory := NewNonPlayerDirectory()
	actor := StaticEntity{Entity: Entity{ID: 15, Kind: EntityKindStaticActor, Name: "PracticeMob"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300, CombatProfile: StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.mob", DeathReward: StaticActorDeathReward{Experience: 75, Gold: 30, DropVnums: []uint32{27001}}}
	if !directory.Register(actor) {
		t.Fatal("expected static actor registration to succeed")
	}

	updated := actor
	updated.DeathReward = StaticActorDeathReward{Experience: 80, Gold: 35, DropVnums: []uint32{27002}}
	if !directory.Update(updated) {
		t.Fatal("expected static actor update to succeed")
	}
	updated.DeathReward.DropVnums[0] = 99999

	lookup, ok := directory.ByEntityID(actor.Entity.ID)
	if !ok {
		t.Fatal("expected updated static actor lookup to succeed")
	}
	if lookup.DeathReward.Experience != 80 || lookup.DeathReward.Gold != 35 || len(lookup.DeathReward.DropVnums) != 1 || lookup.DeathReward.DropVnums[0] != 27002 {
		t.Fatalf("expected update to clone death reward drop vnums, got %+v", lookup.DeathReward)
	}
}
