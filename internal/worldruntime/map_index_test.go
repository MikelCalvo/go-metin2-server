package worldruntime

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

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

func TestMapIndexUpdatePrunesDuplicatePlayerMapBuckets(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(7, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	if !index.Register(alpha) {
		t.Fatal("expected map-index registration to succeed")
	}
	stale := alpha
	stale.Character.MapIndex = 77
	stale.Character.X = 9999
	stale.Character.Y = 8888
	index.byMapIndex[77] = map[uint64]PlayerEntity{alpha.Entity.ID: stale}

	updated := alpha
	updated.Character.MapIndex = 99
	updated.Character.X = 1700
	updated.Character.Y = 2800
	if !index.Update(updated) {
		t.Fatal("expected player update to prune duplicate map-bucket ownership")
	}

	if characters := index.PlayerCharacters(42); len(characters) != 0 {
		t.Fatalf("expected old map player bucket to be empty after update, got %+v", characters)
	}
	if characters := index.PlayerCharacters(77); len(characters) != 0 {
		t.Fatalf("expected duplicate stale player bucket to be pruned after update, got %+v", characters)
	}
	characters := index.PlayerCharacters(99)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].X != 1700 || characters[0].Y != 2800 {
		t.Fatalf("expected updated Alpha only in map 99 bucket, got %+v", characters)
	}
}

func TestMapIndexUpdateRejectsStaticBucketCollisionWhenStaticEntityIndexMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(15, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	if !index.Register(alpha) {
		t.Fatal("expected player registration to succeed")
	}
	actor := StaticEntity{Entity: Entity{ID: alpha.Entity.ID, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(77, 1700, 2800), RaceNum: 20300}
	index.staticByMapIndex[77] = map[uint64]StaticEntity{actor.Entity.ID: actor}

	updated := alpha
	updated.Character.MapIndex = 99
	updated.Character.X = 1900
	updated.Character.Y = 3000
	if index.Update(updated) {
		t.Fatal("expected player update to reject surviving static actor map-bucket ownership")
	}

	characters := index.PlayerCharacters(42)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].MapIndex != 42 {
		t.Fatalf("expected original player map bucket to remain after rejected update, got %+v", characters)
	}
	actors := index.StaticActors(77)
	if len(actors) != 1 || actors[0].Entity.ID != actor.Entity.ID || actors[0].Entity.Name != "VillageGuard" {
		t.Fatalf("expected surviving static actor map bucket to remain after rejected player update, got %+v", actors)
	}
	if characters := index.PlayerCharacters(99); len(characters) != 0 {
		t.Fatalf("expected rejected player update not to insert destination bucket, got %+v", characters)
	}
}

