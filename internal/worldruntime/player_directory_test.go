package worldruntime

import "testing"

func TestPlayerDirectoryLooksUpPlayersByEntityID(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 42, 1300, 2300))

	directory := NewPlayerDirectory()
	if !directory.Register(alpha) || !directory.Register(bravo) {
		t.Fatal("expected player directory registration to succeed")
	}

	lookup, ok := directory.ByEntityID(alpha.Entity.ID)
	if !ok || lookup.Entity.ID != alpha.Entity.ID || lookup.Entity.Name != "Alpha" {
		t.Fatalf("expected entity-id lookup to return Alpha, got entity=%+v ok=%v", lookup, ok)
	}
}

func TestPlayerDirectoryLooksUpPlayersByVIDAndExactName(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	bravo := registry.RegisterPlayer(entityRegistryCharacter("Bravo", 0x02040102, 42, 1300, 2300))

	directory := NewPlayerDirectory()
	if !directory.Register(alpha) || !directory.Register(bravo) {
		t.Fatal("expected player directory registration to succeed")
	}

	byVID, ok := directory.ByVID(bravo.Entity.VID)
	if !ok || byVID.Entity.ID != bravo.Entity.ID || byVID.Entity.Name != "Bravo" {
		t.Fatalf("expected VID lookup to return Bravo, got entity=%+v ok=%v", byVID, ok)
	}

	byName, ok := directory.ByName(alpha.Entity.Name)
	if !ok || byName.Entity.ID != alpha.Entity.ID || byName.Entity.VID != alpha.Entity.VID {
		t.Fatalf("expected exact-name lookup to return Alpha, got entity=%+v ok=%v", byName, ok)
	}
}

func TestPlayerDirectoryUpdatesVIDAndExactNameIndexes(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))

	directory := NewPlayerDirectory()
	if !directory.Register(alpha) {
		t.Fatal("expected player directory registration to succeed")
	}

	updated := alpha
	updated.Entity.VID = 0x02040111
	updated.Entity.Name = "AlphaPrime"
	updated.Character.VID = 0x02040111
	updated.Character.Name = "AlphaPrime"
	updated.Character.MapIndex = 42
	if !directory.Update(updated) {
		t.Fatal("expected player directory update to succeed")
	}

	if _, ok := directory.ByVID(alpha.Entity.VID); ok {
		t.Fatal("expected old VID index to be cleared after update")
	}
	if _, ok := directory.ByName(alpha.Entity.Name); ok {
		t.Fatal("expected old exact-name index to be cleared after update")
	}
	byVID, ok := directory.ByVID(updated.Entity.VID)
	if !ok || byVID.Entity.Name != "AlphaPrime" || byVID.Character.MapIndex != 42 {
		t.Fatalf("expected updated VID lookup to return AlphaPrime, got entity=%+v ok=%v", byVID, ok)
	}
	byName, ok := directory.ByName(updated.Entity.Name)
	if !ok || byName.Entity.ID != updated.Entity.ID || byName.Entity.VID != updated.Entity.VID {
		t.Fatalf("expected updated exact-name lookup to return AlphaPrime, got entity=%+v ok=%v", byName, ok)
	}
}

func TestPlayerDirectoryRemoveClearsEntityIDVIDAndNameIndexes(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))

	directory := NewPlayerDirectory()
	if !directory.Register(alpha) {
		t.Fatal("expected player directory registration to succeed")
	}

	removed, ok := directory.Remove(alpha.Entity.ID)
	if !ok || removed.Entity.ID != alpha.Entity.ID {
		t.Fatalf("expected remove to return Alpha, got entity=%+v ok=%v", removed, ok)
	}
	if _, ok := directory.ByEntityID(alpha.Entity.ID); ok {
		t.Fatal("expected entity-id lookup to be cleared after removal")
	}
	if _, ok := directory.ByVID(alpha.Entity.VID); ok {
		t.Fatal("expected VID lookup to be cleared after removal")
	}
	if _, ok := directory.ByName(alpha.Entity.Name); ok {
		t.Fatal("expected exact-name lookup to be cleared after removal")
	}
}
