package accountstore

import (
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLRosterStoreRequiresExecutor(t *testing.T) {
	for _, store := range []*SQLRosterStore{nil, NewSQLRosterStore(nil)} {
		if _, err := store.Load("Alpha"); !errors.Is(err, ErrAccountCharacterRosterImportExecutorRequired) {
			t.Fatalf("Load nil executor: %v", err)
		}
		if _, err := store.List(); !errors.Is(err, ErrAccountCharacterRosterImportExecutorRequired) {
			t.Fatalf("List nil executor: %v", err)
		}
		if _, err := store.ExportAccountCharacterRoster(); !errors.Is(err, ErrAccountCharacterRosterImportExecutorRequired) {
			t.Fatalf("Export nil executor: %v", err)
		}
		if err := store.Save(Account{Login: "Alpha"}); !errors.Is(err, ErrAccountCharacterRosterImportExecutorRequired) {
			t.Fatalf("Save nil executor: %v", err)
		}
	}
}

func TestSQLRosterStoreRejectsInvalidSaveBeforeOpeningTransaction(t *testing.T) {
	store := NewSQLRosterStore(failingItemStateImportExecutor{})
	for _, account := range []Account{
		{Login: " Alpha"},
		{Login: "Alpha", Characters: []loginticket.Character{rosterExportShell(11, "Name"), rosterExportShell(11, "Duplicate")}},
		{Login: "Alpha", Characters: []loginticket.Character{rosterExportShell(11, "Name"), {}, {}, {}, rosterExportShell(12, "Extra")}},
		{Login: "Alpha", Characters: []loginticket.Character{func() loginticket.Character { c := rosterExportShell(11, "Name"); c.Points[1] = 3; return c }()}},
		{Login: "Alpha", CharacterChannels: map[uint32]uint8{11: 2}, Characters: []loginticket.Character{rosterExportShell(11, "Name")}},
	} {
		if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
			t.Fatalf("Save(%#v) = %v, want invalid account before transaction", account, err)
		}
	}
}
