package worldruntime

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestEntityRegistryRegistersLooksUpUpdatesAndRemovesPlayerEntities(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	if alpha.Entity.Kind != EntityKindPlayer || alpha.Entity.ID == 0 || alpha.Entity.VID != 0x02040101 || alpha.Entity.Name != "Alpha" {
		t.Fatalf("unexpected registered player entity: %+v", alpha)
	}

	lookup, ok := registry.Player(alpha.Entity.ID)
	if !ok || lookup.Entity.Name != "Alpha" || lookup.Character.X != 1100 || lookup.Character.Y != 2100 {
		t.Fatalf("expected lookup to return registered player entity, got entity=%+v ok=%v", lookup, ok)
	}

	updated := lookup.Character
	updated.MapIndex = 42
	updated.X = 1700
	updated.Y = 2800
	if !registry.UpdatePlayer(alpha.Entity.ID, updated) {
		t.Fatal("expected player update to succeed")
	}
	lookup, ok = registry.Player(alpha.Entity.ID)
	if !ok || lookup.Character.MapIndex != 42 || lookup.Character.X != 1700 || lookup.Character.Y != 2800 {
		t.Fatalf("expected updated player entity snapshot, got entity=%+v ok=%v", lookup, ok)
	}

	removed, ok := registry.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected remove to return registered entity, got entity=%+v ok=%v", removed, ok)
	}
	if _, ok := registry.Player(alpha.Entity.ID); ok {
		t.Fatal("expected removed player entity to disappear from lookup")
	}
}

func TestEntityRegistryLooksUpPlayersByVIDAndExactName(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 42, 1300, 2300))

	byVID, ok := registry.PlayerByVID(bravo.Entity.VID)
	if !ok || byVID.Entity.ID != bravo.Entity.ID || byVID.Entity.Name != "Bravo" {
		t.Fatalf("expected VID lookup to return Bravo, got entity=%+v ok=%v", byVID, ok)
	}
	byName, ok := registry.PlayerByName(alpha.Entity.Name)
	if !ok || byName.Entity.ID != alpha.Entity.ID || byName.Entity.VID != alpha.Entity.VID {
		t.Fatalf("expected exact-name lookup to return Alpha, got entity=%+v ok=%v", byName, ok)
	}
}

func TestEntityRegistryReturnsDeterministicSortedPlayerCharacters(t *testing.T) {
	registry := NewEntityRegistry()
	registry.RegisterPlayer(entityRegistryCharacter("Zulu", 0x02040103, 42, 1900, 3000))
	registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 1, 1300, 2300))

	characters := registry.PlayerCharacters()
	if len(characters) != 3 {
		t.Fatalf("expected 3 player characters, got %d", len(characters))
	}
	if characters[0].Name != "Alpha" || characters[1].Name != "Bravo" || characters[2].Name != "Zulu" {
		t.Fatalf("expected deterministic player character order, got %+v", characters)
	}
}

func TestEntityRegistryTracksPlayersByEffectiveMapIndex(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 0, 1100, 2100))
	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 42, 1300, 2300))

	bootstrapCharacters := registry.MapCharacters(0)
	if len(bootstrapCharacters) != 1 || bootstrapCharacters[0].Name != "Alpha" {
		t.Fatalf("expected Alpha in effective bootstrap map bucket, got %+v", bootstrapCharacters)
	}
	map42Characters := registry.MapCharacters(42)
	if len(map42Characters) != 1 || map42Characters[0].Name != "Bravo" {
		t.Fatalf("expected Bravo in map 42 bucket, got %+v", map42Characters)
	}

	moved := alpha.Character
	moved.MapIndex = 42
	moved.X = 1700
	moved.Y = 2800
	if !registry.UpdatePlayer(alpha.Entity.ID, moved) {
		t.Fatal("expected Alpha move to update the map index")
	}
	map42Characters = registry.MapCharacters(42)
	if len(map42Characters) != 2 || map42Characters[0].Name != "Alpha" || map42Characters[1].Name != "Bravo" {
		t.Fatalf("expected Alpha and Bravo in sorted map 42 bucket after relocation, got %+v", map42Characters)
	}

	removed, ok := registry.Remove(bravo.Entity.ID)
	if !ok || removed.Entity.ID != bravo.Entity.ID {
		t.Fatalf("expected remove to return Bravo, got entity=%+v ok=%v", removed, ok)
	}
	map42Characters = registry.MapCharacters(42)
	if len(map42Characters) != 1 || map42Characters[0].Name != "Alpha" {
		t.Fatalf("expected only Alpha in map 42 bucket after removal, got %+v", map42Characters)
	}

	occupancy := registry.MapOccupancy()
	if len(occupancy) != 1 || occupancy[0].MapIndex != 42 || len(occupancy[0].Characters) != 1 || occupancy[0].Characters[0].Name != "Alpha" {
		t.Fatalf("expected deterministic map-occupancy snapshot after removal, got %+v", occupancy)
	}
}

