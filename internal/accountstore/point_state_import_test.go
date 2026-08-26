package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestImportCharacterPointStateRejectsNilExecutor(t *testing.T) {
	character := rosterExportCharacter(11, "AlphaWar")
	character.Points[0] = 12
	character.Points[1] = -3

	export, err := ExportCharacterPointState([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				character,
			},
		},
	})
	if err != nil {
		t.Fatalf("export sample point state: %v", err)
	}

	_, err = ImportCharacterPointState(context.Background(), nil, export)
	if !errors.Is(err, ErrCharacterPointStateImportExecutorRequired) {
		t.Fatalf("ImportCharacterPointState(nil) error = %v, want %v", err, ErrCharacterPointStateImportExecutorRequired)
	}
}

func TestImportCharacterPointStateRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := CharacterPointStateExport{
		MigrationVersion: 99,
		MigrationName:    "not-point-state",
		Points:           []CharacterPointRow{},
	}

	_, err := ImportCharacterPointState(context.Background(), failingPointStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterPointStateExport) {
		t.Fatalf("ImportCharacterPointState(invalid) error = %v, want %v", err, ErrInvalidCharacterPointStateExport)
	}
}

func TestImportCharacterPointStateRejectsNilPointsBeforeOpeningTransaction(t *testing.T) {
	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           nil,
	}

	_, err := ImportCharacterPointState(context.Background(), failingPointStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterPointStateExport) {
		t.Fatalf("ImportCharacterPointState(nil points) error = %v, want %v", err, ErrInvalidCharacterPointStateExport)
	}
}

type failingPointStateImportExecutor struct{}

func (failingPointStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid point-state exports")
}
