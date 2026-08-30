//go:build sqlite_harness

package safeboxstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessSafeboxStateImportInsertsPasswordsAndItems(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}

	accounts := []accountstore.Account{
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
	}

	rosterExport, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	snapshot := Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "secret",
		Money:       1500,
		Cells: []Cell{
			{Cell: 1, ID: 1002, Vnum: 27002, Count: 1, HasSockets: true, HasAttributes: true},
			{Cell: 0, ID: 1001, Vnum: 27001, Count: 2, Locked: true, HasSockets: true, Socket0: 1, Socket2: 7, HasAttributes: true, Attributes: &inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 4, Value: -5}}},
		},
	}, {
		Login:       "Beta",
		CharacterID: 9,
		Password:    "",
		Money:       0,
		Cells:       []Cell{},
	}}}

	safeboxExport, err := ExportCharacterSafeboxState(snapshot)
	if err != nil {
		t.Fatalf("ExportCharacterSafeboxState: %v", err)
	}

	result, err := ImportCharacterSafeboxState(ctx, db, safeboxExport)
	if err != nil {
		t.Fatalf("ImportCharacterSafeboxState: %v", err)
	}
	if result.MigrationVersion != CharacterSafeboxStateMigrationVersion || result.MigrationName != CharacterSafeboxStateMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.CharacterCount != 2 || result.PasswordCount != 2 || result.ItemCount != 2 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.CharacterIDs) != 2 || result.CharacterIDs[0] != 7 || result.CharacterIDs[1] != 9 {
		t.Fatalf("unexpected character ids: %+v", result.CharacterIDs)
	}

	var passwordRows, itemRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_passwords`).Scan(&passwordRows); err != nil {
		t.Fatalf("count safebox passwords: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_items`).Scan(&itemRows); err != nil {
		t.Fatalf("count safebox items: %v", err)
	}
	if passwordRows != 2 || itemRows != 2 {
		t.Fatalf("row counts passwords=%d items=%d, want 2/2", passwordRows, itemRows)
	}

	assertSafeboxPasswordRow(t, db, 7, "Alpha", "secret", 1500)
	assertSafeboxPasswordRow(t, db, 9, "Beta", "", 0)
	assertSafeboxItemRow(t, db, 1001, 7, "Alpha", 0, 27001, 2, true, true, 1, 0, 7, true, 1, 25, 4, -5)
	assertSafeboxItemRow(t, db, 1002, 7, "Alpha", 1, 27002, 1, false, true, 0, 0, 0, true, 0, 0, 0, 0)
}

func TestSQLiteHarnessSafeboxStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}

	accounts := []accountstore.Account{{
		Login:  "Alpha",
		Empire: 1,
		Characters: []loginticket.Character{
			safeboxStateImportCharacter(7, "AlphaWar"),
		},
	}}
	rosterExport, err := accountstore.ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := accountstore.ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	safeboxExport, err := ExportCharacterSafeboxState(Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "pass01",
		Money:       42,
		Cells:       []Cell{{Cell: 0, ID: 1001, Vnum: 27001, Count: 1}},
	}}})
	if err != nil {
		t.Fatalf("ExportCharacterSafeboxState: %v", err)
	}
	if _, err := ImportCharacterSafeboxState(ctx, db, safeboxExport); err != nil {
		t.Fatalf("first ImportCharacterSafeboxState: %v", err)
	}

	_, err = ImportCharacterSafeboxState(ctx, db, safeboxExport)
	if err == nil {
		t.Fatal("second ImportCharacterSafeboxState succeeded, want unique conflict")
	}

	var passwordRows, itemRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_passwords`).Scan(&passwordRows); err != nil {
		t.Fatalf("count passwords after failed reimport: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_items`).Scan(&itemRows); err != nil {
		t.Fatalf("count items after failed reimport: %v", err)
	}
	if passwordRows != 1 || itemRows != 1 {
		t.Fatalf("rows after failed reimport passwords=%d items=%d, want 1/1 (no partial second import)", passwordRows, itemRows)
	}
}

func TestSQLiteHarnessSafeboxStateImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}

	_, err := ImportCharacterSafeboxState(ctx, db, export)
	if !errors.Is(err, ErrCharacterSafeboxStateImportSchemaRequired) {
		t.Fatalf("ImportCharacterSafeboxState on empty DB error = %v, want %v", err, ErrCharacterSafeboxStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessSafeboxStateImportRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}

	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords: []CharacterSafeboxPasswordRow{
			{CharacterID: 7, Login: "Alpha", Password: "secret", Money: 10},
		},
		Items: []CharacterSafeboxItemRow{},
	}

	_, err := ImportCharacterSafeboxState(ctx, db, export)
	if err == nil {
		t.Fatal("ImportCharacterSafeboxState without parent character succeeded, want FK failure")
	}

	var passwordRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_safebox_passwords`).Scan(&passwordRows); err != nil {
		t.Fatalf("count passwords after FK failure: %v", err)
	}
	if passwordRows != 0 {
		t.Fatalf("password rows after FK failure = %d, want 0", passwordRows)
	}
}

func TestSQLiteHarnessSafeboxStateImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceAttributesMigrationVersion, err)
	}

	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}
	result, err := ImportCharacterSafeboxState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportCharacterSafeboxState(empty): %v", err)
	}
	if result.CharacterCount != 0 || result.PasswordCount != 0 || result.ItemCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
	if len(result.CharacterIDs) != 0 {
		t.Fatalf("empty import character ids = %+v, want empty", result.CharacterIDs)
	}
}

func TestSQLiteHarnessSafeboxStateImportRejectsTipFifteenOnlyLedger(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxStateMigrationVersion, err)
	}

	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}
	_, err := ImportCharacterSafeboxState(ctx, db, export)
	if !errors.Is(err, ErrCharacterSafeboxStateImportSchemaRequired) {
		t.Fatalf("ImportCharacterSafeboxState tip-15-only error = %v, want %v", err, ErrCharacterSafeboxStateImportSchemaRequired)
	}
	if err == nil || !strings.Contains(err.Error(), "25") || !strings.Contains(err.Error(), CharacterSafeboxItemInstanceSocketsMigrationName) {
		t.Fatalf("expected tip-15-only error to name additive 25, got %v", err)
	}
}

func TestSQLiteHarnessSafeboxStateImportRejectsTipTwentyFiveOnlyLedger(t *testing.T) {
	db := openSQLiteSafeboxStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterSafeboxItemInstanceSocketsMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterSafeboxItemInstanceSocketsMigrationVersion, err)
	}

	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}
	_, err := ImportCharacterSafeboxState(ctx, db, export)
	if !errors.Is(err, ErrCharacterSafeboxStateImportSchemaRequired) {
		t.Fatalf("ImportCharacterSafeboxState tip-25-only error = %v, want %v", err, ErrCharacterSafeboxStateImportSchemaRequired)
	}
	if err == nil || !strings.Contains(err.Error(), "28") || !strings.Contains(err.Error(), CharacterSafeboxItemInstanceAttributesMigrationName) {
		t.Fatalf("expected tip-25-only error to name additive 28, got %v", err)
	}
}

func assertSafeboxPasswordRow(t *testing.T, db *sql.DB, characterID uint32, login, password string, money int64) {
	t.Helper()

	var (
		gotCharacterID int64
		gotLogin       string
		gotPassword    string
		gotMoney       int64
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT character_id, login, password, money
FROM character_safebox_passwords WHERE character_id = ?`,
		characterID).Scan(&gotCharacterID, &gotLogin, &gotPassword, &gotMoney); err != nil {
		t.Fatalf("select safebox password character %d: %v", characterID, err)
	}
	if gotCharacterID != int64(characterID) || gotLogin != login || gotPassword != password || gotMoney != money {
		t.Fatalf("password row mismatch for character %d: character=%d login=%q password=%q money=%d want login=%q password=%q money=%d",
			characterID, gotCharacterID, gotLogin, gotPassword, gotMoney, login, password, money)
	}
}

