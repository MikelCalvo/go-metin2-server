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
