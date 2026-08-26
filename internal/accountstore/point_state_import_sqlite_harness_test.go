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

func TestSQLiteHarnessPointStateImportInsertsFixedWidthVectors(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}

	alphaWar := rosterExportCharacter(11, "AlphaWar")
	alphaWar.Points[0] = 12
	alphaWar.Points[1] = -3
	alphaWar.Points[254] = 99

	bravoNinja := rosterExportCharacter(22, "BravoNinja")
	bravoNinja.Points[0] = 1
	bravoNinja.Points[100] = 700

	accounts := []Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				alphaWar,
			},
		},
		{
			Login:  "Bravo",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				bravoNinja,
			},
		},
	}

	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	pointExport, err := ExportCharacterPointState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterPointState: %v", err)
	}

	result, err := ImportCharacterPointState(ctx, db, pointExport)
	if err != nil {
		t.Fatalf("ImportCharacterPointState: %v", err)
	}
	if result.MigrationVersion != CharacterPointStateMigrationVersion || result.MigrationName != CharacterPointStateMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.CharacterCount != 2 || result.PointRowCount != 2*255 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.CharacterIDs) != 2 || result.CharacterIDs[0] != 11 || result.CharacterIDs[1] != 22 {
		t.Fatalf("unexpected character ids: %+v", result.CharacterIDs)
	}

	var pointRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_points`).Scan(&pointRows); err != nil {
		t.Fatalf("count character points: %v", err)
	}
	if pointRows != 510 {
		t.Fatalf("point rows = %d, want 510", pointRows)
	}

	assertPointRow(t, db, 11, 0, 12)
	assertPointRow(t, db, 11, 1, -3)
	assertPointRow(t, db, 11, 2, 0)
	assertPointRow(t, db, 11, 254, 99)
	assertPointRow(t, db, 22, 0, 1)
	assertPointRow(t, db, 22, 100, 700)
	assertPointRow(t, db, 22, 254, 0)
}

func TestSQLiteHarnessPointStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}

	character := rosterExportCharacter(11, "AlphaWar")
	character.Points[0] = 12
	accounts := []Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				character,
			},
		},
	}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	pointExport, err := ExportCharacterPointState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterPointState: %v", err)
	}
	if _, err := ImportCharacterPointState(ctx, db, pointExport); err != nil {
		t.Fatalf("first ImportCharacterPointState: %v", err)
	}

	_, err = ImportCharacterPointState(ctx, db, pointExport)
	if err == nil {
		t.Fatal("second ImportCharacterPointState succeeded, want unique conflict")
	}

	var pointRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_points`).Scan(&pointRows); err != nil {
		t.Fatalf("count points after failed reimport: %v", err)
	}
	if pointRows != 255 {
		t.Fatalf("point rows after failed reimport = %d, want 255 (no partial second import)", pointRows)
	}
}

func TestSQLiteHarnessPointStateImportRejectsMissingSchema(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           []CharacterPointRow{},
	}

	_, err := ImportCharacterPointState(ctx, db, export)
	if !errors.Is(err, ErrCharacterPointStateImportSchemaRequired) {
		t.Fatalf("ImportCharacterPointState on empty DB error = %v, want %v", err, ErrCharacterPointStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessPointStateImportRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}

	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           fullPointVector(11, 7),
	}

	_, err := ImportCharacterPointState(ctx, db, export)
	if err == nil {
		t.Fatal("ImportCharacterPointState without parent character succeeded, want FK failure")
	}

	var pointRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_points`).Scan(&pointRows); err != nil {
		t.Fatalf("count points after FK failure: %v", err)
	}
	if pointRows != 0 {
		t.Fatalf("point rows after FK failure = %d, want 0", pointRows)
	}
}

func TestSQLiteHarnessPointStateImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}

	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           []CharacterPointRow{},
	}
	result, err := ImportCharacterPointState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportCharacterPointState(empty): %v", err)
	}
	if result.CharacterCount != 0 || result.PointRowCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
	if len(result.CharacterIDs) != 0 {
		t.Fatalf("empty import character ids = %+v, want empty", result.CharacterIDs)
	}
}

func assertPointRow(t *testing.T, db *sql.DB, characterID uint32, pointIndex uint8, wantValue int32) {
	t.Helper()

	var (
		gotCharacterID int64
		gotPointIndex  int
		gotValue       int64
	)
	if err := db.QueryRowContext(context.Background(), `
SELECT character_id, point_index, value
FROM character_points WHERE character_id = ? AND point_index = ?`,
		characterID, pointIndex).Scan(&gotCharacterID, &gotPointIndex, &gotValue); err != nil {
		t.Fatalf("select point character %d index %d: %v", characterID, pointIndex, err)
	}
	if gotCharacterID != int64(characterID) || gotPointIndex != int(pointIndex) || gotValue != int64(wantValue) {
		t.Fatalf("point row mismatch for character %d index %d: character=%d index=%d value=%d want value=%d",
			characterID, pointIndex, gotCharacterID, gotPointIndex, gotValue, wantValue)
	}
}

func openSQLitePointStateImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "point-state-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite point-state import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
