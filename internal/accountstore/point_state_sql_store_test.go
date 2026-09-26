package accountstore

import (
	"errors"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLPointStateStoreLoadRejectsNilExecutor(t *testing.T) {
	_, err := (*SQLPointStateStore)(nil).Load("Alpha")
	if !errors.Is(err, ErrCharacterPointStateImportExecutorRequired) {
		t.Fatalf("nil SQLPointStateStore.Load error = %v, want %v", err, ErrCharacterPointStateImportExecutorRequired)
	}
	_, err = NewSQLPointStateStore(nil).Load("Alpha")
	if !errors.Is(err, ErrCharacterPointStateImportExecutorRequired) {
		t.Fatalf("NewSQLPointStateStore(nil).Load error = %v, want %v", err, ErrCharacterPointStateImportExecutorRequired)
	}
}

func TestSQLPointStateStoreLoadRejectsBlankLoginBeforeOpeningTransaction(t *testing.T) {
	_, err := NewSQLPointStateStore(failingItemStateImportExecutor{}).Load("  ")
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("Load(blank) error = %v, want %v", err, ErrLoginRequired)
	}
}

func TestSQLPointStateStoreLoadRejectsPaddedOrNULLoginBeforeOpeningTransaction(t *testing.T) {
	store := NewSQLPointStateStore(failingItemStateImportExecutor{})
	_, err := store.Load(" Alpha")
	if err == nil || !errors.Is(err, ErrInvalidAccount) || !strings.Contains(err.Error(), "leading or trailing whitespace") {
		t.Fatalf("Load(padded) error = %v, want invalid account whitespace", err)
	}
	_, err = store.Load("Alpha\x00")
	if err == nil || !errors.Is(err, ErrInvalidAccount) || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("Load(NUL) error = %v, want invalid account NUL", err)
	}
}

func TestSQLPointStateStoreSaveRejectsNilExecutor(t *testing.T) {
	account := Account{Login: "Alpha", Empire: 1}
	if err := (*SQLPointStateStore)(nil).Save(account); !errors.Is(err, ErrCharacterPointStateImportExecutorRequired) {
		t.Fatalf("nil SQLPointStateStore.Save error = %v, want %v", err, ErrCharacterPointStateImportExecutorRequired)
	}
	if err := NewSQLPointStateStore(nil).Save(account); !errors.Is(err, ErrCharacterPointStateImportExecutorRequired) {
		t.Fatalf("NewSQLPointStateStore(nil).Save error = %v, want %v", err, ErrCharacterPointStateImportExecutorRequired)
	}
}

func TestSQLPointStateStoreExportRejectsNilExecutor(t *testing.T) {
	_, err := NewSQLPointStateStore(nil).ExportCharacterPointState()
	if !errors.Is(err, ErrCharacterPointStateImportExecutorRequired) {
		t.Fatalf("NewSQLPointStateStore(nil).Export error = %v, want %v", err, ErrCharacterPointStateImportExecutorRequired)
	}
}

func TestSQLPointStateStoreSaveRejectsInvalidAccountBeforeOpeningTransaction(t *testing.T) {
	store := NewSQLPointStateStore(failingItemStateImportExecutor{})
	character := rosterExportCharacter(11, "AlphaWar")
	character.Points[1] = 40
	character.Level = 0
	err := store.Save(Account{
		Login:      "Alpha",
		Empire:     1,
		Characters: []loginticket.Character{character},
	})
	if err == nil || !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("SQLPointStateStore.Save(invalid) error = %v, want %v", err, ErrInvalidAccount)
	}
	if !strings.Contains(err.Error(), "validate point-state accounts") {
		t.Fatalf("SQLPointStateStore.Save(invalid) error = %v, want validate point-state accounts", err)
	}
}
