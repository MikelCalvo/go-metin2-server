//go:build sqlite_harness

package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessMyShopUnitPricesImportInsertsCanonicalRows(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterMyShopUnitPricesMigrationVersion, err)
	}

	alphaWar := rosterExportCharacter(11, "AlphaWar")
	alphaWar.MyShopUnitPrices = []loginticket.MyShopUnitPrice{
		{Vnum: 27002, UnitPrice: 200},
		{Vnum: 27001, UnitPrice: 500},
	}
	bravoNinja := rosterExportCharacter(22, "BravoNinja")
	bravoNinja.MyShopUnitPrices = []loginticket.MyShopUnitPrice{
		{Vnum: 1, UnitPrice: 0},
	}
	accounts := []Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravoNinja}},
	}

	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	priceExport, err := ExportCharacterMyShopUnitPrices(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterMyShopUnitPrices: %v", err)
	}
	result, err := ImportCharacterMyShopUnitPrices(ctx, db, priceExport)
	if err != nil {
		t.Fatalf("ImportCharacterMyShopUnitPrices: %v", err)
	}
	if result.MigrationVersion != CharacterMyShopUnitPricesMigrationVersion || result.MigrationName != CharacterMyShopUnitPricesMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.CharacterCount != 2 || result.PriceRowCount != 3 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.CharacterIDs) != 2 || result.CharacterIDs[0] != 11 || result.CharacterIDs[1] != 22 {
		t.Fatalf("unexpected character ids: %+v", result.CharacterIDs)
	}

	var priceRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices`).Scan(&priceRows); err != nil {
		t.Fatalf("count myshop unit prices: %v", err)
	}
	if priceRows != 3 {
		t.Fatalf("price rows = %d, want 3", priceRows)
	}
	assertMyShopUnitPriceRow(t, db, 11, 27001, 500)
	assertMyShopUnitPriceRow(t, db, 11, 27002, 200)
	assertMyShopUnitPriceRow(t, db, 22, 1, 0)
}

func TestSQLiteHarnessMyShopUnitPricesImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterMyShopUnitPricesMigrationVersion, err)
	}

	character := rosterExportCharacter(11, "AlphaWar")
	character.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 27001, UnitPrice: 500}}
	accounts := []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
	priceExport, err := ExportCharacterMyShopUnitPrices(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterMyShopUnitPrices: %v", err)
	}
	if _, err := ImportCharacterMyShopUnitPrices(ctx, db, priceExport); err != nil {
		t.Fatalf("first ImportCharacterMyShopUnitPrices: %v", err)
	}
	if _, err := ImportCharacterMyShopUnitPrices(ctx, db, priceExport); err == nil {
		t.Fatal("second ImportCharacterMyShopUnitPrices succeeded, want unique conflict")
	}

	var priceRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices`).Scan(&priceRows); err != nil {
		t.Fatalf("count prices after failed reimport: %v", err)
	}
	if priceRows != 1 {
		t.Fatalf("price rows after failed reimport = %d, want 1", priceRows)
	}
}

func TestSQLiteHarnessMyShopUnitPricesImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()

	_, err := ImportCharacterMyShopUnitPrices(context.Background(), db, CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	})
	if !errors.Is(err, ErrCharacterMyShopUnitPricesImportSchemaRequired) {
		t.Fatalf("ImportCharacterMyShopUnitPrices on empty DB error = %v, want %v", err, ErrCharacterMyShopUnitPricesImportSchemaRequired)
	}
}