func TestMapIndexUpdateRejectsStaticEntityIndexCollisionWhenStaticMapBucketMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := newPlayerEntity(16, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	if !index.Register(alpha) {
		t.Fatal("expected player registration to succeed")
	}
	actor := StaticEntity{Entity: Entity{ID: alpha.Entity.ID, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(77, 1700, 2800), RaceNum: 20300}
	index.staticByEntityID[actor.Entity.ID] = actor

	updated := alpha
	updated.Character.MapIndex = 99
	updated.Character.X = 1900
	updated.Character.Y = 3000
	if index.Update(updated) {
		t.Fatal("expected player update to reject surviving static actor entity-index ownership")
	}

	characters := index.PlayerCharacters(42)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].MapIndex != 42 {
		t.Fatalf("expected original player map bucket to remain after rejected entity-index update, got %+v", characters)
	}
	if stored, ok := index.StaticActor(actor.Entity.ID); !ok || stored.Entity.Name != "VillageGuard" {
		t.Fatalf("expected static actor entity index to remain after rejected player update, got actor=%+v ok=%v", stored, ok)
	}
	if characters := index.PlayerCharacters(99); len(characters) != 0 {
		t.Fatalf("expected rejected player update not to insert destination bucket, got %+v", characters)
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

func TestMapIndexRegisterClearsStaleMapBucketsForSameEntityID(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	stale := newPlayerEntity(13, entityRegistryCharacter("StaleAlpha", 0x02040101, 42, 1100, 2100))
	index.byMapIndex[42] = map[uint64]PlayerEntity{stale.Entity.ID: stale}

	registered := newPlayerEntity(stale.Entity.ID, entityRegistryCharacter("Alpha", 0x02040101, 77, 1700, 2800))
	if !index.Register(registered) {
		t.Fatal("expected player registration to clear stale map-bucket ownership")
	}

	if characters := index.PlayerCharacters(42); len(characters) != 0 {
		t.Fatalf("expected stale player map bucket to be cleared after registration, got %+v", characters)
	}
	characters := index.PlayerCharacters(77)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].MapIndex != 77 {
		t.Fatalf("expected registered player only in map 77 bucket, got %+v", characters)
	}
}

func TestMapIndexPlayerLookupPrunesDuplicateStaleMapBuckets(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	stale := newPlayerEntity(14, entityRegistryCharacter("StaleAlpha", 0x02040101, 42, 1100, 2100))
	current := stale
	current.Character = entityRegistryCharacter("Alpha", 0x02040101, 77, 1700, 2800)
	current.Entity.Name = current.Character.Name
	index.byEntityID[stale.Entity.ID] = current
	index.effectiveMapByEntityID[stale.Entity.ID] = 77
	index.byMapIndex[42] = map[uint64]PlayerEntity{stale.Entity.ID: stale}
	index.byMapIndex[77] = map[uint64]PlayerEntity{stale.Entity.ID: current}

	lookup, ok := index.Player(stale.Entity.ID)
	if !ok || lookup.Entity.Name != "Alpha" || lookup.Character.MapIndex != 77 {
		t.Fatalf("expected current player lookup, got player=%+v ok=%v", lookup, ok)
	}
	if characters := index.PlayerCharacters(42); len(characters) != 0 {
		t.Fatalf("expected stale player map bucket to be pruned after lookup, got %+v", characters)
	}
	characters := index.PlayerCharacters(77)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].MapIndex != 77 {
		t.Fatalf("expected current map bucket to remain after lookup prune, got %+v", characters)
	}
}

func TestMapIndexPlayerByVIDPrunesStaleAliasForSurvivingPlayer(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	stale := newPlayerEntity(16, entityRegistryCharacter("StaleAlpha", 0x02040101, 42, 1100, 2100))
	current := stale
	current.Character = entityRegistryCharacter("Alpha", 0x02040111, 77, 1700, 2800)
	current.Entity.VID = current.Character.VID
	current.Entity.Name = current.Character.Name
	index.byEntityID[stale.Entity.ID] = current
	index.effectiveMapByEntityID[stale.Entity.ID] = 77
	index.byMapIndex[42] = map[uint64]PlayerEntity{stale.Entity.ID: stale}
	index.byMapIndex[77] = map[uint64]PlayerEntity{stale.Entity.ID: current}

	if lookup, ok := index.PlayerByVID(stale.Entity.VID); ok {
		t.Fatalf("expected stale player VID alias to fail closed, got player=%+v", lookup)
	}
	if characters := index.PlayerCharacters(42); len(characters) != 0 {
		t.Fatalf("expected stale player map bucket to be pruned after old VID lookup, got %+v", characters)
	}
	lookup, ok := index.PlayerByVID(current.Entity.VID)
	if !ok || lookup.Entity.Name != "Alpha" || lookup.Character.MapIndex != 77 {
		t.Fatalf("expected current player VID lookup to remain intact, got player=%+v ok=%v", lookup, ok)
	}
	characters := index.PlayerCharacters(77)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].VID != current.Entity.VID {
		t.Fatalf("expected current player map bucket to remain after old VID prune, got %+v", characters)
	}
}

