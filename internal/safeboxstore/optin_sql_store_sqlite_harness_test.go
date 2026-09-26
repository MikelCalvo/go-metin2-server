//go:build sqlite_harness

package safeboxstore

import (
	"context"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessOptInSQLStoreRematerializeSurvivesReloadBesideFileStore(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion: %v", err)
	}
	seedSQLStoreRoster(t, db, []accountstore.Account{{
		Login: "Alpha", Empire: 1,
		Characters: []loginticket.Character{safeboxStateImportCharacter(7, "AlphaWar")},
	}})

	fileStore := NewMemoryStore()
	if err := fileStore.Save(Snapshot{Characters: []CharacterRow{{
		Login: "Alpha", CharacterID: 7,
		Cells: []Cell{{Cell: 0, ID: 11, Vnum: 11200, Count: 1}},
	}}}); err != nil {
		t.Fatalf("seed file store: %v", err)
	}
	sqlStore := NewSQLStore(db)
	selected := SelectRematerializeStore(fileStore, sqlStore)
	if selected != sqlStore {
		t.Fatal("opt-in selector did not return the SQLStore")
	}
	if err := rematerializeSafeboxCheckin(selected, "Alpha", 7, map[uint8]inventory.ItemInstance{
		1: {ID: 1102, Vnum: 27001, Count: 4, Slot: 1},
	}); err != nil {
		t.Fatalf("opt-in check-in: %v", err)
	}

	reloaded := SelectRematerializeStore(NewMemoryStore(), NewSQLStore(db))
	cells, err := rematerializeSafeboxOpen(reloaded, "Alpha", 7)
	if err != nil {
		t.Fatalf("opt-in reopen: %v", err)
	}
	if cells[1].Vnum != 27001 || cells[1].Count != 4 || cells[1].ID != 1102 {
		t.Fatalf("opt-in reopened cell = %+v, want vnum 27001 count 4", cells[1])
	}
	fileCells, err := rematerializeSafeboxOpen(fileStore, "Alpha", 7)
	if err != nil {
		t.Fatalf("file reopen: %v", err)
	}
	if _, wrote := fileCells[1]; wrote || fileCells[0].Vnum != 11200 {
		t.Fatalf("opt-in check-in changed the FileStore: %+v", fileCells)
	}
}
