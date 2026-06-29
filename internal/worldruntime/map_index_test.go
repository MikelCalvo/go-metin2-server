package worldruntime

import "testing"

func TestMapIndexRegistersPlayersIntoEffectiveMapBuckets(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(1, entityRegistryCharacter("Alpha", 0x02040101, 0, 1100, 2100))
	bravo := newPlayerEntity(2, entityRegistryCharacter("Bravo", 0x02040102, 42, 1300, 2300))

	if !index.Register(alpha) || !index.Register(bravo) {
		t.Fatal("expected map-index registration to succeed")
	}

	bootstrapCharacters := index.PlayerCharacters(0)
	if len(bootstrapCharacters) != 1 || bootstrapCharacters[0].Name != "Alpha" || bootstrapCharacters[0].MapIndex != 0 {
		t.Fatalf("expected Alpha in the effective bootstrap bucket, got %+v", bootstrapCharacters)
	}
	map42Characters := index.PlayerCharacters(42)
	if len(map42Characters) != 1 || map42Characters[0].Name != "Bravo" {
		t.Fatalf("expected Bravo in map 42 bucket, got %+v", map42Characters)
	}
}

func TestMapIndexUpdateMovesPlayersBetweenMapBuckets(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(1, entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	if !index.Register(alpha) {
		t.Fatal("expected map-index registration to succeed")
	}

	moved := alpha
	moved.Character.MapIndex = 42
	moved.Character.X = 1700
	moved.Character.Y = 2800
	if !index.Update(moved) {
		t.Fatal("expected map-index update to succeed")
	}

	if bootstrapCharacters := index.PlayerCharacters(1); len(bootstrapCharacters) != 0 {
		t.Fatalf("expected bootstrap bucket to be empty after move, got %+v", bootstrapCharacters)
	}
	map42Characters := index.PlayerCharacters(42)
	if len(map42Characters) != 1 || map42Characters[0].Name != "Alpha" || map42Characters[0].X != 1700 || map42Characters[0].Y != 2800 {
		t.Fatalf("expected moved Alpha in map 42 bucket, got %+v", map42Characters)
	}
}

func TestMapIndexUpdateRepairsEntityIndexWhenMapBucketSurvives(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(7, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	if !index.Register(alpha) {
		t.Fatal("expected map-index registration to succeed")
	}
	delete(index.byEntityID, alpha.Entity.ID)

	updated := alpha
	updated.Character.MapIndex = 99
	updated.Character.X = 1700
	updated.Character.Y = 2800
	if !index.Update(updated) {
		t.Fatal("expected player update to repair surviving map-bucket ownership")
	}

	if characters := index.PlayerCharacters(42); len(characters) != 0 {
		t.Fatalf("expected old map player bucket to be empty after repaired update, got %+v", characters)
	}
	characters := index.PlayerCharacters(99)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].X != 1700 || characters[0].Y != 2800 {
		t.Fatalf("expected updated Alpha in new map bucket after repair, got %+v", characters)
	}
	stored, ok := index.byEntityID[alpha.Entity.ID]
	if !ok || stored.Character.MapIndex != 99 || stored.Character.X != 1700 || stored.Character.Y != 2800 {
		t.Fatalf("expected repaired entity index to point at updated player, got entity=%+v ok=%v", stored, ok)
	}
}

func TestMapIndexRemoveClearsOccupancy(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(1, entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	if !index.Register(alpha) {
		t.Fatal("expected map-index registration to succeed")
	}

	removed, ok := index.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected remove to return Alpha, got entity=%+v ok=%v", removed, ok)
	}
	if bootstrapCharacters := index.PlayerCharacters(1); len(bootstrapCharacters) != 0 {
		t.Fatalf("expected bootstrap bucket to be empty after removal, got %+v", bootstrapCharacters)
	}
	if snapshots := index.Snapshot(); len(snapshots) != 0 {
		t.Fatalf("expected no map snapshots after removal, got %+v", snapshots)
	}
}

func TestMapIndexRemoveClearsMapBucketWhenEntityIndexAlreadyMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(7, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	if !index.Register(alpha) {
		t.Fatal("expected map-index registration to succeed")
	}
	delete(index.byEntityID, alpha.Entity.ID)

	removed, ok := index.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected tolerant removal after entity-index loss, got entity=%+v ok=%v", removed, ok)
	}
	if characters := index.PlayerCharacters(42); len(characters) != 0 {
		t.Fatalf("expected player map bucket to be cleared after tolerant removal, got %+v", characters)
	}
	if _, ok := index.effectiveMapByEntityID[alpha.Entity.ID]; ok {
		t.Fatal("expected effective map index entry to be cleared after tolerant removal")
	}
	if snapshots := index.Snapshot(); len(snapshots) != 0 {
		t.Fatalf("expected no map snapshots after tolerant player removal, got %+v", snapshots)
	}
}

