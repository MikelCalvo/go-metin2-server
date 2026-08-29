package accountstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestMemoryStoreLoadSaveListRoundTripWithoutFilesystem(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore()

	character := rosterExportCharacter(11, "AlphaWar")
	character.Gold = 1234
	character.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 2, Slot: 8}}
	character.Equipment = []inventory.ItemInstance{{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	character.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 8}}
	character.Points[0] = 15
	character.Points[254] = 42

	if err := store.Save(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}); err != nil {
		t.Fatalf("save alpha: %v", err)
	}
	if err := store.Save(Account{Login: "Bravo", Empire: 2}); err != nil {
		t.Fatalf("save bravo: %v", err)
	}

	loaded, err := store.Load("alpha")
	if err != nil {
		t.Fatalf("load alpha fold-insensitive: %v", err)
	}
	if loaded.Login != "Alpha" || loaded.Empire != 1 || len(loaded.Characters) != 1 || loaded.Characters[0].Name != "AlphaWar" {
		t.Fatalf("unexpected loaded account: %#v", loaded)
	}
	if loaded.Characters[0].Inventory[0].Count != 2 || loaded.Characters[0].Points[0] != 15 {
		t.Fatalf("unexpected loaded item/point payload: %#v", loaded.Characters[0])
	}

	// Mutation of the returned account must not affect the store.
	loaded.Characters[0].Gold = 9999
	loaded.Characters[0].Inventory[0].Count = 99
	reloaded, err := store.Load("Alpha")
	if err != nil {
		t.Fatalf("reload alpha: %v", err)
	}
	if reloaded.Characters[0].Gold != 1234 || reloaded.Characters[0].Inventory[0].Count != 2 {
		t.Fatalf("memory store leaked caller mutation: %#v", reloaded.Characters[0])
	}

	accounts, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 2 || accounts[0].Login != "Alpha" || accounts[1].Login != "Bravo" {
		t.Fatalf("unexpected list order: %#v", accounts)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("memory store wrote filesystem entries: %#v", entries)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".account-*.json")); err != nil || len(matches) != 0 {
		t.Fatalf("memory store created crash-temp shaped files: matches=%v err=%v", matches, err)
	}
}

func TestMemoryStoreRejectsInvalidSaveAndMissingLoad(t *testing.T) {
	store := NewMemoryStore()
	if _, err := store.Load("missing"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
	if err := store.Save(Account{Login: "  spaced  "}); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for whitespace login, got %v", err)
	}
	if err := store.Save(Account{}); !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("expected ErrLoginRequired, got %v", err)
	}
}

func TestMemoryStoreExportsMatchFileStoreAndPassQuarantine(t *testing.T) {
	character := rosterExportCharacter(11, "AlphaWar")
	character.Gold = 500
	character.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 2, Slot: 8}}
	character.Equipment = []inventory.ItemInstance{{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	character.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 8}}
	character.Points[0] = 12
	character.Points[1] = -3
	character.Points[254] = 99
	seed := Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}

	fileStore := NewFileStore(t.TempDir())
	if err := fileStore.Save(seed); err != nil {
		t.Fatalf("save file store: %v", err)
	}
	memoryStore := NewMemoryStore()
	if err := memoryStore.Save(seed); err != nil {
		t.Fatalf("save memory store: %v", err)
	}

	fileRoster, err := fileStore.ExportAccountCharacterRoster()
	if err != nil {
		t.Fatalf("file roster export: %v", err)
	}
	memoryRoster, err := memoryStore.ExportAccountCharacterRoster()
	if err != nil {
		t.Fatalf("memory roster export: %v", err)
	}
	if !reflect.DeepEqual(fileRoster, memoryRoster) {
		t.Fatalf("roster export mismatch:\n file: %#v\nmemory: %#v", fileRoster, memoryRoster)
	}
	if _, err := ValidateAccountCharacterRosterExport(memoryRoster); err != nil {
		t.Fatalf("quarantine memory roster: %v", err)
	}

	fileItems, err := fileStore.ExportCharacterItemState()
	if err != nil {
		t.Fatalf("file item export: %v", err)
	}
	memoryItems, err := memoryStore.ExportCharacterItemState()
	if err != nil {
		t.Fatalf("memory item export: %v", err)
	}
	if !reflect.DeepEqual(fileItems, memoryItems) {
		t.Fatalf("item-state export mismatch:\n file: %#v\nmemory: %#v", fileItems, memoryItems)
	}
	if _, err := ValidateCharacterItemStateExport(memoryItems); err != nil {
		t.Fatalf("quarantine memory item-state: %v", err)
	}

	filePoints, err := fileStore.ExportCharacterPointState()
	if err != nil {
		t.Fatalf("file point export: %v", err)
	}
	memoryPoints, err := memoryStore.ExportCharacterPointState()
	if err != nil {
		t.Fatalf("memory point export: %v", err)
	}
	if !reflect.DeepEqual(filePoints, memoryPoints) {
		t.Fatalf("point-state export mismatch:\n file: %#v\nmemory: %#v", filePoints, memoryPoints)
	}
	if _, err := ValidateCharacterPointStateExport(memoryPoints); err != nil {
		t.Fatalf("quarantine memory point-state: %v", err)
	}
}

func TestMemoryStoreSatisfiesAccountCharacterStateExporter(t *testing.T) {
	var exporter AccountCharacterStateExporter = NewMemoryStore()
	if _, err := exporter.ExportAccountCharacterRoster(); err != nil {
		t.Fatalf("empty roster export: %v", err)
	}
	if _, err := exporter.ExportCharacterItemState(); err != nil {
		t.Fatalf("empty item export: %v", err)
	}
	if _, err := exporter.ExportCharacterPointState(); err != nil {
		t.Fatalf("empty point export: %v", err)
	}
	if _, err := exporter.ExportCharacterMyShopUnitPrices(); err != nil {
		t.Fatalf("empty myshop unit-prices export: %v", err)
	}
}
