package safeboxstore

import (
	"errors"
	"strings"
	"testing"
)

func TestSQLStoreLoadRejectsNilExecutor(t *testing.T) {
	_, err := (*SQLStore)(nil).Load()
	if !errors.Is(err, ErrCharacterSafeboxStateImportExecutorRequired) {
		t.Fatalf("nil SQLStore.Load error = %v, want %v", err, ErrCharacterSafeboxStateImportExecutorRequired)
	}
	_, err = NewSQLStore(nil).Load()
	if !errors.Is(err, ErrCharacterSafeboxStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Load error = %v, want %v", err, ErrCharacterSafeboxStateImportExecutorRequired)
	}
}

func TestSQLStoreSaveRejectsNilExecutor(t *testing.T) {
	snapshot := Snapshot{Characters: []CharacterRow{}}
	if err := (*SQLStore)(nil).Save(snapshot); !errors.Is(err, ErrCharacterSafeboxStateImportExecutorRequired) {
		t.Fatalf("nil SQLStore.Save error = %v, want %v", err, ErrCharacterSafeboxStateImportExecutorRequired)
	}
	if err := NewSQLStore(nil).Save(snapshot); !errors.Is(err, ErrCharacterSafeboxStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Save error = %v, want %v", err, ErrCharacterSafeboxStateImportExecutorRequired)
	}
}

func TestSQLStoreExportRejectsNilExecutor(t *testing.T) {
	_, err := NewSQLStore(nil).ExportCharacterSafeboxState()
	if !errors.Is(err, ErrCharacterSafeboxStateImportExecutorRequired) {
		t.Fatalf("NewSQLStore(nil).Export error = %v, want %v", err, ErrCharacterSafeboxStateImportExecutorRequired)
	}
}

func TestSQLStoreSaveRejectsInvalidSnapshotBeforeOpeningTransaction(t *testing.T) {
	store := NewSQLStore(failingSafeboxStateImportExecutor{})
	err := store.Save(Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Cells:       []Cell{{Cell: 15, ID: 1, Vnum: 27001, Count: 1}},
	}}})
	if err == nil || !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("SQLStore.Save(invalid) error = %v, want %v", err, ErrInvalidSnapshot)
	}
	if !strings.Contains(err.Error(), "validate safebox snapshot") {
		t.Fatalf("SQLStore.Save(invalid) error = %v, want validate safebox snapshot", err)
	}
}
