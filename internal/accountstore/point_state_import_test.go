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

func TestCharacterPointStateProjectedHPKeepsAboveFloorDeathFloorAndRestartMaxHPOnTip0011(t *testing.T) {
	cases := []struct {
		name string
		hp   int32
	}{
		{name: "above-floor partial HP", hp: 748},
		{name: "death-floor zero", hp: 0},
		{name: "restart MaxHP", hp: 750},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			character := rosterExportCharacter(11, "AlphaWar")
			character.Points[CharacterPointStateHPIndex] = tc.hp
			export, err := ExportCharacterPointState([]Account{
				{
					Login:      "Alpha",
					Empire:     1,
					Characters: []loginticket.Character{character},
				},
			})
			if err != nil {
				t.Fatalf("export character point state: %v", err)
			}
			if export.MigrationVersion != CharacterPointStateMigrationVersion || export.MigrationName != CharacterPointStateMigrationName {
				t.Fatalf("invented point-state tip identity: version=%d name=%q", export.MigrationVersion, export.MigrationName)
			}

			got, present, err := CharacterPointStateProjectedHP(export, 11)
			if err != nil {
				t.Fatalf("CharacterPointStateProjectedHP: %v", err)
			}
			if !present || got != tc.hp {
				t.Fatalf("projected HP present=%v value=%d, want present value %d", present, got, tc.hp)
			}

			canonical, summary, err := QuarantineCharacterPointStateExport(export)
			if err != nil {
				t.Fatalf("quarantine character point-state export: %v", err)
			}
			if canonical.MigrationVersion != CharacterPointStateMigrationVersion || canonical.MigrationName != CharacterPointStateMigrationName {
				t.Fatalf("quarantine invented tip identity: version=%d name=%q", canonical.MigrationVersion, canonical.MigrationName)
			}
			if summary.CharacterCount != 1 || summary.PointRowCount != characterPointStatePointCount {
				t.Fatalf("unexpected quarantine summary: %#v", summary)
			}
			if int(CharacterPointStateHPIndex) >= len(canonical.Points) {
				t.Fatalf("canonical export missing HP slot: %d points", len(canonical.Points))
			}
			hpRow := canonical.Points[CharacterPointStateHPIndex]
			if hpRow.CharacterID != 11 || hpRow.PointIndex != CharacterPointStateHPIndex || hpRow.Value != tc.hp {
				t.Fatalf("quarantine HP row = %#v, want character 11 index %d value %d", hpRow, CharacterPointStateHPIndex, tc.hp)
			}
		})
	}
}

func TestCharacterPointStateProjectedHPReportsAbsentHPForDeclaredWipe(t *testing.T) {
	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		CharacterIDs:     []uint32{11},
		Points:           []CharacterPointRow{},
	}
	got, present, err := CharacterPointStateProjectedHP(export, 11)
	if err != nil {
		t.Fatalf("CharacterPointStateProjectedHP(wipe): %v", err)
	}
	if present || got != 0 {
		t.Fatalf("wipe HP present=%v value=%d, want absent", present, got)
	}
}

func TestCharacterPointStateProjectedHPRejectsInvalidExport(t *testing.T) {
	_, _, err := CharacterPointStateProjectedHP(CharacterPointStateExport{
		MigrationVersion: 99,
		MigrationName:    "not-point-state",
		Points:           []CharacterPointRow{},
	}, 11)
	if !errors.Is(err, ErrInvalidCharacterPointStateExport) {
		t.Fatalf("CharacterPointStateProjectedHP(invalid) error = %v, want %v", err, ErrInvalidCharacterPointStateExport)
	}

	character := rosterExportCharacter(11, "AlphaWar")
	character.Points[CharacterPointStateHPIndex] = 748
	export, err := ExportCharacterPointState([]Account{
		{
			Login:      "Alpha",
			Empire:     1,
			Characters: []loginticket.Character{character},
		},
	})
	if err != nil {
		t.Fatalf("export character point state: %v", err)
	}
	_, _, err = CharacterPointStateProjectedHP(export, 0)
	if !errors.Is(err, ErrInvalidCharacterPointStateExport) {
		t.Fatalf("CharacterPointStateProjectedHP(zero id) error = %v, want %v", err, ErrInvalidCharacterPointStateExport)
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
