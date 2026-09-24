package worldruntime

import (
	"errors"
	"testing"
)

func TestSQLGroundItemStoreLoadRejectsNilExecutor(t *testing.T) {
	_, err := (*SQLGroundItemStore)(nil).LoadGroundItems()
	if !errors.Is(err, ErrBootstrapGroundItemStateImportExecutorRequired) {
		t.Fatalf("nil SQLGroundItemStore.LoadGroundItems error = %v, want %v", err, ErrBootstrapGroundItemStateImportExecutorRequired)
	}
	_, err = NewSQLGroundItemStore(nil).LoadGroundItems()
	if !errors.Is(err, ErrBootstrapGroundItemStateImportExecutorRequired) {
		t.Fatalf("NewSQLGroundItemStore(nil).LoadGroundItems error = %v, want %v", err, ErrBootstrapGroundItemStateImportExecutorRequired)
	}
}

func TestSQLGroundItemStoreSaveRejectsNilExecutor(t *testing.T) {
	if err := (*SQLGroundItemStore)(nil).SaveGroundItems(nil); !errors.Is(err, ErrBootstrapGroundItemStateImportExecutorRequired) {
		t.Fatalf("nil SQLGroundItemStore.SaveGroundItems error = %v, want %v", err, ErrBootstrapGroundItemStateImportExecutorRequired)
	}
	if err := NewSQLGroundItemStore(nil).SaveGroundItems(nil); !errors.Is(err, ErrBootstrapGroundItemStateImportExecutorRequired) {
		t.Fatalf("NewSQLGroundItemStore(nil).SaveGroundItems error = %v, want %v", err, ErrBootstrapGroundItemStateImportExecutorRequired)
	}
}

func TestSQLGroundItemStoreExportRejectsNilExecutor(t *testing.T) {
	_, err := NewSQLGroundItemStore(nil).ExportBootstrapGroundItemState()
	if !errors.Is(err, ErrBootstrapGroundItemStateImportExecutorRequired) {
		t.Fatalf("NewSQLGroundItemStore(nil).Export error = %v, want %v", err, ErrBootstrapGroundItemStateImportExecutorRequired)
	}
}

func TestSQLGroundItemStoreSaveRejectsInvalidSnapshotBeforeOpeningTransaction(t *testing.T) {
	err := NewSQLGroundItemStore(failingGroundItemStateImportExecutor{}).SaveGroundItems([]GroundItemSnapshot{{
		VID: 0, Vnum: 3001, Count: 1,
		OwnerLogin: "ground-item-owner", OwnerName: "GroundItemOwner",
		OwnerCharacterID: 1, OwnerVID: 2, MapIndex: 1, PickupRange: 300,
	}})
	if err == nil || !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
		t.Fatalf("SQLGroundItemStore.SaveGroundItems(invalid) error = %v, want %v", err, ErrInvalidBootstrapGroundItemStateExport)
	}
}