func TestEntityRegistryRemoveClearsMapOccupancyWhenPlayerDirectoryEntryAlreadyMissing(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if alpha.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}
	if _, ok := registry.players.Remove(alpha.Entity.ID); !ok {
		t.Fatal("expected direct player-directory removal to simulate partial teardown")
	}

	removed, ok := registry.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected entity removal to still succeed after player-directory loss, got entity=%+v ok=%v", removed, ok)
	}
	if mapCharacters := registry.MapCharacters(42); len(mapCharacters) != 0 {
		t.Fatalf("expected map occupancy to be cleared after tolerant remove, got %+v", mapCharacters)
	}
	if occupancy := registry.MapOccupancy(); len(occupancy) != 0 {
		t.Fatalf("expected no map-occupancy snapshots after tolerant remove, got %+v", occupancy)
	}
}

func TestEntityRegistryRemoveClearsPlayerLookupWhenMapIndexEntryAlreadyMissing(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 42, 1700, 2800))
	if alpha.Entity.ID == 0 {
		t.Fatal("expected player registration to succeed")
	}
	if _, ok := registry.maps.Remove(alpha.Entity.ID); !ok {
		t.Fatal("expected direct map-index removal to simulate partial teardown")
	}

	removed, ok := registry.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected entity removal to still succeed after map-index loss, got entity=%+v ok=%v", removed, ok)
	}
	if _, ok := registry.Player(alpha.Entity.ID); ok {
		t.Fatal("expected player lookup to be cleared after tolerant remove")
	}
	if _, ok := registry.PlayerByVID(alpha.Entity.VID); ok {
		t.Fatal("expected player VID lookup to be cleared after tolerant remove")
	}
	if occupancy := registry.MapOccupancy(); len(occupancy) != 0 {
		t.Fatalf("expected no map-occupancy snapshots after tolerant remove, got %+v", occupancy)
	}
}

func TestEntityRegistryRegistersAndLooksUpStaticActors(t *testing.T) {
	registry := NewEntityRegistry()
	registered, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected static actor registration to succeed")
	}
	if registered.Entity.ID == 0 || registered.Entity.Kind != EntityKindStaticActor || registered.Entity.Name != "VillageGuard" {
		t.Fatalf("unexpected registered static actor: %+v", registered)
	}
	lookup, ok := registry.StaticActor(registered.Entity.ID)
	if !ok || lookup.Entity.ID != registered.Entity.ID || lookup.Position != registered.Position || lookup.RaceNum != 20300 {
		t.Fatalf("expected static actor lookup to round-trip, got actor=%+v ok=%v", lookup, ok)
	}
	actors := registry.StaticActors(42)
	if len(actors) != 1 || actors[0].Entity.ID != registered.Entity.ID {
		t.Fatalf("expected static actor map snapshot for map 42, got %+v", actors)
	}
}

