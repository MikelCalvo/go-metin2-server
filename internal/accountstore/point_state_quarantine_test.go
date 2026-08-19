package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestValidateCharacterPointStateExportAcceptsCanonicalExport(t *testing.T) {
	character := rosterExportCharacter(11, "AlphaWar")
	character.Points[0] = 12
	character.Points[1] = -3
	character.Points[254] = 99

	export, err := ExportCharacterPointState([]Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}},
	})
	if err != nil {
		t.Fatalf("seed export: %v", err)
	}

	summary, err := ValidateCharacterPointStateExport(export)
	if err != nil {
		t.Fatalf("validate character point-state export: %v", err)
	}
	want := CharacterPointStateQuarantineSummary{
		CharacterCount: 1,
		PointRowCount:  255,
		CharacterIDs:   []uint32{11},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateCharacterPointStateExportRejectsWrongMigrationBoundary(t *testing.T) {
	export := CharacterPointStateExport{
		MigrationVersion: 3,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           []CharacterPointRow{},
	}
	if _, err := ValidateCharacterPointStateExport(export); !errors.Is(err, ErrInvalidCharacterPointStateExport) {
		t.Fatalf("expected ErrInvalidCharacterPointStateExport, got %v", err)
	}
}

func TestValidateCharacterPointStateExportRejectsSparseOrDuplicateVectors(t *testing.T) {
	cases := []struct {
		name   string
		export CharacterPointStateExport
	}{
		{
			name: "missing indices",
			export: CharacterPointStateExport{
				MigrationVersion: CharacterPointStateMigrationVersion,
				MigrationName:    CharacterPointStateMigrationName,
				Points: []CharacterPointRow{
					{CharacterID: 11, PointIndex: 0, Value: 1},
					{CharacterID: 11, PointIndex: 2, Value: 2},
				},
			},
		},
		{
			name: "duplicate index",
			export: CharacterPointStateExport{
				MigrationVersion: CharacterPointStateMigrationVersion,
				MigrationName:    CharacterPointStateMigrationName,
				Points: append(
					fullPointVector(11, 0),
					CharacterPointRow{CharacterID: 11, PointIndex: 0, Value: 9},
				),
			},
		},
		{
			name: "zero character id",
			export: CharacterPointStateExport{
				MigrationVersion: CharacterPointStateMigrationVersion,
				MigrationName:    CharacterPointStateMigrationName,
				Points:           fullPointVector(0, 0),
			},
		},
		{
			name: "nil points",
			export: CharacterPointStateExport{
				MigrationVersion: CharacterPointStateMigrationVersion,
				MigrationName:    CharacterPointStateMigrationName,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateCharacterPointStateExport(tc.export); !errors.Is(err, ErrInvalidCharacterPointStateExport) {
				t.Fatalf("expected ErrInvalidCharacterPointStateExport, got %v", err)
			}
		})
	}
}

func TestQuarantineCharacterPointStateExportCanonicalizesRowOrder(t *testing.T) {
	export := CharacterPointStateExport{
		MigrationVersion: CharacterPointStateMigrationVersion,
		MigrationName:    CharacterPointStateMigrationName,
		Points:           append(fullPointVector(22, 1), fullPointVector(11, 2)...),
	}

	quarantined, summary, err := QuarantineCharacterPointStateExport(export)
	if err != nil {
		t.Fatalf("quarantine character point-state export: %v", err)
	}
	wantSummary := CharacterPointStateQuarantineSummary{
		CharacterCount: 2,
		PointRowCount:  510,
		CharacterIDs:   []uint32{11, 22},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}
	if quarantined.Points[0].CharacterID != 11 || quarantined.Points[0].PointIndex != 0 {
		t.Fatalf("expected canonicalization to place character 11 first, got %#v", quarantined.Points[0])
	}
	if quarantined.Points[255].CharacterID != 22 || quarantined.Points[255].PointIndex != 0 {
		t.Fatalf("expected character 22 after character 11, got %#v", quarantined.Points[255])
	}
	if quarantined.Points[0].Value != 2 || quarantined.Points[255].Value != 1 {
		t.Fatalf("expected per-character fill values preserved, got first=%d second=%d", quarantined.Points[0].Value, quarantined.Points[255].Value)
	}
}

func fullPointVector(characterID uint32, fill int32) []CharacterPointRow {
	rows := make([]CharacterPointRow, 0, characterPointStatePointCount)
	for index := 0; index < characterPointStatePointCount; index++ {
		rows = append(rows, CharacterPointRow{
			CharacterID: characterID,
			PointIndex:  uint8(index),
			Value:       fill,
		})
	}
	return rows
}
