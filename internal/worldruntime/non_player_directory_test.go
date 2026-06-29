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

func TestNonPlayerDirectoryUpdateClearsVisibilityVIDLookupWhenActorStopsBeingEncodable(t *testing.T) {
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
	if !directory.Update(updated) {
		t.Fatal("expected static actor update to succeed even when actor stops being visibility-encodable")
	}
	if _, ok := directory.ByVID(7); ok {
		t.Fatal("expected static actor VID lookup to be cleared after actor stops being visibility-encodable")
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
