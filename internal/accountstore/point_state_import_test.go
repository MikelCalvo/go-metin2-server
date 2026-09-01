package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

func TestImportCharacterPointStateRejectsTooManyOptions(t *testing.T) {
	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           []CharacterPointRow{},
	}
	_, err := ImportCharacterPointState(
		context.Background(),
		failingPointStateImportExecutor{},
		export,
		ImportCharacterPointStateOptions{Replace: true},
		ImportCharacterPointStateOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportCharacterPointState(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineCharacterPointStateExportMergesDeclaredCharacterIDs(t *testing.T) {
	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		CharacterIDs:     []uint32{11},
		Points:           []CharacterPointRow{},
	}
	canonical, summary, err := QuarantineCharacterPointStateExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.CharacterCount != 1 || len(summary.CharacterIDs) != 1 || summary.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected declared wipe summary: %#v", summary)
	}
	if len(canonical.CharacterIDs) != 1 || canonical.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected canonical character_ids: %#v", canonical.CharacterIDs)
	}
	if summary.PointRowCount != 0 {
		t.Fatalf("declared wipe should keep zero point counts: %#v", summary)
	}
}

func TestQuarantineCharacterPointStateExportRejectsInvalidDeclaredCharacterIDs(t *testing.T) {
	base := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           []CharacterPointRow{},
	}

	zeroID := base
	zeroID.CharacterIDs = []uint32{0}
	if _, _, err := QuarantineCharacterPointStateExport(zeroID); err == nil || !errors.Is(err, ErrInvalidCharacterPointStateExport) {
		t.Fatalf("zero character_ids error = %v, want invalid export", err)
	}

	dupID := base
	dupID.CharacterIDs = []uint32{7, 7}
	if _, _, err := QuarantineCharacterPointStateExport(dupID); err == nil || !errors.Is(err, ErrInvalidCharacterPointStateExport) {
		t.Fatalf("duplicate character_ids error = %v, want invalid export", err)
	}
}

type failingPointStateImportExecutor struct{}

func (failingPointStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid point-state exports")
}
