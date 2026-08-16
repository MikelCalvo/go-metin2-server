package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestExportCharacterPointStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	alphaWar := rosterExportCharacter(11, "AlphaWar")
	alphaWar.Points[0] = 12
	alphaWar.Points[1] = -3
	alphaWar.Points[254] = 99

	bravoNinja := rosterExportCharacter(22, "BravoNinja")
	bravoNinja.Points[0] = 1
	bravoNinja.Points[100] = 700

	export, err := ExportCharacterPointState([]Account{
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravoNinja}},
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
	})
	if err != nil {
		t.Fatalf("export character point state: %v", err)
	}
	if export.MigrationVersion != CharacterPointStateMigrationVersion || export.MigrationName != CharacterPointStateMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	if len(export.Points) != 2*255 {
		t.Fatalf("expected 255 point rows per non-empty character, got %d", len(export.Points))
	}
	wantPrefix := []CharacterPointRow{
		{CharacterID: 11, PointIndex: 0, Value: 12},
		{CharacterID: 11, PointIndex: 1, Value: -3},
		{CharacterID: 11, PointIndex: 2, Value: 0},
	}
	if !reflect.DeepEqual(export.Points[:3], wantPrefix) {
		t.Fatalf("unexpected first point rows:\n got: %#v\nwant: %#v", export.Points[:3], wantPrefix)
	}
	if row := export.Points[254]; row.CharacterID != 11 || row.PointIndex != 254 || row.Value != 99 {
		t.Fatalf("expected alpha final point row at index 254, got %#v", row)
	}
	if row := export.Points[255]; row.CharacterID != 22 || row.PointIndex != 0 || row.Value != 1 {
		t.Fatalf("expected bravo rows after alpha point vector, got %#v", row)
	}
	if row := export.Points[255+100]; row.CharacterID != 22 || row.PointIndex != 100 || row.Value != 700 {
		t.Fatalf("expected bravo point 100 to be preserved, got %#v", row)
	}

	exportAgain, err := ExportCharacterPointState([]Account{
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravoNinja}},
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
	})
	if err != nil {
		t.Fatalf("export character point state again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic character point-state export:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportCharacterPointStateRejectsInvalidRosterPrerequisites(t *testing.T) {
	invalid := rosterExportCharacter(0, "MissingID")
	invalid.Points[1] = 100

	_, err := ExportCharacterPointState([]Account{{Login: "Alpha", Characters: []loginticket.Character{invalid}}})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount, got %v", err)
	}
}

func TestFileStoreExportCharacterPointStateReadsCommittedSnapshots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	character := rosterExportCharacter(200, "BetaWar")
	character.Points[0] = 15
	character.Points[1] = -1
	character.Points[254] = 42
	if err := store.Save(Account{Login: "Beta", Empire: 2, Characters: []loginticket.Character{character}}); err != nil {
		t.Fatalf("save beta account: %v", err)
	}
	if err := store.Save(Account{Login: "Alpha", Empire: 1}); err != nil {
		t.Fatalf("save alpha account: %v", err)
	}

	export, err := store.ExportCharacterPointState()
	if err != nil {
		t.Fatalf("file-store character point-state export: %v", err)
	}
	if len(export.Points) != 255 {
		t.Fatalf("expected one committed character point vector, got %d rows", len(export.Points))
	}
	if export.Points[0].CharacterID != 200 || export.Points[0].PointIndex != 0 || export.Points[0].Value != 15 {
		t.Fatalf("unexpected file-store first point row: %#v", export.Points[0])
	}
	if export.Points[1].CharacterID != 200 || export.Points[1].PointIndex != 1 || export.Points[1].Value != -1 {
		t.Fatalf("unexpected file-store negative point row: %#v", export.Points[1])
	}
	if export.Points[254].CharacterID != 200 || export.Points[254].PointIndex != 254 || export.Points[254].Value != 42 {
		t.Fatalf("unexpected file-store final point row: %#v", export.Points[254])
	}
}