func TestEntityRegistryReturnsDeterministicSortedStaticActors(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	blacksmith, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "Blacksmith"}, Position: NewPosition(42, 1900, 3000), RaceNum: 20301})
	if !ok {
		t.Fatal("expected blacksmith registration to succeed")
	}

	actors := registry.AllStaticActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 static actors in registry snapshot, got %d", len(actors))
	}
	if actors[0].Entity.ID != blacksmith.Entity.ID || actors[0].Entity.Name != "Blacksmith" {
		t.Fatalf("expected Blacksmith first in sorted static actor snapshot, got %+v", actors[0])
	}
	if actors[1].Entity.ID != guard.Entity.ID || actors[1].Entity.Name != "VillageGuard" {
		t.Fatalf("expected VillageGuard second in sorted static actor snapshot, got %+v", actors[1])
	}
}

func TestEntityRegistryRemoveStaticActorClearsLookupAndMapPresence(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	removed, ok := registry.RemoveStaticActor(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected static actor removal to return guard, got actor=%+v ok=%v", removed, ok)
	}
	if _, ok := registry.StaticActor(guard.Entity.ID); ok {
		t.Fatal("expected static actor lookup to be cleared after removal")
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected map static actor snapshot to be empty after removal, got %+v", actors)
	}
	if actors := registry.AllStaticActors(); len(actors) != 0 {
		t.Fatalf("expected global static actor snapshot to be empty after removal, got %+v", actors)
	}
}

func TestEntityRegistryRemoveStaticActorClearsMapPresenceWhenDirectoryEntryAlreadyMissing(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}
	if _, ok := registry.staticActors.Remove(guard.Entity.ID); !ok {
		t.Fatal("expected direct non-player-directory removal to simulate partial teardown")
	}

	removed, ok := registry.RemoveStaticActor(guard.Entity.ID)
	if !ok || removed.Entity.ID != guard.Entity.ID {
		t.Fatalf("expected tolerant static actor removal after directory loss, got actor=%+v ok=%v", removed, ok)
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected map static actor snapshot to be cleared after tolerant removal, got %+v", actors)
	}
	if occupancy := registry.MapOccupancy(); len(occupancy) != 0 {
		t.Fatalf("expected no map occupancy after tolerant static actor removal, got %+v", occupancy)
	}
}

func TestEntityRegistryUpdateStaticActorUpdatesLookupAndMapPresence(t *testing.T) {
	registry := NewEntityRegistry()
	guard, ok := registry.RegisterStaticActor(StaticEntity{Entity: Entity{Name: "VillageGuard"}, Position: NewPosition(42, 1700, 2800), RaceNum: 20300})
	if !ok {
		t.Fatal("expected guard registration to succeed")
	}

	updated := guard
	updated.Entity.Name = "Blacksmith"
	updated.Position = NewPosition(99, 900, 1200)
	updated.RaceNum = 20016
	result, ok := registry.UpdateStaticActor(updated)
	if !ok {
		t.Fatal("expected static actor update to succeed")
	}
	if result.Entity.ID != guard.Entity.ID || result.Entity.Name != "Blacksmith" || result.Position != NewPosition(99, 900, 1200) || result.RaceNum != 20016 {
		t.Fatalf("unexpected updated static actor result: %+v", result)
	}
	lookup, ok := registry.StaticActor(guard.Entity.ID)
	if !ok || lookup.Entity.Name != "Blacksmith" || lookup.Position != NewPosition(99, 900, 1200) || lookup.RaceNum != 20016 {
		t.Fatalf("expected static actor lookup to reflect update, got actor=%+v ok=%v", lookup, ok)
	}
	if actors := registry.StaticActors(42); len(actors) != 0 {
		t.Fatalf("expected old map static actor snapshot to be empty after update, got %+v", actors)
	}
	actors := registry.StaticActors(99)
	if len(actors) != 1 || actors[0].Entity.ID != guard.Entity.ID || actors[0].Entity.Name != "Blacksmith" {
		t.Fatalf("expected updated static actor in map 99 snapshot, got %+v", actors)
	}
}

func entityRegistryCharacter(name string, vid uint32, mapIndex uint32, x int32, y int32) loginticket.Character {
	return loginticket.Character{
		ID:       vid,
		VID:      vid,
		Name:     name,
		MapIndex: mapIndex,
		X:        x,
		Y:        y,
	}
}
