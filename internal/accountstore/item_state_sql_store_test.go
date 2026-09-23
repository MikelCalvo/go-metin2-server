package accountstore

import (
	"errors"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLItemStateStoreLoadRejectsNilExecutor(t *testing.T) {
	_, err := (*SQLItemStateStore)(nil).Load("Alpha")
	if !errors.Is(err, ErrCharacterItemStateImportExecutorRequired) {
		t.Fatalf("nil SQLItemStateStore.Load error = %v, want %v", err, ErrCharacterItemStateImportExecutorRequired)
	}
	_, err = NewSQLItemStateStore(nil).Load("Alpha")
	if !errors.Is(err, ErrCharacterItemStateImportExecutorRequired) {
		t.Fatalf("NewSQLItemStateStore(nil).Load error = %v, want %v", err, ErrCharacterItemStateImportExecutorRequired)
	}
}

func TestSQLItemStateStoreLoadRejectsBlankLoginBeforeOpeningTransaction(t *testing.T) {
	_, err := NewSQLItemStateStore(failingItemStateImportExecutor{}).Load("  ")
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Load(blank) error = %v, want %v", err, ErrLoginRequired)
	}
}

func TestSQLItemStateStoreSaveRejectsNilExecutor(t *testing.T) {
	account := Account{Login: "Alpha", Empire: 1}
	if err := (*SQLItemStateStore)(nil).Save(account); !errors.Is(err, ErrCharacterItemStateImportExecutorRequired) {
		t.Fatalf("nil SQLItemStateStore.Save error = %v, want %v", err, ErrCharacterItemStateImportExecutorRequired)
	}
	if err := NewSQLItemStateStore(nil).Save(account); !errors.Is(err, ErrCharacterItemStateImportExecutorRequired) {
		t.Fatalf("NewSQLItemStateStore(nil).Save error = %v, want %v", err, ErrCharacterItemStateImportExecutorRequired)
	}
}

func TestSQLItemStateStoreExportRejectsNilExecutor(t *testing.T) {
	_, err := NewSQLItemStateStore(nil).ExportCharacterItemState()
	if !errors.Is(err, ErrCharacterItemStateImportExecutorRequired) {
		t.Fatalf("NewSQLItemStateStore(nil).Export error = %v, want %v", err, ErrCharacterItemStateImportExecutorRequired)
	}
}

func TestSQLItemStateStoreSaveRejectsInvalidAccountBeforeOpeningTransaction(t *testing.T) {
	store := NewSQLItemStateStore(failingItemStateImportExecutor{})
	character := rosterExportCharacter(11, "AlphaWar")
	character.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 0, Slot: 5}}
	err := store.Save(Account{
		Login:      "Alpha",
		Empire:     1,
		Characters: []loginticket.Character{character},
	})
	if err == nil || !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("SQLItemStateStore.Save(invalid) error = %v, want %v", err, ErrInvalidAccount)
	}
	if !strings.Contains(err.Error(), "validate item-state accounts") {
		t.Fatalf("SQLItemStateStore.Save(invalid) error = %v, want validate item-state accounts", err)
	}
}