func assertSafeboxItemRow(t *testing.T, db *sql.DB, id uint64, characterID uint32, login string, cell uint8, vnum uint32, count uint16, locked bool, hasSockets bool, socket0, socket1, socket2 int32, hasAttributes bool, attr0Type uint8, attr0Value int16, attr1Type uint8, attr1Value int16) {
	t.Helper()

	var (
		gotID            int64
		gotCharacterID   int64
		gotLogin         string
		gotCell          int
		gotVnum          int64
		gotCount         int
		gotLocked        int
		gotHasSockets    int
		gotSocket0       int
		gotSocket1       int
		gotSocket2       int
		gotHasAttributes int
		gotAttr0Type     int
		gotAttr0Value    int
		gotAttr1Type     int
		gotAttr1Value    int
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT id, character_id, login, cell, vnum, count, locked, has_sockets, socket0, socket1, socket2,
       has_attributes, attr0_type, attr0_value, attr1_type, attr1_value
FROM character_safebox_items WHERE id = ?`,
		id).Scan(&gotID, &gotCharacterID, &gotLogin, &gotCell, &gotVnum, &gotCount, &gotLocked, &gotHasSockets, &gotSocket0, &gotSocket1, &gotSocket2, &gotHasAttributes, &gotAttr0Type, &gotAttr0Value, &gotAttr1Type, &gotAttr1Value); err != nil {
		t.Fatalf("select safebox item id %d: %v", id, err)
	}
	wantLocked := 0
	if locked {
		wantLocked = 1
	}
	wantHasSockets := 0
	if hasSockets {
		wantHasSockets = 1
	}
	wantHasAttributes := 0
	if hasAttributes {
		wantHasAttributes = 1
	}
	if gotID != int64(id) || gotCharacterID != int64(characterID) || gotLogin != login || gotCell != int(cell) || gotVnum != int64(vnum) || gotCount != int(count) || gotLocked != wantLocked || gotHasSockets != wantHasSockets || gotSocket0 != int(socket0) || gotSocket1 != int(socket1) || gotSocket2 != int(socket2) || gotHasAttributes != wantHasAttributes || gotAttr0Type != int(attr0Type) || gotAttr0Value != int(attr0Value) || gotAttr1Type != int(attr1Type) || gotAttr1Value != int(attr1Value) {
		t.Fatalf("item row mismatch for id %d: got id=%d character=%d login=%q cell=%d vnum=%d count=%d locked=%d has_sockets=%d sockets=(%d,%d,%d) has_attributes=%d attrs=(%d/%d,%d/%d) want character=%d login=%q cell=%d vnum=%d count=%d locked=%d has_sockets=%d sockets=(%d,%d,%d) has_attributes=%d attrs=(%d/%d,%d/%d)",
			id, gotID, gotCharacterID, gotLogin, gotCell, gotVnum, gotCount, gotLocked, gotHasSockets, gotSocket0, gotSocket1, gotSocket2, gotHasAttributes, gotAttr0Type, gotAttr0Value, gotAttr1Type, gotAttr1Value, characterID, login, cell, vnum, count, wantLocked, wantHasSockets, socket0, socket1, socket2, wantHasAttributes, attr0Type, attr0Value, attr1Type, attr1Value)
	}
}

func safeboxStateImportCharacter(id uint32, name string) loginticket.Character {
	return loginticket.Character{
		ID:       id,
		Name:     name,
		Job:      0,
		RaceNum:  0,
		Level:    1,
		X:        100,
		Y:        200,
		MapIndex: 1,
		Empire:   1,
		Gold:     0,
	}
}

func openSQLiteSafeboxStateImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "safebox-state-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite safebox-state import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