func TestSQLiteHarnessMyShopUnitPricesImportRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterMyShopUnitPricesMigrationVersion, err)
	}

	_, err := ImportCharacterMyShopUnitPrices(ctx, db, CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices:       []CharacterMyShopUnitPriceRow{{CharacterID: 11, Vnum: 27001, UnitPrice: 500}},
	})
	if err == nil {
		t.Fatal("ImportCharacterMyShopUnitPrices without parent character succeeded, want FK failure")
	}

	var priceRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices`).Scan(&priceRows); err != nil {
		t.Fatalf("count prices after FK failure: %v", err)
	}
	if priceRows != 0 {
		t.Fatalf("price rows after FK failure = %d, want 0", priceRows)
	}
}

func TestSQLiteHarnessMyShopUnitPricesImportReplaceOverwritesCanonicalRows(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterMyShopUnitPricesMigrationVersion, err)
	}

	character := rosterExportCharacter(11, "AlphaWar")
	character.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 27001, UnitPrice: 500}}
	accounts := []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	priceExport, err := ExportCharacterMyShopUnitPrices(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterMyShopUnitPrices: %v", err)
	}
	if _, err := ImportCharacterMyShopUnitPrices(ctx, db, priceExport); err != nil {
		t.Fatalf("first insert-only ImportCharacterMyShopUnitPrices: %v", err)
	}
	if _, err := ImportCharacterMyShopUnitPrices(ctx, db, priceExport); err == nil {
		t.Fatal("second insert-only ImportCharacterMyShopUnitPrices succeeded, want unique conflict")
	}

	replacedCharacter := rosterExportCharacter(11, "AlphaWar")
	replacedCharacter.MyShopUnitPrices = []loginticket.MyShopUnitPrice{
		{Vnum: 27001, UnitPrice: 750},
		{Vnum: 27002, UnitPrice: 100},
	}
	replacedAccounts := []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{replacedCharacter}}}
	replacedExport, err := ExportCharacterMyShopUnitPrices(replacedAccounts)
	if err != nil {
		t.Fatalf("ExportCharacterMyShopUnitPrices(replaced): %v", err)
	}
	result, err := ImportCharacterMyShopUnitPrices(ctx, db, replacedExport, ImportCharacterMyShopUnitPricesOptions{Replace: true})
	if err != nil {
		t.Fatalf("replace ImportCharacterMyShopUnitPrices: %v", err)
	}
	if !result.Replaced {
		t.Fatalf("replace result.Replaced = false, want true")
	}
	if result.CharacterCount != 1 || result.PriceRowCount != 2 {
		t.Fatalf("unexpected replace counts: %+v", result)
	}

	var priceRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices`).Scan(&priceRows); err != nil {
		t.Fatalf("count prices after replace: %v", err)
	}
	if priceRows != 2 {
		t.Fatalf("price rows after replace = %d, want 2", priceRows)
	}
	assertMyShopUnitPriceRow(t, db, 11, 27001, 750)
	assertMyShopUnitPriceRow(t, db, 11, 27002, 100)
}

func TestSQLiteHarnessMyShopUnitPricesImportReplaceLeavesUnlistedCharactersUntouched(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterMyShopUnitPricesMigrationVersion, err)
	}

	alphaWar := rosterExportCharacter(11, "AlphaWar")
	alphaWar.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 27001, UnitPrice: 500}}
	bravoNinja := rosterExportCharacter(22, "BravoNinja")
	bravoNinja.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 1, UnitPrice: 0}}
	accounts := []Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravoNinja}},
	}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	fullExport, err := ExportCharacterMyShopUnitPrices(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterMyShopUnitPrices: %v", err)
	}
	if _, err := ImportCharacterMyShopUnitPrices(ctx, db, fullExport); err != nil {
		t.Fatalf("seed ImportCharacterMyShopUnitPrices: %v", err)
	}

	alphaOnly := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		CharacterIDs:     []uint32{11},
		UnitPrices: []CharacterMyShopUnitPriceRow{
			{CharacterID: 11, Vnum: 27001, UnitPrice: 999},
		},
	}
	result, err := ImportCharacterMyShopUnitPrices(ctx, db, alphaOnly, ImportCharacterMyShopUnitPricesOptions{Replace: true})
	if err != nil {
		t.Fatalf("scoped replace ImportCharacterMyShopUnitPrices: %v", err)
	}
	if !result.Replaced || result.CharacterCount != 1 || result.PriceRowCount != 1 {
		t.Fatalf("unexpected scoped replace result: %+v", result)
	}

	var alphaPrices, bravoPrices int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices WHERE character_id = 11`).Scan(&alphaPrices); err != nil {
		t.Fatalf("count alpha prices: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices WHERE character_id = 22`).Scan(&bravoPrices); err != nil {
		t.Fatalf("count bravo prices: %v", err)
	}
	if alphaPrices != 1 || bravoPrices != 1 {
		t.Fatalf("scoped replace left unexpected counts alpha=%d bravo=%d", alphaPrices, bravoPrices)
	}
	assertMyShopUnitPriceRow(t, db, 11, 27001, 999)
	assertMyShopUnitPriceRow(t, db, 22, 1, 0)
}