func TestMapIndexPlayerByNamePrunesStaleAliasForSurvivingPlayer(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	stale := newPlayerEntity(17, entityRegistryCharacter("StaleAlpha", 0x02040101, 42, 1100, 2100))
	current := stale
	current.Character = entityRegistryCharacter("Alpha", 0x02040101, 77, 1700, 2800)
	current.Entity.Name = current.Character.Name
	index.byEntityID[stale.Entity.ID] = current
	index.effectiveMapByEntityID[stale.Entity.ID] = 77
	index.byMapIndex[42] = map[uint64]PlayerEntity{stale.Entity.ID: stale}
	index.byMapIndex[77] = map[uint64]PlayerEntity{stale.Entity.ID: current}

	if lookup, ok := index.PlayerByName(stale.Entity.Name); ok {
		t.Fatalf("expected stale player name alias to fail closed, got player=%+v", lookup)
	}
	if characters := index.PlayerCharacters(42); len(characters) != 0 {
		t.Fatalf("expected stale player map bucket to be pruned after old name lookup, got %+v", characters)
	}
	lookup, ok := index.PlayerByName(current.Entity.Name)
	if !ok || lookup.Entity.Name != "Alpha" || lookup.Character.MapIndex != 77 {
		t.Fatalf("expected current player name lookup to remain intact, got player=%+v ok=%v", lookup, ok)
	}
	characters := index.PlayerCharacters(77)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].VID != current.Entity.VID {
		t.Fatalf("expected current player map bucket to remain after old name prune, got %+v", characters)
	}
}

