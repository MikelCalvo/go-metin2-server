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

func TestSQLiteHarnessRosterImportReplaceOverwritesCanonicalRows(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AccountCharacterRosterMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AccountCharacterRosterMigrationVersion, err)
	}

	seedExport, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster(seed): %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, seedExport); err != nil {
		t.Fatalf("first insert-only ImportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, seedExport); err == nil {
		t.Fatal("second insert-only ImportAccountCharacterRoster succeeded, want unique conflict")
	}

	replacedExport, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 3,
			Characters: []loginticket.Character{
				{
					ID:       11,
					Name:     "AlphaWar",
					Job:      0,
					RaceNum:  4,
					Level:    9,
					X:        500,
					Y:        600,
					MapIndex: 2,
					Empire:   3,
					Gold:     999,
				},
				{},
				{},
				rosterExportCharacter(12, "AlphaSura"),
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster(replaced): %v", err)
	}
	result, err := ImportAccountCharacterRoster(ctx, db, replacedExport, ImportAccountCharacterRosterOptions{Replace: true})
	if err != nil {
		t.Fatalf("replace ImportAccountCharacterRoster: %v", err)
	}
	if !result.Replaced {
		t.Fatalf("replace result.Replaced = false, want true")
	}
	if result.AccountCount != 1 || result.CharacterCount != 2 {
		t.Fatalf("unexpected replace counts: %+v", result)
	}

	var accountRows, characterRows, empire int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accountRows); err != nil {
		t.Fatalf("count accounts after replace: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM characters`).Scan(&characterRows); err != nil {
		t.Fatalf("count characters after replace: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT empire FROM accounts WHERE id = ?`, replacedExport.Accounts[0].ID).Scan(&empire); err != nil {
		t.Fatalf("select replaced account empire: %v", err)
	}
	if accountRows != 1 || characterRows != 2 || empire != 3 {
		t.Fatalf("after replace accounts=%d characters=%d empire=%d, want 1/2/3", accountRows, characterRows, empire)
	}

	var gotLevel, gotMapIndex int
	var gotGold int64
	if err := db.QueryRowContext(ctx, `SELECT level, map_index, gold FROM characters WHERE id = 11`).
		Scan(&gotLevel, &gotMapIndex, &gotGold); err != nil {
		t.Fatalf("select replaced character 11: %v", err)
	}
	if gotLevel != 9 || gotMapIndex != 2 || gotGold != 999 {
		t.Fatalf("character 11 after replace level=%d map=%d gold=%d, want 9/2/999", gotLevel, gotMapIndex, gotGold)
	}
	var slot3Name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM characters WHERE id = 12`).Scan(&slot3Name); err != nil {
		t.Fatalf("select replaced character 12: %v", err)
	}
	if slot3Name != "AlphaSura" {
		t.Fatalf("character 12 name = %q, want AlphaSura", slot3Name)
	}
}

func TestSQLiteHarnessRosterImportReplaceLeavesUnlistedAccountsUntouched(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AccountCharacterRosterMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AccountCharacterRosterMigrationVersion, err)
	}

	seedExport, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
			},
		},
		{
			Login:  "Bravo",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				rosterExportCharacter(22, "BravoNinja"),
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster(seed): %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, seedExport); err != nil {
		t.Fatalf("seed ImportAccountCharacterRoster: %v", err)
	}

	alphaOnly, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 5,
			Characters: []loginticket.Character{
				{
					ID:       11,
					Name:     "AlphaWar",
					Job:      0,
					RaceNum:  4,
					Level:    12,
					X:        10,
					Y:        20,
					MapIndex: 3,
					Empire:   5,
					Gold:     42,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster(alpha-only): %v", err)
	}
	// Force declared scope to the seeded Alpha account id so wipe/replace stays
	// account-scoped even if export helper id derivation differs across runs.
	alphaOnly.AccountIDs = []int64{seedExport.Accounts[0].ID}
	alphaOnly.Accounts[0].ID = seedExport.Accounts[0].ID
	alphaOnly.Characters[0].AccountID = seedExport.Accounts[0].ID

	result, err := ImportAccountCharacterRoster(ctx, db, alphaOnly, ImportAccountCharacterRosterOptions{Replace: true})
	if err != nil {
		t.Fatalf("scoped replace ImportAccountCharacterRoster: %v", err)
	}
	if !result.Replaced || result.AccountCount != 1 || result.CharacterCount != 1 {
		t.Fatalf("unexpected scoped replace result: %+v", result)
	}

	var alphaAccounts, bravoAccounts, alphaChars, bravoChars int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE id = ?`, seedExport.Accounts[0].ID).Scan(&alphaAccounts); err != nil {
		t.Fatalf("count alpha accounts: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE id = ?`, seedExport.Accounts[1].ID).Scan(&bravoAccounts); err != nil {
		t.Fatalf("count bravo accounts: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM characters WHERE account_id = ?`, seedExport.Accounts[0].ID).Scan(&alphaChars); err != nil {
		t.Fatalf("count alpha characters: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM characters WHERE account_id = ?`, seedExport.Accounts[1].ID).Scan(&bravoChars); err != nil {
		t.Fatalf("count bravo characters: %v", err)
	}
	if alphaAccounts != 1 || bravoAccounts != 1 || alphaChars != 1 || bravoChars != 1 {
		t.Fatalf("scoped replace left unexpected counts alphaA=%d bravoA=%d alphaC=%d bravoC=%d", alphaAccounts, bravoAccounts, alphaChars, bravoChars)
	}

	var bravoName string
	if err := db.QueryRowContext(ctx, `SELECT name FROM characters WHERE id = 22`).Scan(&bravoName); err != nil {
		t.Fatalf("select untouched bravo character: %v", err)
	}
	if bravoName != "BravoNinja" {
		t.Fatalf("bravo character name = %q, want BravoNinja", bravoName)
	}
}

func TestSQLiteHarnessRosterImportReplaceWipesListedAccountWithEmptyRows(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AccountCharacterRosterMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AccountCharacterRosterMigrationVersion, err)
	}

	seedExport, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster(seed): %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, seedExport); err != nil {
		t.Fatalf("seed ImportAccountCharacterRoster: %v", err)
	}

	emptyWipe := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		AccountIDs:       []int64{seedExport.Accounts[0].ID},
		Accounts:         []AccountCharacterRosterAccountRow{},
		Characters:       []AccountCharacterRosterCharacterRow{},
	}
	result, err := ImportAccountCharacterRoster(ctx, db, emptyWipe, ImportAccountCharacterRosterOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty wipe replace: %v", err)
	}
	if !result.Replaced || result.AccountCount != 1 || result.CharacterCount != 0 {
		t.Fatalf("unexpected empty wipe result: %+v", result)
	}

	var accountRows, characterRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accountRows); err != nil {
		t.Fatalf("count accounts after wipe: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM characters`).Scan(&characterRows); err != nil {
		t.Fatalf("count characters after wipe: %v", err)
	}
	if accountRows != 0 || characterRows != 0 {
		t.Fatalf("after empty wipe accounts=%d characters=%d, want 0/0", accountRows, characterRows)
	}
}

