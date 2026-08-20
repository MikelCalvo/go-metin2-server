package itemstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMemoryStoreSaveLoadWithoutFilesystem(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryStore()

	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound before save, got %v", err)
	}

	seed := Snapshot{Templates: []Template{
		{
			Vnum:         27001,
			Name:         "Small Red Potion",
			Stackable:    true,
			MaxCount:     200,
			ShopBuyPrice: 50,
			UseEffect:    &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, ConsumeCount: 1, Message: "Recovered HP"},
		},
		{
			Vnum:        11200,
			Name:        "Wooden Sword",
			Stackable:   false,
			MaxCount:    1,
			EquipSlot:   "weapon",
			Refineable:  true,
			RefineInfo:  &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75, Materials: []RefineMaterial{{Vnum: 27001, Count: 2}}},
			EquipEffect: &PointEffect{PointType: 1, PointIndex: 0, PointDelta: 4},
		},
	}}
	if err := store.Save(seed); err != nil {
		t.Fatalf("save memory item templates: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load memory item templates: %v", err)
	}
	if len(loaded.Templates) != 2 || loaded.Templates[0].Vnum != 11200 || loaded.Templates[1].Vnum != 27001 {
		t.Fatalf("unexpected loaded templates: %#v", loaded.Templates)
	}
	if loaded.Templates[0].RefineInfo == nil || loaded.Templates[0].RefineInfo.ResultVnum != 11201 {
		t.Fatalf("expected refine info to round-trip, got %#v", loaded.Templates[0].RefineInfo)
	}
	if loaded.Templates[1].UseEffect == nil || loaded.Templates[1].UseEffect.Message != "Recovered HP" {
		t.Fatalf("expected potion use effect to round-trip, got %#v", loaded.Templates[1].UseEffect)
	}

	loaded.Templates[0].Name = "mutated"
	loaded.Templates[0].RefineInfo.Cost = 9999
	loaded.Templates[0].RefineInfo.Materials[0].Count = 99
	if loaded.Templates[0].EquipEffect != nil {
		loaded.Templates[0].EquipEffect.PointDelta = 99
	}
	loaded.Templates[1].UseEffect.Message = "mutated"

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload memory item templates: %v", err)
	}
	if reloaded.Templates[0].Name != "Wooden Sword" {
		t.Fatalf("memory store leaked caller mutation into name: %#v", reloaded.Templates[0])
	}
	if reloaded.Templates[0].RefineInfo.Cost != 2500 || reloaded.Templates[0].RefineInfo.Materials[0].Count != 2 {
		t.Fatalf("memory store leaked caller mutation into refine info: %#v", reloaded.Templates[0].RefineInfo)
	}
	if reloaded.Templates[0].EquipEffect == nil || reloaded.Templates[0].EquipEffect.PointDelta != 4 {
		t.Fatalf("memory store leaked caller mutation into equip effect: %#v", reloaded.Templates[0].EquipEffect)
	}
	if reloaded.Templates[1].UseEffect == nil || reloaded.Templates[1].UseEffect.Message != "Recovered HP" {
		t.Fatalf("memory store leaked caller mutation into use effect: %#v", reloaded.Templates[1].UseEffect)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("memory store wrote filesystem entries: %#v", entries)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".item-templates-*.json")); err != nil || len(matches) != 0 {
		t.Fatalf("memory store created crash-temp shaped files: matches=%v err=%v", matches, err)
	}
}

func TestMemoryStoreRejectsInvalidSave(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(Snapshot{Templates: []Template{{Vnum: 0, Name: "Broken", MaxCount: 1}}}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("expected ErrInvalidSnapshot for zero vnum, got %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("invalid save must leave store uncommitted, got %v", err)
	}
}

func TestMemoryStoreExportsMatchFileStoreAndPassQuarantine(t *testing.T) {
	snapshot := Snapshot{Templates: []Template{
		{
			Vnum:              27001,
			Name:              "Small Red Potion",
			Stackable:         true,
			MaxCount:          200,
			ShopBuyPrice:      50,
			ShopSellPrice:     13,
			Highlight:         true,
			Unique:            true,
			AntiSell:          true,
			AntiGet:           true,
			AntiSafebox:       true,
			PickupRange:       750,
			Sockets:           SocketValues{1, 2, 3},
			Attributes:        AttributeValues{{Type: 1, Value: 10}},
			UseEffect:         &UseEffect{PointType: 7, PointIndex: 1, PointDelta: 25, ConsumeCount: 2, Message: "Recovered HP", InfoMessage: "You feel better.", SpecialEffectType: 3},
			UseRejectText:     "You cannot use this yet.",
			BuyRejectText:     "The merchant will not sell this.",
			DropRejectText:    "You cannot drop this.",
			PickupRejectText:  "You cannot pick this up.",
			SellRejectText:    "The merchant refuses this.",
			SafeboxRejectText: "You cannot store this.",
		},
		{
			Vnum:              11200,
			Name:              "Wooden Sword",
			Stackable:         false,
			MaxCount:          1,
			Refineable:        true,
			RefineInfo:        &RefineInfo{ResultVnum: 11201, Cost: 2500, Probability: 75, Materials: []RefineMaterial{{Vnum: 27001, Count: 2}, {Vnum: 27002, Count: 3}}},
			Save:              true,
			Irremovable:       true,
			AntiMale:          true,
			AppearanceVnum:    11201,
			EquipSlot:         "weapon",
			EquipEffect:       &PointEffect{PointType: 1, PointIndex: 0, PointDelta: 4},
			EquipRejectText:   "You cannot wield this.",
			UnequipRejectText: "You cannot remove this.",
		},
	}}

	fileStore := NewFileStore(filepath.Join(t.TempDir(), "item-templates.json"))
	memoryStore := NewMemoryStore()
	if err := fileStore.Save(snapshot); err != nil {
		t.Fatalf("save file item templates: %v", err)
	}
	if err := memoryStore.Save(snapshot); err != nil {
		t.Fatalf("save memory item templates: %v", err)
	}

	fileExport, err := fileStore.ExportItemTemplateState()
	if err != nil {
		t.Fatalf("file item-template-state export: %v", err)
	}
	memoryExport, err := memoryStore.ExportItemTemplateState()
	if err != nil {
		t.Fatalf("memory item-template-state export: %v", err)
	}
	if !reflect.DeepEqual(fileExport, memoryExport) {
		t.Fatalf("item-template-state export mismatch:\n file: %#v\nmemory: %#v", fileExport, memoryExport)
	}
	if _, err := ValidateItemTemplateStateExport(memoryExport); err != nil {
		t.Fatalf("quarantine memory item-template-state export: %v", err)
	}
}

func TestMemoryStoreExportTreatsEmptyStoreAsEmpty(t *testing.T) {
	store := NewMemoryStore()
	export, err := store.ExportItemTemplateState()
	if err != nil {
		t.Fatalf("export empty memory store: %v", err)
	}
	if export.MigrationVersion != ItemTemplateStateMigrationVersion || export.MigrationName != ItemTemplateStateMigrationName || len(export.Templates) != 0 {
		t.Fatalf("expected empty item-template-state export, got %#v", export)
	}
}

func TestMemoryStoreSatisfiesItemTemplateStateExporter(t *testing.T) {
	var exporter ItemTemplateStateExporter = NewMemoryStore()
	if _, err := exporter.ExportItemTemplateState(); err != nil {
		t.Fatalf("empty item-template-state export: %v", err)
	}
}