func TestMapIndexRegisterRejectsStaticBucketCollisionWhenStaticEntityIndexMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	actor := StaticEntity{Entity: Entity{ID: 14, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	index.staticByMapIndex[42] = map[uint64]StaticEntity{actor.Entity.ID: actor}

	player := newPlayerEntity(actor.Entity.ID, entityRegistryCharacter("Alpha", 0x02040101, 77, 900, 1200))
	if index.Register(player) {
		t.Fatal("expected player registration to reject surviving static actor map-bucket ownership")
	}

	actors := index.StaticActors(42)
	if len(actors) != 1 || actors[0].Entity.ID != actor.Entity.ID || actors[0].Entity.Name != "VillageGuard" {
		t.Fatalf("expected original static actor map bucket to remain after rejected player registration, got %+v", actors)
	}
	if characters := index.PlayerCharacters(77); len(characters) != 0 {
		t.Fatalf("expected no player to be inserted after rejected collision, got %+v", characters)
	}
}

func TestMapIndexRegisterRejectsStaticEntityIndexCollisionWhenStaticMapBucketMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	actor := StaticEntity{Entity: Entity{ID: 15, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	index.staticByEntityID[actor.Entity.ID] = actor

	player := newPlayerEntity(actor.Entity.ID, entityRegistryCharacter("Alpha", 0x02040101, 77, 900, 1200))
	if index.Register(player) {
		t.Fatal("expected player registration to reject surviving static actor entity-index ownership")
	}

	if stored, ok := index.StaticActor(actor.Entity.ID); !ok || stored.Entity.Name != "VillageGuard" {
		t.Fatalf("expected original static actor entity index to remain after rejected player registration, got actor=%+v ok=%v", stored, ok)
	}
	if characters := index.PlayerCharacters(77); len(characters) != 0 {
		t.Fatalf("expected no player to be inserted after rejected entity-index collision, got %+v", characters)
	}
}

func TestMapIndexSnapshotPrunesDuplicatePlayerMapBuckets(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	stale := newPlayerEntity(14, entityRegistryCharacter("StaleAlpha", 0x02040101, 42, 1100, 2100))
	current := stale
	current.Character = entityRegistryCharacter("Alpha", 0x02040101, 77, 1700, 2800)
	current.Entity.Name = current.Character.Name
	index.byEntityID[stale.Entity.ID] = current
	index.effectiveMapByEntityID[stale.Entity.ID] = 77
	index.byMapIndex[42] = map[uint64]PlayerEntity{stale.Entity.ID: stale}
	index.byMapIndex[77] = map[uint64]PlayerEntity{stale.Entity.ID: current}

	snapshots := index.Snapshot()
	if len(snapshots) != 1 || snapshots[0].MapIndex != 77 || len(snapshots[0].Characters) != 1 || snapshots[0].Characters[0].Name != "Alpha" {
		t.Fatalf("expected map snapshot to prune stale player bucket and keep only current map 77, got %+v", snapshots)
	}
	if characters := index.PlayerCharacters(42); len(characters) != 0 {
		t.Fatalf("expected snapshot to prune stale player bucket from map 42, got %+v", characters)
	}
	characters := index.PlayerCharacters(77)
	if len(characters) != 1 || characters[0].Name != "Alpha" || characters[0].MapIndex != 77 {
		t.Fatalf("expected current player bucket to remain after snapshot repair, got %+v", characters)
	}
}

func TestMapIndexSnapshotPrunesDuplicateStaticActorMapBuckets(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	stale := StaticEntity{Entity: Entity{ID: 14, Kind: EntityKindStaticActor, Name: "StaleGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	current := stale
	current.Entity.Name = "VillageGuard"
	current.Position = NewPosition(77, 900, 1200)
	index.staticByEntityID[stale.Entity.ID] = current
	index.staticByMapIndex[42] = map[uint64]StaticEntity{stale.Entity.ID: stale}
	index.staticByMapIndex[77] = map[uint64]StaticEntity{stale.Entity.ID: current}

	snapshots := index.Snapshot()
	if len(snapshots) != 1 || snapshots[0].MapIndex != 77 || len(snapshots[0].StaticActors) != 1 || snapshots[0].StaticActors[0].Entity.Name != "VillageGuard" {
		t.Fatalf("expected map snapshot to prune stale static actor bucket and keep only current map 77, got %+v", snapshots)
	}
	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected snapshot to prune stale static actor bucket from map 42, got %+v", actors)
	}
	actors := index.StaticActors(77)
	if len(actors) != 1 || actors[0].Entity.Name != "VillageGuard" || actors[0].Position.MapIndex != 77 {
		t.Fatalf("expected current static actor bucket to remain after snapshot repair, got %+v", actors)
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

func TestMapIndexSnapshotClonesPlayerItemState(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	alpha := entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100)
	alpha.Inventory = append(alpha.Inventory, inventory.ItemInstance{ID: 101, Vnum: 27001, Count: 1})
	alpha.Equipment = append(alpha.Equipment, inventory.ItemInstance{ID: 201, Vnum: 11299, Count: 1})
	alpha.Quickslots = append(alpha.Quickslots, loginticket.Quickslot{Type: 1, Position: 2})
	if !index.Register(newPlayerEntity(1, alpha)) {
		t.Fatal("expected Alpha registration to succeed")
	}

	characters := index.PlayerCharacters(1)
	characters[0].Inventory[0].Vnum = 11111
	characters[0].Equipment[0].Vnum = 22222
	characters[0].Quickslots[0].Position = 9

	snapshots := index.Snapshot()
	if len(snapshots) != 1 || len(snapshots[0].Characters) != 1 {
		t.Fatalf("expected one player occupancy snapshot, got %+v", snapshots)
	}
	snapshots[0].Characters[0].Inventory[0].Count = 7
	snapshots[0].Characters[0].Equipment[0].Count = 8
	snapshots[0].Characters[0].Quickslots[0].Slot = 3

	stored := index.PlayerCharacters(1)
	if len(stored) != 1 {
		t.Fatalf("expected stored Alpha to remain present, got %+v", stored)
	}
	if stored[0].Inventory[0].Vnum != 27001 || stored[0].Inventory[0].Count != 1 {
		t.Fatalf("expected stored inventory to stay cloned, got %+v", stored[0].Inventory)
	}
	if stored[0].Equipment[0].Vnum != 11299 || stored[0].Equipment[0].Count != 1 {
		t.Fatalf("expected stored equipment to stay cloned, got %+v", stored[0].Equipment)
	}
	if stored[0].Quickslots[0].Position != 2 || stored[0].Quickslots[0].Slot != 0 {
		t.Fatalf("expected stored quickslots to stay cloned, got %+v", stored[0].Quickslots)
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

func TestMapIndexUpdateStaticRejectsPlayerBucketCollisionWhenEntityIndexMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	player := newPlayerEntity(7, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	if !index.Register(player) {
		t.Fatal("expected player registration to succeed")
	}
	delete(index.byEntityID, player.Entity.ID)

	actor := StaticEntity{Entity: Entity{ID: player.Entity.ID, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(99, 900, 1200), RaceNum: 20300}
	if index.UpdateStatic(actor) {
		t.Fatal("expected static actor update to reject a surviving player map-bucket collision")
	}

	characters := index.PlayerCharacters(42)
	if len(characters) != 1 || characters[0].Name != "Alpha" {
		t.Fatalf("expected original player map bucket to remain after rejected static update, got %+v", characters)
	}
	if actors := index.StaticActors(99); len(actors) != 0 {
		t.Fatalf("expected no static actor to be inserted after rejected collision, got %+v", actors)
	}
}

func TestMapIndexUpdateStaticRejectsPlayerEntityIndexCollisionWhenPlayerMapBucketMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	player := newPlayerEntity(8, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	index.byEntityID[player.Entity.ID] = player
	staleActor := StaticEntity{Entity: Entity{ID: player.Entity.ID, Kind: EntityKindStaticActor, Name: "StaleGuard"}, Position: NewPosition(77, 1500, 2500), RaceNum: 20301}
	index.staticByMapIndex[77] = map[uint64]StaticEntity{staleActor.Entity.ID: staleActor}

	actor := StaticEntity{Entity: Entity{ID: player.Entity.ID, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(99, 900, 1200), RaceNum: 20300}
	if index.UpdateStatic(actor) {
		t.Fatal("expected static actor update to reject a surviving player entity-index collision")
	}

	stored, ok := index.Player(player.Entity.ID)
	if !ok || stored.Entity.Name != "Alpha" {
		t.Fatalf("expected original player entity index to remain after rejected static update, got player=%+v ok=%v", stored, ok)
	}
	actors := index.StaticActors(77)
	if len(actors) != 1 || actors[0].Entity.Name != "StaleGuard" {
		t.Fatalf("expected original static actor map bucket to remain after rejected update, got %+v", actors)
	}
	if actors := index.StaticActors(99); len(actors) != 0 {
		t.Fatalf("expected no static actor to be inserted after rejected entity-index collision, got %+v", actors)
	}
}

func TestMapIndexUpdateStaticClearsStaleMapBucketsForSameEntityID(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 11, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}
	index.staticByMapIndex[99] = map[uint64]StaticEntity{guard.Entity.ID: guard}

	updated := guard
	updated.Position = NewPosition(77, 900, 1200)
	if !index.UpdateStatic(updated) {
		t.Fatal("expected static actor update to clear stale map-bucket ownership")
	}

	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected old map static bucket to be empty after update, got %+v", actors)
	}
	if actors := index.StaticActors(99); len(actors) != 0 {
		t.Fatalf("expected stale map static bucket to be cleared after update, got %+v", actors)
	}
	actors := index.StaticActors(77)
	if len(actors) != 1 || actors[0].Entity.ID != guard.Entity.ID || actors[0].Position.MapIndex != 77 {
		t.Fatalf("expected updated actor only in map 77 bucket, got %+v", actors)
	}
}

func TestMapIndexRemoveStaticClearsStaleMapBucketsForSameEntityID(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 12, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}
	index.staticByMapIndex[99] = map[uint64]StaticEntity{guard.Entity.ID: guard}

	removed, ok := index.RemoveStatic(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected tolerant static actor removal to return guard, got actor=%+v ok=%v", removed, ok)
	}
	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected original map static bucket to be cleared after remove, got %+v", actors)
	}
	if actors := index.StaticActors(99); len(actors) != 0 {
		t.Fatalf("expected stale map static bucket to be cleared after remove, got %+v", actors)
	}
	if snapshots := index.Snapshot(); len(snapshots) != 0 {
		t.Fatalf("expected no map snapshots after stale-bucket removal, got %+v", snapshots)
	}
}

func TestMapIndexRegisterStaticClearsStaleMapBucketsForSameEntityID(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	stale := StaticEntity{Entity: Entity{ID: 13, Kind: EntityKindStaticActor, Name: "StaleGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	index.staticByMapIndex[42] = map[uint64]StaticEntity{stale.Entity.ID: stale}

	registered := stale
	registered.Entity.Name = "VillageGuard"
	registered.Position = NewPosition(77, 900, 1200)
	if !index.RegisterStatic(registered) {
		t.Fatal("expected static actor registration to clear stale map-bucket ownership")
	}

	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected stale map static bucket to be cleared after registration, got %+v", actors)
	}
	actors := index.StaticActors(77)
	if len(actors) != 1 || actors[0].Entity.ID != stale.Entity.ID || actors[0].Entity.Name != "VillageGuard" || actors[0].Position.MapIndex != 77 {
		t.Fatalf("expected registered actor only in map 77 bucket, got %+v", actors)
	}
}

func TestMapIndexRegisterStaticRejectsPlayerBucketCollisionWhenPlayerEntityIndexMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	player := newPlayerEntity(14, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	index.byMapIndex[42] = map[uint64]PlayerEntity{player.Entity.ID: player}

	actor := StaticEntity{Entity: Entity{ID: player.Entity.ID, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(77, 900, 1200), RaceNum: 20300}
	if index.RegisterStatic(actor) {
		t.Fatal("expected static actor registration to reject surviving player map-bucket ownership")
	}

	characters := index.PlayerCharacters(42)
	if len(characters) != 1 || characters[0].Name != "Alpha" {
		t.Fatalf("expected original player map bucket to remain after rejected static registration, got %+v", characters)
	}
	if actors := index.StaticActors(77); len(actors) != 0 {
		t.Fatalf("expected no static actor to be inserted after rejected collision, got %+v", actors)
	}
}

func TestMapIndexRegisterStaticRejectsPlayerEntityIndexCollisionWhenPlayerMapBucketMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	player := newPlayerEntity(15, entityRegistryCharacter("Alpha", 0x02040101, 42, 1100, 2100))
	index.byEntityID[player.Entity.ID] = player

	actor := StaticEntity{Entity: Entity{ID: player.Entity.ID, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(77, 900, 1200), RaceNum: 20300}
	if index.RegisterStatic(actor) {
		t.Fatal("expected static actor registration to reject surviving player entity-index ownership")
	}

	stored, ok := index.Player(player.Entity.ID)
	if !ok || stored.Entity.Name != "Alpha" {
		t.Fatalf("expected original player entity index to remain after rejected static registration, got player=%+v ok=%v", stored, ok)
	}
	if actors := index.StaticActors(77); len(actors) != 0 {
		t.Fatalf("expected no static actor to be inserted after rejected entity-index collision, got %+v", actors)
	}
}

func TestMapIndexStaticActorLookupPrunesDuplicateStaleMapBuckets(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	stale := StaticEntity{Entity: Entity{ID: 14, Kind: EntityKindStaticActor, Name: "StaleGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	current := stale
	current.Entity.Name = "VillageGuard"
	current.Position = NewPosition(77, 900, 1200)
	index.staticByEntityID[stale.Entity.ID] = current
	index.staticByMapIndex[42] = map[uint64]StaticEntity{stale.Entity.ID: stale}
	index.staticByMapIndex[77] = map[uint64]StaticEntity{stale.Entity.ID: current}

	lookup, ok := index.StaticActor(stale.Entity.ID)
	if !ok || lookup.Entity.Name != "VillageGuard" || lookup.Position.MapIndex != 77 {
		t.Fatalf("expected current static actor lookup, got actor=%+v ok=%v", lookup, ok)
	}
	if actors := index.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected stale map static bucket to be pruned after lookup, got %+v", actors)
	}
	actors := index.StaticActors(77)
	if len(actors) != 1 || actors[0].Entity.ID != stale.Entity.ID || actors[0].Entity.Name != "VillageGuard" {
		t.Fatalf("expected current map bucket to remain after lookup prune, got %+v", actors)
	}
}

func TestMapIndexStaticActorByVIDFindsMapOnlyPresence(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 15, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}
	delete(index.staticByEntityID, guard.Entity.ID)

	lookup, ok := index.StaticActorByVID(uint32(guard.Entity.ID))
	if !ok || lookup.Entity.ID != guard.Entity.ID || lookup.Entity.Name != "VillageGuard" {
		t.Fatalf("expected visibility VID lookup through surviving map bucket, got actor=%+v ok=%v", lookup, ok)
	}
	lookup.RaceNum = 99999
	second, ok := index.StaticActorByVID(uint32(guard.Entity.ID))
	if !ok || second.RaceNum != 20300 {
		t.Fatalf("expected map-only visibility lookup to clone actor snapshots, got actor=%+v ok=%v", second, ok)
	}
}

func TestMapIndexAllStaticActorsIncludesMapOnlyPresence(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{Entity: Entity{ID: 16, Kind: EntityKindStaticActor, Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}
	delete(index.staticByEntityID, guard.Entity.ID)

	actors := index.AllStaticActors()
	if len(actors) != 1 || actors[0].Entity.ID != guard.Entity.ID || actors[0].Entity.Name != "VillageGuard" {
		t.Fatalf("expected all-static snapshot to include surviving map-only actor, got %+v", actors)
	}
	actors[0].RaceNum = 99999
	lookup, ok := index.StaticActor(guard.Entity.ID)
	if !ok || lookup.RaceNum != 20300 {
		t.Fatalf("expected all-static snapshot to be cloned from map-index state, got actor=%+v ok=%v", lookup, ok)
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

func TestMapIndexStaticActorLookupClonesDeathRewardDropVnumsWhenEntityIndexIsMissing(t *testing.T) {
	index := NewMapIndex(NewBootstrapTopology(0))
	guard := StaticEntity{
		Entity:        Entity{ID: 12, Kind: EntityKindStaticActor, Name: "PracticeMob"},
		Position:      NewPosition(42, 1700, 2800),
		RaceNum:       20300,
		CombatProfile: StaticActorCombatProfilePracticeMob,
		SpawnGroupRef: "practice.mob_delta",
		DeathReward:   StaticActorDeathReward{Experience: 75, Gold: 60, DropVnums: []uint32{27001, 27002}},
	}
	if !index.RegisterStatic(guard) {
		t.Fatal("expected static actor registration to succeed")
	}
	delete(index.staticByEntityID, guard.Entity.ID)

	lookup, ok := index.StaticActor(guard.Entity.ID)
	if !ok {
		t.Fatal("expected static actor lookup through surviving map bucket to succeed")
	}
	lookup.DeathReward.DropVnums[0] = 99999

	lookup, ok = index.StaticActor(guard.Entity.ID)
	if !ok {
		t.Fatal("expected second static actor lookup through surviving map bucket to succeed")
	}
	if len(lookup.DeathReward.DropVnums) != 2 || lookup.DeathReward.DropVnums[0] != 27001 || lookup.DeathReward.DropVnums[1] != 27002 {
		t.Fatalf("expected static actor lookup to clone reward drops from surviving map bucket, got %+v", lookup.DeathReward.DropVnums)
	}
}
