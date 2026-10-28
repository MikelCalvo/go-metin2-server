//go:build sqlite_harness

package accountstore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessSQLRosterStoreRoundTripScopedSaveAndEmptyAccount(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, 2); err != nil {
		t.Fatal(err)
	}

	a := rosterExportShell(11, "AlphaWar")
	a.Level = 9
	a.X, a.Y, a.Z = 15, -20, 30
	a.Gold = 900
	alpha := Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{{}, a}}
	bravo := Account{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{rosterExportShell(22, "BravoWar")}}
	store := NewSQLRosterStore(db)
	for _, account := range []Account{alpha, bravo, {Login: "Empty", Empire: 3}} {
		if err := store.Save(account); err != nil {
			t.Fatalf("Save %q: %v", account.Login, err)
		}
	}
	loaded, err := store.Load("aLpHa")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Login != "Alpha" || loaded.Empire != 1 || len(loaded.Characters) != 4 || !loaded.Characters[0].IsEmptySlot() || loaded.Characters[1].ID != 11 || loaded.Characters[1].Gold != 900 || loaded.Characters[1].X != 15 || loaded.Characters[1].Y != -20 || loaded.Characters[1].Z != 30 || loaded.Characters[1].Level != 9 {
		t.Fatalf("loaded roster: %#v", loaded)
	}
	wantInitial, err := ExportAccountCharacterRoster([]Account{alpha, bravo, {Login: "Empty", Empire: 3}})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := store.ExportAccountCharacterRoster()
	if err != nil || !reflect.DeepEqual(initial.Accounts, wantInitial.Accounts) || !reflect.DeepEqual(initial.Characters, wantInitial.Characters) {
		t.Fatalf("roster SQL roundtrip: got %#v; want %#v; err %v", initial, wantInitial, err)
	}
	if empty, err := store.Load("Empty"); err != nil || empty.Login != "Empty" || len(empty.Characters) != 4 {
		t.Fatalf("empty account = %#v, %v", empty, err)
	}
	if _, err := store.Load("Missing"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("missing = %v", err)
	}

	// A second Save replaces exactly this account's roster; it is not an
	// insert-only import or a global truncate, and empty slots remove rows.
	alpha.Characters = []loginticket.Character{rosterExportShell(12, "AlphaSura")}
	alpha.Empire = 3
	if err := store.Save(alpha); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(Account{Login: "Empty", Empire: 1}); err != nil {
		t.Fatal(err)
	}
	listed, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 3 || listed[0].Login != "Alpha" || listed[0].Empire != 3 || listed[0].Characters[0].ID != 12 || !listed[0].Characters[1].IsEmptySlot() || listed[1].Login != "Bravo" || listed[1].Characters[0].ID != 22 || listed[2].Login != "Empty" || listed[2].Empire != 1 {
		t.Fatalf("listed roster: %#v", listed)
	}
	exported, err := store.ExportAccountCharacterRoster()
	if err != nil {
		t.Fatal(err)
	}
	if exported.MigrationVersion != 2 || exported.MigrationName != "account_character_roster" || len(exported.Accounts) != 3 || len(exported.Characters) != 2 {
		t.Fatalf("export: %#v", exported)
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM characters WHERE id = 11`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("old slot rows = %d, %v", rows, err)
	}
}

func TestSQLiteHarnessSQLRosterStoreLoadsImportedIDsAndPreservesThemOnSave(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, 2); err != nil {
		t.Fatal(err)
	}
	// Imported IDs need not be the deterministic FileStore projection ID.
	export := AccountCharacterRosterExport{MigrationVersion: 2, MigrationName: "account_character_roster", Accounts: []AccountCharacterRosterAccountRow{{ID: 123, Login: "Alpha", LoginNormalized: "alpha", Empire: 1}}, Characters: []AccountCharacterRosterCharacterRow{}}
	if _, err := ImportAccountCharacterRoster(context.Background(), db, export); err != nil {
		t.Fatal(err)
	}
	store := NewSQLRosterStore(db)
	loaded, err := store.Load("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	loaded.Characters[3] = rosterExportShell(42, "Name")
	if err := store.Save(loaded); err != nil {
		t.Fatal(err)
	}
	got, err := store.ExportAccountCharacterRoster()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Accounts, export.Accounts) || len(got.Characters) != 1 || got.Characters[0].AccountID != 123 {
		t.Fatalf("imported ID changed: %#v", got)
	}
}

func TestSQLiteHarnessSQLRosterStoreFailureDoesNotDestroyRoster(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, 3); err != nil {
		t.Fatal(err)
	}
	store := NewSQLRosterStore(db)
	alpha := Account{Login: "Alpha", Characters: []loginticket.Character{rosterExportShell(11, "AlphaWar")}}
	bravo := Account{Login: "Bravo", Characters: []loginticket.Character{rosterExportShell(22, "BravoWar")}}
	if err := store.Save(alpha); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(bravo); err != nil {
		t.Fatal(err)
	}
	before, err := store.ExportAccountCharacterRoster()
	if err != nil {
		t.Fatal(err)
	}

	// A conflicting name on another account must roll back the DELETE.
	alpha.Characters[0].Name = "BravoWar"
	if err := store.Save(alpha); err == nil {
		t.Fatal("unique-name conflict accepted")
	}
	// A dependent child blocks roster replacement: no implicit cascade.
	alpha.Characters[0].Name = "AlphaWar"
	if _, err := db.Exec(`INSERT INTO character_quickslots (character_id, position, type, slot) VALUES (11, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(alpha); err == nil {
		t.Fatal("dependent item-state child was deleted")
	}
	after, err := store.ExportAccountCharacterRoster()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("rollback changed roster:\n before=%#v\n after=%#v", before, after)
	}
}

func TestSQLiteHarnessSQLRosterStoreRejectsMissingSchema(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()
	store := NewSQLRosterStore(db)
	if _, err := store.Load("Alpha"); !errors.Is(err, ErrAccountCharacterRosterImportSchemaRequired) {
		t.Fatalf("Load schema error = %v", err)
	}
	if err := store.Save(Account{Login: "Alpha"}); !errors.Is(err, ErrAccountCharacterRosterImportSchemaRequired) {
		t.Fatalf("Save schema error = %v", err)
	}
}

func TestSQLiteHarnessSQLRosterStoreRejectsMalformedSQLRows(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, 2); err != nil {
		t.Fatal(err)
	}
	// The schema permits a value outside the client empire carrier. The read
	// must fail rather than narrowing it or synthesizing an empty account.
	if _, err := db.Exec(`INSERT INTO accounts (id, login, login_normalized, empire) VALUES (1, 'Alpha', 'alpha', 256)`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSQLRosterStore(db).List(); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("out-of-range empire: %v", err)
	}
}
