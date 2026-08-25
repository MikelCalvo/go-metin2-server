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

func TestSQLiteHarnessRosterImportInsertsAccountsAndCharacters(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AccountCharacterRosterMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AccountCharacterRosterMigrationVersion, err)
	}

	export, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				{
					ID:          11,
					Name:        "AlphaWar",
					Job:         0,
					RaceNum:     4,
					Level:       5,
					PlayMinutes: 42,
					ST:          6,
					HT:          7,
					DX:          8,
					IQ:          9,
					MainPart:    11200,
					ChangeName:  1,
					HairPart:    100,
					X:           111,
					Y:           222,
					Z:           333,
					MapIndex:    1,
					Empire:      1,
					SkillGroup:  2,
					GuildID:     99,
					GuildName:   "GuildA",
					Gold:        1234,
				},
				{},
				{},
				rosterExportCharacter(12, "AlphaSura"),
			},
		},
		{
			Login:  "Bravo",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				{
					ID:       22,
					Name:     "BravoNinja",
					Job:      1,
					RaceNum:  1,
					Level:    7,
					X:        1700,
					Y:        2800,
					MapIndex: 42,
					Empire:   2,
					Gold:     500,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}

	result, err := ImportAccountCharacterRoster(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
	if result.MigrationVersion != AccountCharacterRosterMigrationVersion || result.MigrationName != AccountCharacterRosterMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.AccountCount != 2 || result.CharacterCount != 3 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.AccountIDs) != 2 || len(result.CharacterIDs) != 3 {
		t.Fatalf("unexpected import ids: %+v", result)
	}

	var accountRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accountRows); err != nil {
		t.Fatalf("count accounts: %v", err)
	}
	if accountRows != 2 {
		t.Fatalf("accounts rows = %d, want 2", accountRows)
	}

	var characterRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM characters`).Scan(&characterRows); err != nil {
		t.Fatalf("count characters: %v", err)
	}
	if characterRows != 3 {
		t.Fatalf("characters rows = %d, want 3", characterRows)
	}

	var (
		gotLogin           string
		gotLoginNormalized string
		gotEmpire          int
	)
	if err := db.QueryRowContext(ctx, `SELECT login, login_normalized, empire FROM accounts WHERE id = ?`, export.Accounts[0].ID).
		Scan(&gotLogin, &gotLoginNormalized, &gotEmpire); err != nil {
		t.Fatalf("select first account: %v", err)
	}
	if gotLogin != "Alpha" || gotLoginNormalized != "alpha" || gotEmpire != 1 {
		t.Fatalf("first account row = (%q,%q,%d), want (Alpha,alpha,1)", gotLogin, gotLoginNormalized, gotEmpire)
	}

	var (
		gotName           string
		gotNameNormalized string
		gotAccountID      int64
		gotSlot           int
		gotLevel          int
		gotMapIndex       int
		gotGold           int64
		gotGuildName      string
	)
	if err := db.QueryRowContext(ctx, `
SELECT name, name_normalized, account_id, slot, level, map_index, gold, guild_name
FROM characters WHERE id = ?`, 11).Scan(
		&gotName, &gotNameNormalized, &gotAccountID, &gotSlot, &gotLevel, &gotMapIndex, &gotGold, &gotGuildName,
	); err != nil {
		t.Fatalf("select character 11: %v", err)
	}
	if gotName != "AlphaWar" || gotNameNormalized != "alphawar" || gotAccountID != export.Accounts[0].ID || gotSlot != 0 || gotLevel != 5 || gotMapIndex != 1 || gotGold != 1234 || gotGuildName != "GuildA" {
		t.Fatalf("character 11 row mismatch: name=%q norm=%q account=%d slot=%d level=%d map=%d gold=%d guild=%q",
			gotName, gotNameNormalized, gotAccountID, gotSlot, gotLevel, gotMapIndex, gotGold, gotGuildName)
	}
}

func TestSQLiteHarnessRosterImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AccountCharacterRosterMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AccountCharacterRosterMigrationVersion, err)
	}

	export, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, export); err != nil {
		t.Fatalf("first ImportAccountCharacterRoster: %v", err)
	}

	_, err = ImportAccountCharacterRoster(ctx, db, export)
	if err == nil {
		t.Fatal("second ImportAccountCharacterRoster succeeded, want unique conflict")
	}

	var accountRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accountRows); err != nil {
		t.Fatalf("count accounts after failed reimport: %v", err)
	}
	if accountRows != 1 {
		t.Fatalf("accounts rows after failed reimport = %d, want 1 (no partial second import)", accountRows)
	}
}

func TestSQLiteHarnessRosterImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	export, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}

	_, err = ImportAccountCharacterRoster(ctx, db, export)
	if !errors.Is(err, ErrAccountCharacterRosterImportSchemaRequired) {
		t.Fatalf("ImportAccountCharacterRoster on empty DB error = %v, want %v", err, ErrAccountCharacterRosterImportSchemaRequired)
	}
}

func TestSQLiteHarnessRosterImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AccountCharacterRosterMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AccountCharacterRosterMigrationVersion, err)
	}

	export := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		Accounts:         []AccountCharacterRosterAccountRow{},
		Characters:       []AccountCharacterRosterCharacterRow{},
	}
	result, err := ImportAccountCharacterRoster(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportAccountCharacterRoster(empty): %v", err)
	}
	if result.AccountCount != 0 || result.CharacterCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
}

func openSQLiteRosterImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "roster-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite roster import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
