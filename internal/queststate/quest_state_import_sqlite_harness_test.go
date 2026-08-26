//go:build sqlite_harness

package queststate

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessQuestStateImportInsertsFlags(t *testing.T) {
	db := openSQLiteQuestStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterQuestStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterQuestStateMigrationVersion, err)
	}

	accounts := []accountstore.Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				questStateImportCharacter(11, "AlphaWar"),
			},
		},
		{
			Login:  "Bravo",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				questStateImportCharacter(22, "BravoNinja"),
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

	questExport := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags: []CharacterQuestFlagRow{
			{CharacterID: 22, Character: "BravoNinja", QuestRef: "quest:kill_qa_mob", Flag: "killed_qa_mob", Value: 2},
			{CharacterID: 11, Character: "AlphaWar", QuestRef: "quest:first_steps", Flag: "step", Value: 3},
			{CharacterID: 11, Character: "AlphaWar", QuestRef: "quest:first_steps", Flag: "met_guide", Value: 1},
		},
	}

	result, err := ImportCharacterQuestState(ctx, db, questExport)
	if err != nil {
		t.Fatalf("ImportCharacterQuestState: %v", err)
	}
	if result.MigrationVersion != CharacterQuestStateMigrationVersion || result.MigrationName != CharacterQuestStateMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.CharacterCount != 2 || result.FlagCount != 3 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.CharacterIDs) != 2 || result.CharacterIDs[0] != 11 || result.CharacterIDs[1] != 22 {
		t.Fatalf("unexpected character ids: %+v", result.CharacterIDs)
	}

	var flagRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_quest_flags`).Scan(&flagRows); err != nil {
		t.Fatalf("count quest flags: %v", err)
	}
	if flagRows != 3 {
		t.Fatalf("quest flag rows = %d, want 3", flagRows)
	}

	var (
		gotCharacterID int64
		gotQuestRef    string
		gotFlagName    string
		gotValue       int64
	)
	if err := db.QueryRowContext(ctx, `
SELECT character_id, quest_ref, flag_name, value
FROM character_quest_flags
WHERE character_id = ? AND quest_ref = ? AND flag_name = ?`,
		11, "quest:first_steps", "met_guide").Scan(
		&gotCharacterID, &gotQuestRef, &gotFlagName, &gotValue,
	); err != nil {
		t.Fatalf("select quest flag met_guide: %v", err)
	}
	if gotCharacterID != 11 || gotQuestRef != "quest:first_steps" || gotFlagName != "met_guide" || gotValue != 1 {
		t.Fatalf("met_guide row mismatch: character=%d quest_ref=%q flag=%q value=%d",
			gotCharacterID, gotQuestRef, gotFlagName, gotValue)
	}

	if err := db.QueryRowContext(ctx, `
SELECT character_id, quest_ref, flag_name, value
FROM character_quest_flags
WHERE character_id = ? AND quest_ref = ? AND flag_name = ?`,
		11, "quest:first_steps", "step").Scan(
		&gotCharacterID, &gotQuestRef, &gotFlagName, &gotValue,
	); err != nil {
		t.Fatalf("select quest flag step: %v", err)
	}
	if gotCharacterID != 11 || gotQuestRef != "quest:first_steps" || gotFlagName != "step" || gotValue != 3 {
		t.Fatalf("step row mismatch: character=%d quest_ref=%q flag=%q value=%d",
			gotCharacterID, gotQuestRef, gotFlagName, gotValue)
	}

	if err := db.QueryRowContext(ctx, `
SELECT character_id, quest_ref, flag_name, value
FROM character_quest_flags
WHERE character_id = ? AND quest_ref = ? AND flag_name = ?`,
		22, "quest:kill_qa_mob", "killed_qa_mob").Scan(
		&gotCharacterID, &gotQuestRef, &gotFlagName, &gotValue,
	); err != nil {
		t.Fatalf("select quest flag killed_qa_mob: %v", err)
	}
	if gotCharacterID != 22 || gotQuestRef != "quest:kill_qa_mob" || gotFlagName != "killed_qa_mob" || gotValue != 2 {
		t.Fatalf("killed_qa_mob row mismatch: character=%d quest_ref=%q flag=%q value=%d",
			gotCharacterID, gotQuestRef, gotFlagName, gotValue)
	}
}

func TestSQLiteHarnessQuestStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteQuestStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterQuestStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterQuestStateMigrationVersion, err)
	}

	accounts := []accountstore.Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				questStateImportCharacter(11, "AlphaWar"),
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

	questExport := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags: []CharacterQuestFlagRow{
			{CharacterID: 11, Character: "AlphaWar", QuestRef: "quest:first_steps", Flag: "met_guide", Value: 1},
		},
	}
	if _, err := ImportCharacterQuestState(ctx, db, questExport); err != nil {
		t.Fatalf("first ImportCharacterQuestState: %v", err)
	}

	_, err = ImportCharacterQuestState(ctx, db, questExport)
	if err == nil {
		t.Fatal("second ImportCharacterQuestState succeeded, want unique conflict")
	}

	var flagRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_quest_flags`).Scan(&flagRows); err != nil {
		t.Fatalf("count flags after failed reimport: %v", err)
	}
	if flagRows != 1 {
		t.Fatalf("flag rows after failed reimport = %d, want 1 (no partial second import)", flagRows)
	}
}

func TestSQLiteHarnessQuestStateImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteQuestStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            []CharacterQuestFlagRow{},
	}

	_, err := ImportCharacterQuestState(ctx, db, export)
	if !errors.Is(err, ErrCharacterQuestStateImportSchemaRequired) {
		t.Fatalf("ImportCharacterQuestState on empty DB error = %v, want %v", err, ErrCharacterQuestStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessQuestStateImportRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLiteQuestStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterQuestStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterQuestStateMigrationVersion, err)
	}

	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags: []CharacterQuestFlagRow{
			{CharacterID: 11, Character: "AlphaWar", QuestRef: "quest:first_steps", Flag: "met_guide", Value: 1},
		},
	}

	_, err := ImportCharacterQuestState(ctx, db, export)
	if err == nil {
		t.Fatal("ImportCharacterQuestState without parent character succeeded, want FK failure")
	}

	var flagRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_quest_flags`).Scan(&flagRows); err != nil {
		t.Fatalf("count flags after FK failure: %v", err)
	}
	if flagRows != 0 {
		t.Fatalf("flag rows after FK failure = %d, want 0", flagRows)
	}
}

func TestSQLiteHarnessQuestStateImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLiteQuestStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterQuestStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterQuestStateMigrationVersion, err)
	}

	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            []CharacterQuestFlagRow{},
	}
	result, err := ImportCharacterQuestState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportCharacterQuestState(empty): %v", err)
	}
	if result.CharacterCount != 0 || result.FlagCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
	if len(result.CharacterIDs) != 0 {
		t.Fatalf("empty import character ids = %+v, want empty", result.CharacterIDs)
	}
}

func questStateImportCharacter(id uint32, name string) loginticket.Character {
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

func openSQLiteQuestStateImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "quest-state-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite quest-state import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
