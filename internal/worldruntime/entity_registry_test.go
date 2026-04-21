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
