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

func TestSQLiteHarnessSQLPointStateStoreRoundTripPersistsFixedWidthVectors(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}

	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.Empire = 1
	alpha.Points[0] = 12
	alpha.Points[CharacterPointStateHPIndex] = -3
	alpha.Points[254] = 99
	bravo := rosterExportCharacter(22, "BravoNinja")
	bravo.Empire = 2
	bravo.Points[0] = 1
	bravo.Points[100] = 700

	accounts := []Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}},
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravo}},
	}
	seedItemStateRoster(t, db, accounts)

	store := NewSQLPointStateStore(db)
	if err := store.SaveAccounts(accounts); err != nil {
		t.Fatalf("SQLPointStateStore.SaveAccounts: %v", err)
	}

	loadedAlpha, err := store.Load("Alpha")
	if err != nil {
		t.Fatalf("SQLPointStateStore.Load(Alpha): %v", err)
	}
	wantAlpha := pointStateAccountForCompare(accounts[0])
	if !reflect.DeepEqual(loadedAlpha, wantAlpha) {
		t.Fatalf("unexpected Alpha snapshot:\n got: %#v\nwant: %#v", loadedAlpha, wantAlpha)
	}
	loadedBravo, err := store.Load("bravo")
	if err != nil {
		t.Fatalf("SQLPointStateStore.Load(bravo): %v", err)
	}
	wantBravo := pointStateAccountForCompare(accounts[1])
	if !reflect.DeepEqual(loadedBravo, wantBravo) {
		t.Fatalf("unexpected Bravo snapshot:\n got: %#v\nwant: %#v", loadedBravo, wantBravo)
	}

	listed, err := store.List()
	if err != nil {
		t.Fatalf("SQLPointStateStore.List: %v", err)
	}
	wantListed := []Account{wantAlpha, wantBravo}
	if !reflect.DeepEqual(listed, wantListed) {
		t.Fatalf("unexpected SQL list:\n got: %#v\nwant: %#v", listed, wantListed)
	}

	export, err := store.ExportCharacterPointState()
	if err != nil {
		t.Fatalf("SQLPointStateStore.ExportCharacterPointState: %v", err)
	}
	wantExport, err := ExportCharacterPointState(wantListed)
	if err != nil {
		t.Fatalf("ExportCharacterPointState: %v", err)
	}
	if !reflect.DeepEqual(export, wantExport) {
		t.Fatalf("unexpected SQL export:\n got: %#v\nwant: %#v", export, wantExport)
	}
	if export.MigrationVersion != CharacterPointStateMigrationVersion || export.MigrationName != CharacterPointStateMigrationName {
		t.Fatalf("export identity = %d %q, want tip-0011", export.MigrationVersion, export.MigrationName)
	}
	hp, present, err := CharacterPointStateProjectedHP(export, 11)
	if err != nil || !present || hp != -3 {
		t.Fatalf("projected HP = %d present=%v err=%v, want -3", hp, present, err)
	}
}

func TestSQLiteHarnessSQLPointStateStoreLoadEmptyTablesReturnsZeroVector(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.Empire = 1
	alpha.Points[1] = 40
	accounts := []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}}
	seedItemStateRoster(t, db, accounts)

	store := NewSQLPointStateStore(db)
	loaded, err := store.Load("Alpha")
	if err != nil {
		t.Fatalf("SQLPointStateStore.Load empty points: %v", err)
	}
	want := pointStateAccountForCompare(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{rosterExportCharacter(11, "AlphaWar")}})
	want.Characters[0].Empire = 1
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("empty point snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}
	if loaded.Characters[0].Points != ([255]int32{}) {
		t.Fatalf("empty tables loaded non-zero points: %#v", loaded.Characters[0].Points)
	}
	if _, err := store.Load("Missing"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("Load(missing) error = %v, want %v", err, ErrAccountNotFound)
	}
}

