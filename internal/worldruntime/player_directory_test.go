package worldruntime

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

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

func TestPlayerDirectoryRegisterReclaimsStaleVIDAndNameIndexes(t *testing.T) {
	registry := NewEntityRegistry()
	alpha := registry.RegisterPlayer(entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100))
	bravo := newPlayerEntity(alpha.Entity.ID+1, entityRegistryCharacter(alpha.Entity.Name, alpha.Entity.VID, 42, 1300, 2300))

	directory := NewPlayerDirectory()
	directory.entityIDByVID[alpha.Entity.VID] = alpha.Entity.ID
	directory.entityIDByName[alpha.Entity.Name] = alpha.Entity.ID

	if !directory.Register(bravo) {
		t.Fatal("expected stale secondary indexes without entity ownership to be reclaimed during register")
	}
	byVID, ok := directory.ByVID(alpha.Entity.VID)
	if !ok || byVID.Entity.ID != bravo.Entity.ID {
		t.Fatalf("expected VID lookup to resolve registered replacement, got entity=%+v ok=%v", byVID, ok)
	}
	byName, ok := directory.ByName(alpha.Entity.Name)
	if !ok || byName.Entity.ID != bravo.Entity.ID {
		t.Fatalf("expected exact-name lookup to resolve registered replacement, got entity=%+v ok=%v", byName, ok)
	}
}

func TestPlayerDirectoryLookupPrunesStaleVIDAndNameIndexes(t *testing.T) {
	directory := NewPlayerDirectory()
	directory.entityIDByVID[0x02040101] = 99
	directory.entityIDByName["Ghost"] = 99

	if player, ok := directory.ByVID(0x02040101); ok {
		t.Fatalf("expected stale VID lookup to fail, got %+v", player)
	}
	if _, exists := directory.entityIDByVID[0x02040101]; exists {
		t.Fatal("expected stale VID index to be pruned after lookup")
	}
	if player, ok := directory.ByName("Ghost"); ok {
		t.Fatalf("expected stale exact-name lookup to fail, got %+v", player)
	}
	if _, exists := directory.entityIDByName["Ghost"]; exists {
		t.Fatal("expected stale exact-name index to be pruned after lookup")
	}
}

func TestPlayerDirectoryRemovePrunesStaleVIDAndNameIndexes(t *testing.T) {
	directory := NewPlayerDirectory()
	directory.entityIDByVID[0x02040101] = 99
	directory.entityIDByName["Ghost"] = 99

	if player, ok := directory.Remove(99); ok {
		t.Fatalf("expected remove to report missing player entry, got %+v", player)
	}
	if _, exists := directory.entityIDByVID[0x02040101]; exists {
		t.Fatal("expected stale VID index to be pruned during remove")
	}
	if _, exists := directory.entityIDByName["Ghost"]; exists {
		t.Fatal("expected stale exact-name index to be pruned during remove")
	}
}

func TestPlayerDirectoryLookupsDeepCloneItemState(t *testing.T) {
	registry := NewEntityRegistry()
	alphaCharacter := entityRegistryCharacter("Alpha", 0x02040101, 1, 1100, 2100)
	alphaCharacter.Inventory = append(alphaCharacter.Inventory, inventory.ItemInstance{ID: 101, Vnum: 27001, Count: 1})
	alphaCharacter.Equipment = append(alphaCharacter.Equipment, inventory.ItemInstance{ID: 201, Vnum: 11299, Count: 1})
	alphaCharacter.Quickslots = append(alphaCharacter.Quickslots, loginticket.Quickslot{Type: 1, Position: 2})
	alpha := registry.RegisterPlayer(alphaCharacter)

	directory := NewPlayerDirectory()
	if !directory.Register(alpha) {
		t.Fatal("expected player directory registration to succeed")
	}

	byEntityID, ok := directory.ByEntityID(alpha.Entity.ID)
	if !ok {
		t.Fatal("expected entity-id lookup to succeed")
	}
	byEntityID.Character.Inventory[0].Vnum = 11111
	byEntityID.Character.Equipment[0].Vnum = 22222
	byEntityID.Character.Quickslots[0].Position = 9

	byVID, ok := directory.ByVID(alpha.Entity.VID)
	if !ok {
		t.Fatal("expected VID lookup to succeed")
	}
	byVID.Character.Inventory[0].Count = 7

	byName, ok := directory.ByName(alpha.Entity.Name)
	if !ok {
		t.Fatal("expected exact-name lookup to succeed")
	}
	byName.Character.Equipment[0].Count = 8

	characters := directory.PlayerCharacters()
	characters[0].Inventory[0].Vnum = 33333

	stored, ok := directory.ByEntityID(alpha.Entity.ID)
	if !ok {
		t.Fatal("expected stored player to remain present")
	}
	if stored.Character.Inventory[0].Vnum != 27001 || stored.Character.Inventory[0].Count != 1 {
		t.Fatalf("expected stored inventory to stay cloned, got %+v", stored.Character.Inventory)
	}
	if stored.Character.Equipment[0].Vnum != 11299 || stored.Character.Equipment[0].Count != 1 {
		t.Fatalf("expected stored equipment to stay cloned, got %+v", stored.Character.Equipment)
	}
	if stored.Character.Quickslots[0].Position != 2 {
		t.Fatalf("expected stored quickslots to stay cloned, got %+v", stored.Character.Quickslots)
	}
}
