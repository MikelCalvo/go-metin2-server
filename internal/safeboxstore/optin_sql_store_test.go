package safeboxstore

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestSelectRematerializeStoreKeepsFileStoreUntilSQLIsOptedIn(t *testing.T) {
	fileStore := NewFileStore(t.TempDir() + "/safebox.json")
	sqlStore := NewSQLStore(nil)

	if got := SelectRematerializeStore(fileStore, nil); got != fileStore {
		t.Fatalf("stock rematerialize = %T, want the FileStore", got)
	}
	if got := SelectRematerializeStore(nil, nil); got != nil {
		t.Fatalf("nil stock rematerialize = %v, want nil", got)
	}
	if got := SelectRematerializeStore(fileStore, sqlStore); got != sqlStore {
		t.Fatalf("opt-in rematerialize = %T, want the supplied SQLStore", got)
	}
	if _, ok := SelectRematerializeStore(fileStore, nil).(*FileStore); !ok {
		t.Fatal("stock gamed must keep *FileStore when no SQLStore is supplied")
	}
}

func TestStockRematerializeCheckinStaysOnFileStore(t *testing.T) {
	fileStore := NewMemoryStore()
	selected := SelectRematerializeStore(fileStore, nil)
	if selected != fileStore {
		t.Fatal("stock selector did not keep the FileStore")
	}
	if err := rematerializeSafeboxCheckin(selected, "Alpha", 7, map[uint8]inventory.ItemInstance{
		0: {ID: 1101, Vnum: 27001, Count: 1},
	}); err != nil {
		t.Fatalf("stock check-in: %v", err)
	}
	cells, err := rematerializeSafeboxOpen(selected, "Alpha", 7)
	if err != nil {
		t.Fatalf("stock reopen: %v", err)
	}
	if cells[0].Vnum != 27001 || cells[0].Count != 1 {
		t.Fatalf("stock reopened cell = %+v, want vnum 27001 count 1", cells[0])
	}
}

func rematerializeSafeboxOpen(store Store, login string, characterID uint32) (map[uint8]inventory.ItemInstance, error) {
	snapshot, err := LoadOrEmpty(store)
	if err != nil {
		return nil, err
	}
	return CharacterCells(snapshot, login, characterID), nil
}

func rematerializeSafeboxCheckin(store Store, login string, characterID uint32, cells map[uint8]inventory.ItemInstance) error {
	snapshot, err := LoadOrEmpty(store)
	if err != nil {
		return err
	}
	next, err := ReplaceCharacterCells(snapshot, login, characterID, cells)
	if err != nil {
		return err
	}
	return store.Save(next)
}
