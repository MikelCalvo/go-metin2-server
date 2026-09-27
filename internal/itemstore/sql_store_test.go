package itemstore

import (
	"errors"
	"testing"
)

func TestSQLStoreLoadRejectsNilExecutor(t *testing.T) {
	_, err := (*SQLStore)(nil).Load()
	if !errors.Is(err, ErrItemTemplateStateImportExecutorRequired) {
		t.Fatalf("nil SQLStore.Load error = %v, want %v", err, ErrItemTemplateStateImportExecutorRequired)
	}
	_, err = NewSQLStore(nil).Load()
	if !errors.Is(err, ErrItemTemplateStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Load error = %v, want %v", err, ErrItemTemplateStateImportExecutorRequired)
	}
}

func TestSQLStoreSaveRejectsNilExecutor(t *testing.T) {
	if err := (*SQLStore)(nil).Save(Snapshot{}); !errors.Is(err, ErrItemTemplateStateImportExecutorRequired) {
		t.Fatalf("nil SQLStore.Save error = %v, want %v", err, ErrItemTemplateStateImportExecutorRequired)
	}
	if err := NewSQLStore(nil).Save(Snapshot{}); !errors.Is(err, ErrItemTemplateStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Save error = %v, want %v", err, ErrItemTemplateStateImportExecutorRequired)
	}
}

func TestSQLStoreExportRejectsNilExecutor(t *testing.T) {
	_, err := NewSQLStore(nil).ExportItemTemplateState()
	if !errors.Is(err, ErrItemTemplateStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Export error = %v, want %v", err, ErrItemTemplateStateImportExecutorRequired)
	}
}

func TestSQLStoreSaveRejectsInvalidSnapshotBeforeOpeningTransaction(t *testing.T) {
	err := NewSQLStore(failingItemTemplateStateImportExecutor{}).Save(Snapshot{Templates: []Template{{
		Vnum: 0, Name: "Broken", MaxCount: 1,
	}}})
	if err == nil || !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("SQLStore.Save(invalid) error = %v, want %v", err, ErrInvalidSnapshot)
	}
}

func TestSQLStoreSaveRejectsMyShopRejectTextBeforeOpeningTransaction(t *testing.T) {
	err := NewSQLStore(failingItemTemplateStateImportExecutor{}).Save(Snapshot{Templates: []Template{{
		Vnum: 27061, Name: "Cash MyShop Potion", Stackable: true, MaxCount: 200,
		AntiMyShop: true, MyShopRejectText: "This cash item cannot be listed in a private shop.",
	}}})
	if err == nil || !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("SQLStore.Save(myshop text) error = %v, want %v", err, ErrInvalidSnapshot)
	}
}

func TestSelectRematerializeStoreKeepsFileStoreUnlessSQLIsPassed(t *testing.T) {
	fileStore := NewMemoryStore()
	sqlStore := NewSQLStore(nil)
	if got := SelectRematerializeStore(fileStore, nil); got != fileStore {
		t.Fatalf("SelectRematerializeStore(file, nil) = %T, want the FileStore", got)
	}
	if got := SelectRematerializeStore(fileStore, sqlStore); got != sqlStore {
		t.Fatalf("SelectRematerializeStore(file, sql) = %T, want SQLStore", got)
	}
	if got := SelectRematerializeStore(nil, nil); got != nil {
		t.Fatalf("SelectRematerializeStore(nil, nil) = %#v, want nil", got)
	}
	if _, ok := SelectRematerializeStore(fileStore, nil).(*MemoryStore); !ok {
		t.Fatal("stock rematerialize must keep the supplied store when no SQLStore is passed")
	}
}
