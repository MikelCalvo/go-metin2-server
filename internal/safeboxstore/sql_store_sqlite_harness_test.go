//go:build sqlite_harness

package safeboxstore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessSQLStoreRoundTripPersistsPasswordsItemsSocketsAttributes(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}
	seedSQLStoreRoster(t, db, []accountstore.Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				safeboxStateImportCharacter(7, "AlphaWar"),
			},
		},
		{
			Login:  "Beta",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				safeboxStateImportCharacter(9, "BetaNinja"),
			},
		},
	})

	activeAttrs := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 4, Value: -5}}
	zeroAttrs := inventory.AttributeValues{}
	input := Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "secret",
		Money:       1500,
		Cells: []Cell{
			{Cell: 1, ID: 1002, Vnum: 27002, Count: 1, HasSockets: true, HasAttributes: true, Attributes: &zeroAttrs},
			{Cell: 0, ID: 1001, Vnum: 27001, Count: 2, Locked: true, HasSockets: true, Socket0: 1, Socket2: 7, HasAttributes: true, Attributes: &activeAttrs},
		},
	}, {
		Login:       "Beta",
		CharacterID: 9,
		Password:    "",
		Money:       0,
		Cells:       []Cell{},
	}}}

	store := NewSQLStore(db)
	if err := store.Save(input); err != nil {
		t.Fatalf("SQLStore.Save: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("SQLStore.Load: %v", err)
	}
	want := normalizeSnapshot(input)
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("unexpected SQLStore snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}

	export, err := store.ExportCharacterSafeboxState()
	if err != nil {
		t.Fatalf("SQLStore.ExportCharacterSafeboxState: %v", err)
	}
	wantExport, err := ExportCharacterSafeboxState(want)
	if err != nil {
		t.Fatalf("ExportCharacterSafeboxState: %v", err)
	}
	if !reflect.DeepEqual(export, wantExport) {
		t.Fatalf("unexpected SQLStore export:\n got: %#v\nwant: %#v", export, wantExport)
	}

	assertSafeboxPasswordRow(t, db, 7, "Alpha", "secret", 1500)
	assertSafeboxPasswordRow(t, db, 9, "Beta", "", 0)
	assertSafeboxItemRow(t, db, 1001, 7, "Alpha", 0, 27001, 2, true, true, 1, 0, 7, true, 1, 25, 4, -5)
	assertSafeboxItemRow(t, db, 1002, 7, "Alpha", 1, 27002, 1, false, true, 0, 0, 0, true, 0, 0, 0, 0)
}

func TestSQLiteHarnessSQLStoreLoadEmptyTablesReturnsEmptySnapshot(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}

	loaded, err := NewSQLStore(db).Load()
	if err != nil {
		t.Fatalf("SQLStore.Load empty: %v", err)
	}
	want := Snapshot{Characters: []CharacterRow{}}
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("empty SQLStore snapshot = %#v, want %#v", loaded, want)
	}
	export, err := NewSQLStore(db).ExportCharacterSafeboxState()
	if err != nil {
		t.Fatalf("SQLStore.Export empty: %v", err)
	}
	if export.MigrationVersion != CharacterSafeboxStateMigrationVersion || len(export.Passwords) != 0 || len(export.Items) != 0 {
		t.Fatalf("empty SQLStore export = %#v", export)
	}
}

func TestSQLiteHarnessSQLStoreRejectsMissingSchema(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	store := NewSQLStore(db)
	_, err := store.Load()
	if !errors.Is(err, ErrCharacterSafeboxStateImportSchemaRequired) {
		t.Fatalf("SQLStore.Load empty-ledger error = %v, want %v", err, ErrCharacterSafeboxStateImportSchemaRequired)
	}
	if err := store.Save(Snapshot{Characters: []CharacterRow{}}); !errors.Is(err, ErrCharacterSafeboxStateImportSchemaRequired) {
		t.Fatalf("SQLStore.Save empty-ledger error = %v, want %v", err, ErrCharacterSafeboxStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessSQLStoreSaveReplacesWholeSnapshot(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}
	seedSQLStoreRoster(t, db, []accountstore.Account{{
		Login:  "Alpha",
		Empire: 1,
		Characters: []loginticket.Character{
			safeboxStateImportCharacter(7, "AlphaWar"),
		},
	}, {
		Login:  "Beta",
		Empire: 2,
		Characters: []loginticket.Character{
			{},
			safeboxStateImportCharacter(9, "BetaNinja"),
		},
	}})

	store := NewSQLStore(db)
	first := Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "first",
		Money:       9,
		Cells:       []Cell{{Cell: 0, ID: 1001, Vnum: 27001, Count: 1}},
	}, {
		Login:       "Beta",
		CharacterID: 9,
		Password:    "keep",
		Money:       3,
		Cells:       []Cell{{Cell: 1, ID: 2001, Vnum: 27002, Count: 2}},
	}}}
	if err := store.Save(first); err != nil {
		t.Fatalf("first SQLStore.Save: %v", err)
	}

	second := Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "second",
		Money:       42,
		Cells:       []Cell{{Cell: 2, ID: 1003, Vnum: 27003, Count: 4, HasSockets: true, Socket1: 8}},
	}}}
	if err := store.Save(second); err != nil {
		t.Fatalf("second SQLStore.Save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("SQLStore.Load after replace: %v", err)
	}
	want := normalizeSnapshot(second)
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("replaced snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}

	var passwordRows, itemRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_passwords`).Scan(&passwordRows); err != nil {
		t.Fatalf("count passwords after replace: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_items`).Scan(&itemRows); err != nil {
		t.Fatalf("count items after replace: %v", err)
	}
	if passwordRows != 1 || itemRows != 1 {
		t.Fatalf("after whole-snapshot replace counts passwords=%d items=%d, want 1/1", passwordRows, itemRows)
	}
	assertSafeboxPasswordRow(t, db, 7, "Alpha", "second", 42)
	assertSafeboxItemRow(t, db, 1003, 7, "Alpha", 2, 27003, 4, false, true, 0, 8, 0, false, 0, 0, 0, 0)
}

func TestSQLiteHarnessSQLStoreRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}

	err := NewSQLStore(db).Save(Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "secret",
		Money:       10,
		Cells:       []Cell{{Cell: 0, ID: 1001, Vnum: 27001, Count: 1}},
	}}})
	if err == nil {
		t.Fatal("SQLStore.Save without parent character succeeded, want FK failure")
	}

	var passwordRows, itemRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_passwords`).Scan(&passwordRows); err != nil {
		t.Fatalf("count passwords after FK failure: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_items`).Scan(&itemRows); err != nil {
		t.Fatalf("count items after FK failure: %v", err)
	}
	if passwordRows != 0 || itemRows != 0 {
		t.Fatalf("rows after FK failure passwords=%d items=%d, want 0/0", passwordRows, itemRows)
	}
}

func TestSQLiteHarnessSQLStoreLoadRejectsOrphanItem(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}
	seedSQLStoreRoster(t, db, []accountstore.Account{{
		Login:  "Alpha",
		Empire: 1,
		Characters: []loginticket.Character{
			safeboxStateImportCharacter(7, "AlphaWar"),
		},
	}, {
		Login:  "Beta",
		Empire: 2,
		Characters: []loginticket.Character{
			{},
			safeboxStateImportCharacter(9, "BetaNinja"),
		},
	}})

	if _, err := db.ExecContext(ctx, `
INSERT INTO character_safebox_passwords (character_id, login, password, money)
VALUES (7, 'Alpha', 'secret', 1)`); err != nil {
		t.Fatalf("insert password parent: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO character_safebox_items (
    id, character_id, login, cell, vnum, count, locked,
    has_sockets, socket0, socket1, socket2,
    has_attributes,
    attr0_type, attr0_value, attr1_type, attr1_value, attr2_type, attr2_value,
    attr3_type, attr3_value, attr4_type, attr4_value, attr5_type, attr5_value,
    attr6_type, attr6_value
) VALUES (1001, 9, 'Beta', 0, 27001, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)`); err != nil {
		t.Fatalf("insert orphan item: %v", err)
	}

	_, err := NewSQLStore(db).Load()
	if err == nil || !errors.Is(err, ErrInvalidSnapshot) || !strings.Contains(err.Error(), "no password parent") {
		t.Fatalf("SQLStore.Load(orphan item) error = %v, want invalid snapshot with no password parent", err)
	}
}

func TestSQLiteHarnessSQLStoreLoadRejectsLoginMismatch(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}
	seedSQLStoreRoster(t, db, []accountstore.Account{{
		Login:  "Alpha",
		Empire: 1,
		Characters: []loginticket.Character{
			safeboxStateImportCharacter(7, "AlphaWar"),
		},
	}})

	if _, err := db.ExecContext(ctx, `
INSERT INTO character_safebox_passwords (character_id, login, password, money)
VALUES (7, 'Alpha', 'secret', 1)`); err != nil {
		t.Fatalf("insert password parent: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO character_safebox_items (
    id, character_id, login, cell, vnum, count, locked,
    has_sockets, socket0, socket1, socket2,
    has_attributes,
    attr0_type, attr0_value, attr1_type, attr1_value, attr2_type, attr2_value,
    attr3_type, attr3_value, attr4_type, attr4_value, attr5_type, attr5_value,
    attr6_type, attr6_value
) VALUES (1001, 7, 'Beta', 0, 27001, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)`); err != nil {
		t.Fatalf("insert mismatched item: %v", err)
	}

	_, err := NewSQLStore(db).Load()
	if err == nil || !errors.Is(err, ErrInvalidSnapshot) || !strings.Contains(err.Error(), "does not match password login") {
		t.Fatalf("SQLStore.Load(login mismatch) error = %v, want invalid snapshot login mismatch", err)
	}
}

func seedSQLStoreRoster(t *testing.T, executor dbmigrations.SQLMigrationExecutor, accounts []accountstore.Account) {
	t.Helper()
	rosterExport, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(context.Background(), executor, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
}