func TestMapIndexRemoveClearsEntityIndexWhenMapBucketAlreadyMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(8, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	if !index.Register(alpha) {
		t.Fatal("expected map-index registration to succeed")
	}
	delete(index.byMapIndex, uint32(42))

	removed, ok := index.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected tolerant removal after map-bucket loss, got entity=%+v ok=%v", removed, ok)
	}
	if _, ok := index.byEntityID[alpha.Entity.ID]; ok {
		t.Fatal("expected entity index entry to be cleared after tolerant removal")
	}
	if _, ok := index.effectiveMapByEntityID[alpha.Entity.ID]; ok {
		t.Fatal("expected effective map index entry to be cleared after tolerant removal")
	}
	if snapshots := index.Snapshot(); len(snapshots) != 0 {
		t.Fatalf("expected no map snapshots after tolerant player removal, got %+v", snapshots)
	}
}

func TestMapIndexSnapshotReturnsStableSortedCharactersPerMap(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	if !index.Register(newPlayerEntity(3, entityRegistryCharacter("Zulu", 0x02040103, 42, 1900, 3000))) {
		t.Fatal("expected Zulu registration to succeed")
	}
	if !index.Register(newPlayerEntity(1, entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))) {
		t.Fatal("expected Alpha registration to succeed")
	}
	if !index.Register(newPlayerEntity(2, entityRegistryCharacter("Bravo", 0x02040102, 1, 1300, 2300))) {
		t.Fatal("expected Bravo registration to succeed")
	}

	snapshots := index.Snapshot()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 map snapshots, got %d", len(snapshots))
	}
	if snapshots[0].MapIndex != 1 || snapshots[1].MapIndex != 42 {
		t.Fatalf("expected snapshot map order [1 42], got %+v", snapshots)
	}
	if len(snapshots[0].Characters) != 2 || snapshots[0].Characters[0].Name != "Alpha" || snapshots[0].Characters[1].Name != "Bravo" {
		t.Fatalf("expected sorted characters in bootstrap snapshot, got %+v", snapshots[0].Characters)
	}
	if len(snapshots[1].Characters) != 1 || snapshots[1].Characters[0].Name != "Zulu" {
		t.Fatalf("expected Zulu-only snapshot for map 42, got %+v", snapshots[1].Characters)
	}
}

func TestMapIndexTracksStaticActorsByEffectiveMap(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 1, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	blacksmith := StaticEntity{Entity: Entity{ID: 2, Kind: EntityKindStaticActor, Name: "Blacksmith"}, Position: NewPosition(42, 1900, 3000), RaceNum: 20301}

	if !index.RegisterStatic(guard) || !index.RegisterStatic(blacksmith) {
		t.Fatal("expected static actor map-index registration to succeed")
	}
	actors := index.StaticActors(42)
	if len(actors) != 2 || actors[0].Entity.Name != "Blacksmith" || actors[1].Entity.Name != "VillageGuard" {
		t.Fatalf("expected stable sorted static actors in map bucket, got %+v", actors)
	}
	removed, ok := index.RemoveStatic(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected static actor removal to return VillageGuard, got actor=%+v ok=%v", removed, ok)
	}
	actors = index.StaticActors(42)
	if len(actors) != 1 || actors[0].Entity.Name != "Blacksmith" {
		t.Fatalf("expected only Blacksmith after guard removal, got %+v", actors)
	}
}

