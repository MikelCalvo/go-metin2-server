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
