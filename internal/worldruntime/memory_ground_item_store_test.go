package worldruntime

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMemoryGroundItemStoreExportsWithoutFilesystem(t *testing.T) {
	dir := t.TempDir()
	store := NewMemoryGroundItemStore()

	export, err := store.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("export empty memory ground-item store: %v", err)
	}
	if export.MigrationVersion != BootstrapGroundItemStateMigrationVersion || export.MigrationName != BootstrapGroundItemStateMigrationName {
		t.Fatalf("unexpected migration boundary: %#v", export)
	}
	if len(export.GroundItems) != 0 {
		t.Fatalf("expected empty ground-item export, got %#v", export.GroundItems)
	}

	count := uint16(2)
	gold := uint32(250)
	store.Replace([]GroundItemSnapshot{
		{VID: 0x0700002d, Vnum: 1, GoldAmount: gold, OwnerName: "GroundGoldOwner", OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, PickupRange: 750, MapIndex: 42, X: 1200, Y: 2200, Z: 3},
		{VID: 0x0700002c, Vnum: 3001, Count: count, OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, PickupRange: 450, MapIndex: 1, X: 1100, Y: 2100, Z: 2},
	})

	listed := store.List()
	if len(listed) != 2 || listed[0].VID != 0x0700002c || listed[1].VID != 0x0700002d {
		t.Fatalf("unexpected listed snapshots: %#v", listed)
	}
	listed[0].OwnerName = "mutated"
	listed[0].Count = 99

	export, err = store.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("export memory ground-item store: %v", err)
	}
	want := []BootstrapGroundItemStateRow{
		{VID: 0x0700002c, Vnum: 3001, ItemCount: &count, OwnerLogin: "ground-item-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, OwnerName: "GroundItemOwner", MapIndex: 1, X: 1100, Y: 2100, Z: 2, PickupRange: 450},
		{VID: 0x0700002d, Vnum: 1, GoldAmount: &gold, OwnerLogin: "ground-gold-owner", OwnerCharacterID: 0x0103019d, OwnerVID: 0x0204019d, OwnerName: "GroundGoldOwner", MapIndex: 42, X: 1200, Y: 2200, Z: 3, PickupRange: 750},
	}
	if !reflect.DeepEqual(export.GroundItems, want) {
		t.Fatalf("unexpected memory ground-item export rows:\n got: %#v\nwant: %#v", export.GroundItems, want)
	}
	if _, err := ValidateBootstrapGroundItemStateExport(export); err != nil {
		t.Fatalf("quarantine memory ground-item export: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("memory ground-item store wrote filesystem entries: %#v", entries)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, "*")); err != nil || len(matches) != 0 {
		t.Fatalf("memory ground-item store created filesystem residue: matches=%v err=%v", matches, err)
	}
}

func TestMemoryGroundItemStoreReplaceClearsAndExportRejectsInvalidRows(t *testing.T) {
	store := NewMemoryGroundItemStore()
	store.Replace([]GroundItemSnapshot{{VID: 0x0700002c, Vnum: 3001, Count: 2, OwnerName: "GroundOwner", OwnerLogin: "ground-owner", OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c, PickupRange: 300, MapIndex: 1, X: 1100, Y: 2100}})
	store.Replace(nil)
	export, err := store.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("export cleared memory ground-item store: %v", err)
	}
	if len(export.GroundItems) != 0 {
		t.Fatalf("expected cleared store to export empty rows, got %#v", export.GroundItems)
	}

	store.Replace([]GroundItemSnapshot{{VID: 0, Vnum: 3001, Count: 1, OwnerName: "Broken", OwnerLogin: "broken", OwnerCharacterID: 1, OwnerVID: 2, PickupRange: 300, MapIndex: 1}})
	if _, err := store.ExportBootstrapGroundItemState(); err == nil {
		t.Fatal("expected invalid memory ground-item snapshot export to fail closed")
	}
}

func TestMemoryGroundItemStoreSatisfiesBootstrapGroundItemStateExporter(t *testing.T) {
	var exporter BootstrapGroundItemStateExporter = NewMemoryGroundItemStore()
	if _, err := exporter.ExportBootstrapGroundItemState(); err != nil {
		t.Fatalf("empty memory ground-item export: %v", err)
	}
}

func TestSnapshotGroundItemExporterProjectsCallbackSnapshots(t *testing.T) {
	count := uint16(1)
	exporter := SnapshotGroundItemExporter{Snapshots: func() []GroundItemSnapshot {
		return []GroundItemSnapshot{{
			VID: 0x0700002e, Vnum: 27001, Count: count, OwnerName: "AdapterOwner", OwnerLogin: "adapter-owner",
			OwnerCharacterID: 0x0103019e, OwnerVID: 0x0204019e, PickupRange: 300, MapIndex: 1, X: 100, Y: 200,
		}}
	}}
	export, err := exporter.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("snapshot adapter export: %v", err)
	}
	if len(export.GroundItems) != 1 || export.GroundItems[0].VID != 0x0700002e || export.GroundItems[0].ItemCount == nil || *export.GroundItems[0].ItemCount != 1 {
		t.Fatalf("unexpected adapter export rows: %#v", export.GroundItems)
	}

	empty := SnapshotGroundItemExporter{}
	export, err = empty.ExportBootstrapGroundItemState()
	if err != nil {
		t.Fatalf("nil adapter export: %v", err)
	}
	if len(export.GroundItems) != 0 {
		t.Fatalf("expected nil adapter to export empty rows, got %#v", export.GroundItems)
	}
}