func TestMapIndexRemoveStaticClearsMapBucketWhenEntityIndexAlreadyMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor map-index registration to succeed")
	}
	delete(index.staticByEntityID, guard.Entity.ID)

	removed, ok := index.RemoveStatic(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected tolerant static actor removal after entity-index loss, got actor=%+v ok=%v", removed, ok)
	}
	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected map static actor bucket to be cleared after tolerant removal, got %+v", actors)
	}
	if snapshots := index.Snapshot(); len(snapshots) != 0 {
		t.Fatalf("expected no map snapshots after tolerant static actor removal, got %+v", snapshots)
	}
}

func TestMapIndexRemoveStaticClearsEntityIndexWhenMapBucketAlreadyMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 8, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor map-index registration to succeed")
	}
	delete(index.staticByMapIndex, uint32(42))

	removed, ok := index.RemoveStatic(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected tolerant static actor removal after map-bucket loss, got actor=%+v ok=%v", removed, ok)
	}
	if _, ok := index.staticByEntityID[guard.Entity.ID]; ok {
		t.Fatal("expected static actor entity index to be cleared after tolerant removal")
	}
	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected map static actor bucket to stay empty after tolerant removal, got %+v", actors)
	}
	if snapshots := index.Snapshot(); len(snapshots) != 0 {
		t.Fatalf("expected no map snapshots after tolerant static actor removal, got %+v", snapshots)
	}
}

func TestMapIndexSnapshotIncludesStaticActorsAndStaticOnlyMaps(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	if !index.Register(newPlayerEntity(1, entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))) {
		t.Fatal("expected Alpha registration to succeed")
	}
	guard := StaticEntity{Entity: Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}

	snapshots := index.Snapshot()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 map snapshots including static-only map, got %d", len(snapshots))
	}
	if snapshots[0].MapIndex != 1 || len(snapshots[0].Characters) != 1 || snapshots[0].Characters[0].Name != "Alpha" {
		t.Fatalf("unexpected player map snapshot: %+v", snapshots[0])
	}
	if len(snapshots[0].StaticActors) != 0 {
		t.Fatalf("expected no static actors on player map snapshot, got %+v", snapshots[0].StaticActors)
	}
	if snapshots[1].MapIndex != 42 || len(snapshots[1].Characters) != 0 {
		t.Fatalf("expected static-only map snapshot on 42, got %+v", snapshots[1])
	}
	if len(snapshots[1].StaticActors) != 1 || snapshots[1].StaticActors[0].Entity.ID != guard.Entity.ID || snapshots[1].StaticActors[0].Entity.Name != "VillageGuard" {
		t.Fatalf("expected static actor in map 42 snapshot, got %+v", snapshots[1].StaticActors)
	}
}

func TestMapIndexUpdateStaticMovesActorsBetweenMapBuckets(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}

	updated := guard
	updated.Entity.Name = "Blacksmith"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	if !index.UpdateStatic(updated) {
		t.Fatal("expected static actor update to succeed")
	}

	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected old map static bucket to be empty after update, got %+v", actors)
	}
	actors := index.StaticActors(99)
	if len(actors) != 1 || actors[0].Entity.ID != guard.Entity.ID || actors[0].Entity.Name != "Blacksmith" || actors[0].Position != NewPosition(99, 900, 1200) || actors[0].RaceNum != 20016 {
		t.Fatalf("expected updated actor in new map bucket, got %+v", actors)
	}
}