func TestSQLiteHarnessRosterImportReplaceNoOpForEmptyAccountIDs(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, AccountCharacterRosterMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", AccountCharacterRosterMigrationVersion, err)
	}

	seedExport, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster(seed): %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, seedExport); err != nil {
		t.Fatalf("seed ImportAccountCharacterRoster: %v", err)
	}

	emptyIDs := AccountCharacterRosterExport{
		MigrationVersion: AccountCharacterRosterMigrationVersion,
		MigrationName:    AccountCharacterRosterMigrationName,
		AccountIDs:       []int64{},
		Accounts:         []AccountCharacterRosterAccountRow{},
		Characters:       []AccountCharacterRosterCharacterRow{},
	}
	result, err := ImportAccountCharacterRoster(ctx, db, emptyIDs, ImportAccountCharacterRosterOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty account_ids replace: %v", err)
	}
	if !result.Replaced || result.AccountCount != 0 {
		t.Fatalf("unexpected empty account_ids result: %+v", result)
	}

	var accountRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accountRows); err != nil {
		t.Fatalf("count accounts after no-op replace: %v", err)
	}
	if accountRows != 1 {
		t.Fatalf("accounts after no-op replace = %d, want 1", accountRows)
	}
}

func TestSQLiteHarnessRosterImportReplaceFailsClosedOnChildFKDependents(t *testing.T) {
	db := openSQLiteRosterImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemStateMigrationVersion, err)
	}

	seedExport, err := ExportAccountCharacterRoster([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				rosterExportCharacter(11, "AlphaWar"),
			},
		},
		{
			Login:  "Bravo",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				rosterExportCharacter(22, "BravoNinja"),
			},
		},
	})
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster(seed): %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, seedExport); err != nil {
		t.Fatalf("seed ImportAccountCharacterRoster: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO character_inventory_items (
    id, character_id, slot, vnum, count, locked
) VALUES (1001, 11, 0, 27001, 1, 0)`); err != nil {
		t.Fatalf("seed tip-0003 inventory dependent: %v", err)
	}

	replaceExport := seedExport
	replaceExport.Accounts = []AccountCharacterRosterAccountRow{seedExport.Accounts[0]}
	replaceExport.Characters = []AccountCharacterRosterCharacterRow{seedExport.Characters[0]}
	replaceExport.AccountIDs = []int64{seedExport.Accounts[0].ID}

	_, err = ImportAccountCharacterRoster(ctx, db, replaceExport, ImportAccountCharacterRosterOptions{Replace: true})
	if err == nil {
		t.Fatal("replace with tip-0003 dependents succeeded, want FK fail-closed")
	}

	var alphaAccounts, bravoAccounts, inventoryRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE id = ?`, seedExport.Accounts[0].ID).Scan(&alphaAccounts); err != nil {
		t.Fatalf("count alpha accounts after FK failure: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE id = ?`, seedExport.Accounts[1].ID).Scan(&bravoAccounts); err != nil {
		t.Fatalf("count bravo accounts after FK failure: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items`).Scan(&inventoryRows); err != nil {
		t.Fatalf("count inventory after FK failure: %v", err)
	}
	if alphaAccounts != 1 || bravoAccounts != 1 || inventoryRows != 1 {
		t.Fatalf("FK fail-closed left unexpected state alpha=%d bravo=%d inventory=%d", alphaAccounts, bravoAccounts, inventoryRows)
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