func TestSQLiteHarnessMyShopUnitPricesImportReplaceWipesListedCharacterWithEmptyPrices(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterMyShopUnitPricesMigrationVersion, err)
	}

	character := rosterExportCharacter(11, "AlphaWar")
	character.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 27001, UnitPrice: 500}}
	accounts := []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	seedExport, err := ExportCharacterMyShopUnitPrices(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterMyShopUnitPrices: %v", err)
	}
	if _, err := ImportCharacterMyShopUnitPrices(ctx, db, seedExport); err != nil {
		t.Fatalf("seed ImportCharacterMyShopUnitPrices: %v", err)
	}

	emptyWipe := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		CharacterIDs:     []uint32{11},
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	}
	result, err := ImportCharacterMyShopUnitPrices(ctx, db, emptyWipe, ImportCharacterMyShopUnitPricesOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty wipe replace: %v", err)
	}
	if !result.Replaced || result.CharacterCount != 1 || result.PriceRowCount != 0 {
		t.Fatalf("unexpected empty wipe result: %+v", result)
	}

	var priceRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices`).Scan(&priceRows); err != nil {
		t.Fatalf("count prices after wipe: %v", err)
	}
	if priceRows != 0 {
		t.Fatalf("price rows after empty wipe = %d, want 0", priceRows)
	}
}

func TestSQLiteHarnessMyShopUnitPricesImportReplaceNoOpForEmptyCharacterIDs(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterMyShopUnitPricesMigrationVersion, err)
	}

	character := rosterExportCharacter(11, "AlphaWar")
	character.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 27001, UnitPrice: 500}}
	accounts := []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	seedExport, err := ExportCharacterMyShopUnitPrices(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterMyShopUnitPrices: %v", err)
	}
	if _, err := ImportCharacterMyShopUnitPrices(ctx, db, seedExport); err != nil {
		t.Fatalf("seed ImportCharacterMyShopUnitPrices: %v", err)
	}

	emptyIDs := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		CharacterIDs:     []uint32{},
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	}
	result, err := ImportCharacterMyShopUnitPrices(ctx, db, emptyIDs, ImportCharacterMyShopUnitPricesOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty character_ids replace: %v", err)
	}
	if !result.Replaced || result.CharacterCount != 0 {
		t.Fatalf("unexpected empty character_ids result: %+v", result)
	}

	var priceRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_myshop_unit_prices`).Scan(&priceRows); err != nil {
		t.Fatalf("count prices after no-op replace: %v", err)
	}
	if priceRows != 1 {
		t.Fatalf("price rows after no-op replace = %d, want 1", priceRows)
	}
}

func assertMyShopUnitPriceRow(t *testing.T, db *sql.DB, characterID, vnum, wantPrice uint32) {
	t.Helper()
	var (
		gotCharacterID int64
		gotVnum        int64
		gotPrice       int64
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT character_id, vnum, unit_price
FROM character_myshop_unit_prices WHERE character_id = ? AND vnum = ?`,
		characterID, vnum).Scan(&gotCharacterID, &gotVnum, &gotPrice); err != nil {
		t.Fatalf("select myshop unit price character %d vnum %d: %v", characterID, vnum, err)
	}
	if gotCharacterID != int64(characterID) || gotVnum != int64(vnum) || gotPrice != int64(wantPrice) {
		t.Fatalf("myshop unit price mismatch for character %d vnum %d: character=%d vnum=%d price=%d want price=%d",
			characterID, vnum, gotCharacterID, gotVnum, gotPrice, wantPrice)
	}
}

func openSQLiteMyShopUnitPricesImportDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "myshop-unit-prices-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite myshop unit-prices import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