func TestMapIndexUpdateStaticRepairsEntityIndexWhenMapBucketSurvives(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 7, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}
	delete(index.staticByEntityID, guard.Entity.ID)

	updated := guard
	updated.Entity.Name = "Blacksmith"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	if !index.UpdateStatic(updated) {
		t.Fatal("expected static actor update to repair surviving map-bucket ownership")
	}

	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected old map static bucket to be empty after repaired update, got %+v", actors)
	}
	actors := index.StaticActors(99)
	if len(actors) != 1 || actors[0].Entity.ID != guard.Entity.ID || actors[0].Entity.Name != "Blacksmith" || actors[0].Position != NewPosition(99, 900, 1200) || actors[0].RaceNum != 20016 {
		t.Fatalf("expected updated actor in new map bucket after repair, got %+v", actors)
	}
	stored, ok := index.StaticActor(guard.Entity.ID)
	if !ok || stored.Entity.Name != "Blacksmith" || stored.Position != NewPosition(99, 900, 1200) {
		t.Fatalf("expected repaired entity index to point at updated actor, got actor=%+v ok=%v", stored, ok)
	}
}

func TestMapIndexRegisterStaticClonesDeathRewardDropVnums(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{
		Entity:        Entity{ID: 9, Kind: EntityKindStaticActor, Name: "PracticeMob"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.mob_alpha",
		DeathReward:   StaticActorDeathReward{Experience: 75, Gold: 60, DropVnums: []uint32{27001, 27002}},
	}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}
	guard.DeathReward.DropVnums[0] = 99999

	actors := index.StaticActors(42)
	if len(actors) != 1 {
		t.Fatalf("expected one static actor in map bucket, got %+v", actors)
	}
	if len(actors[0].DeathReward.DropVnums) != 2 || actors[0].DeathReward.DropVnums[0] != 27001 || actors[0].DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected registered reward drops to be cloned, got %+v", actors[0].DeathReward.DropVnums)
	}
}

func TestMapIndexUpdateStaticClonesDeathRewardDropVnums(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 10, Kind: EntityKindStaticActor, Name: "PracticeMob"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300, CombatProfile: StaticActorCombatProfilePracticeMob, SpawnGroupRef: "practice.mob_beta"}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}

	updated := guard
	updated.DeathReward = StaticActorDeathReward{Experience: 80, Gold: 65, DropVnums: []uint32{27003, 27004}}
	if !index.UpdateStatic(updated) {
		t.Fatal("expected static actor update to succeed")
	}
	updated.DeathReward.DropVnums[0] = 99999

	actors := index.StaticActors(42)
	if len(actors) != 1 {
		t.Fatalf("expected one static actor in map bucket, got %+v", actors)
	}
	if len(actors[0].DeathReward.DropVnums) != 2 || actors[0].DeathReward.DropVnums[0] != 27003 || actors[0].DeathReward.DropVnums[1] != 27004 {
		t.Fatalf("expected updated reward drops to be cloned, got %+v", actors[0].DeathReward.DropVnums)
	}
}

func TestMapIndexStaticActorSnapshotsCloneDeathRewardDropVnums(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{
		Entity:        Entity{ID: 11, Kind: EntityKindStaticActor, Name: "PracticeMob"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.mob_gamma",
		DeathReward:   StaticActorDeathReward{Experience: 75, Gold: 60, DropVnums: []uint32{27001, 27002}},
	}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}

	actors := index.StaticActors(42)
	actors[0].DeathReward.DropVnums[0] = 99999
	snapshots := index.Snapshot()
	if len(snapshots) != 1 || len(snapshots[0].StaticActors) != 1 {
		t.Fatalf("expected one static actor snapshot, got %+v", snapshots)
	}
	snapshots[0].StaticActors[0].DeathReward.DropVnums[1] = 88888

	actors = index.StaticActors(42)
	if len(actors[0].DeathReward.DropVnums) != 2 || actors[0].DeathReward.DropVnums[0] != 27001 || actors[0].DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected static actor reward snapshots to be isolated from callers, got %+v", actors[0].DeathReward.DropVnums)
	}
}