func TestSQLiteHarnessSQLPointStateStoreRejectsMissingSchema(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	store := NewSQLPointStateStore(db)
	if _, err := store.Load("Alpha"); !errors.Is(err, ErrCharacterPointStateImportSchemaRequired) {
		t.Fatalf("SQLPointStateStore.Load empty-ledger error = %v, want %v", err, ErrCharacterPointStateImportSchemaRequired)
	}
	if err := store.Save(Account{Login: "Alpha", Empire: 1}); !errors.Is(err, ErrCharacterPointStateImportSchemaRequired) {
		t.Fatalf("SQLPointStateStore.Save empty-ledger error = %v, want %v", err, ErrCharacterPointStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessSQLPointStateStoreSaveReplacesListedCharactersOnly(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.Empire = 1
	alpha.Points[1] = 40
	bravo := rosterExportCharacter(22, "BravoNinja")
	bravo.Empire = 2
	bravo.Points[1] = 80
	accounts := []Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}},
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravo}},
	}
	seedItemStateRoster(t, db, accounts)

	store := NewSQLPointStateStore(db)
	if err := store.SaveAccounts(accounts); err != nil {
		t.Fatalf("first SQLPointStateStore.SaveAccounts: %v", err)
	}

	secondAlpha := alpha
	secondAlpha.Points = [255]int32{}
	secondAlpha.Points[1] = 7
	secondAlpha.Points[254] = -1
	if err := store.Save(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{secondAlpha}}); err != nil {
		t.Fatalf("second SQLPointStateStore.Save: %v", err)
	}
	if err := store.SaveAccounts(nil); err != nil {
		t.Fatalf("empty SQLPointStateStore.SaveAccounts: %v", err)
	}

	loaded, err := store.List()
	if err != nil {
		t.Fatalf("SQLPointStateStore.List after replace: %v", err)
	}
	wantAlpha := pointStateAccountForCompare(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{secondAlpha}})
	wantBravo := pointStateAccountForCompare(Account{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravo}})
	if !reflect.DeepEqual(loaded, []Account{wantAlpha, wantBravo}) {
		t.Fatalf("replaced snapshot:\n got: %#v\nwant alpha %#v\nwant bravo %#v", loaded, wantAlpha, wantBravo)
	}

	var pointRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_points`).Scan(&pointRows); err != nil {
		t.Fatalf("count points after replace: %v", err)
	}
	if pointRows != 510 {
		t.Fatalf("point rows after scoped replace = %d, want 510", pointRows)
	}
	assertPointRow(t, db, 11, CharacterPointStateHPIndex, 7)
	assertPointRow(t, db, 11, 254, -1)
	assertPointRow(t, db, 22, CharacterPointStateHPIndex, 80)
}

func TestSQLiteHarnessSQLPointStateStoreRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}

	character := rosterExportCharacter(11, "AlphaWar")
	character.Points[1] = 12
	err := NewSQLPointStateStore(db).Save(Account{
		Login:      "Alpha",
		Empire:     1,
		Characters: []loginticket.Character{character},
	})
	if err == nil {
		t.Fatal("SQLPointStateStore.Save without parent character succeeded, want FK failure")
	}

	var pointRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_points`).Scan(&pointRows); err != nil {
		t.Fatalf("count points after FK failure: %v", err)
	}
	if pointRows != 0 {
		t.Fatalf("point rows after FK failure = %d, want 0", pointRows)
	}
}

func TestSQLiteHarnessSQLPointStateStoreLoadRejectsSparseVector(t *testing.T) {
	db := openSQLitePointStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterPointStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterPointStateMigrationVersion, err)
	}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.Empire = 1
	seedItemStateRoster(t, db, []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}})
	if _, err := db.ExecContext(ctx, `INSERT INTO character_points (character_id, point_index, value) VALUES (11, 1, 40)`); err != nil {
		t.Fatalf("insert sparse point: %v", err)
	}

	_, err := NewSQLPointStateStore(db).Load("Alpha")
	if err == nil || !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("SQLPointStateStore.Load(sparse) error = %v, want invalid account", err)
	}
}

func pointStateAccountForCompare(account Account) Account {
	shells := make([]loginticket.Character, accountCharacterRosterPlayerSlots)
	for i, character := range normalizeAccountCharacters(account.Characters) {
		if i >= len(shells) || character.IsEmptySlot() {
			continue
		}
		shells[i] = loginticket.Character{
			ID:       character.ID,
			Name:     character.Name,
			Level:    1,
			MapIndex: 1,
			Empire:   account.Empire,
			Points:   character.Points,
		}
		shells[i].NormalizeItemState()
	}
	for i := range shells {
		if shells[i].IsEmptySlot() {
			shells[i].NormalizeItemState()
		}
	}
	return Account{Login: account.Login, Empire: account.Empire, Characters: shells}
}
