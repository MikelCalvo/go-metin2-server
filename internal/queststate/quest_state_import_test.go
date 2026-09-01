package queststate

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestImportCharacterQuestStateRejectsNilExecutor(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags: []CharacterQuestFlagRow{
			{CharacterID: 11, Character: "AlphaWar", QuestRef: "quest:first_steps", Flag: "met_guide", Value: 1},
		},
	}

	_, err := ImportCharacterQuestState(context.Background(), nil, export)
	if !errors.Is(err, ErrCharacterQuestStateImportExecutorRequired) {
		t.Fatalf("ImportCharacterQuestState(nil) error = %v, want %v", err, ErrCharacterQuestStateImportExecutorRequired)
	}
}

func TestImportCharacterQuestStateRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: 99,
		MigrationName:    "not-quest-state",
		Flags:            []CharacterQuestFlagRow{},
	}

	_, err := ImportCharacterQuestState(context.Background(), failingQuestStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterQuestStateExport) {
		t.Fatalf("ImportCharacterQuestState(invalid) error = %v, want %v", err, ErrInvalidCharacterQuestStateExport)
	}
}

func TestImportCharacterQuestStateRejectsNilFlagsBeforeOpeningTransaction(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            nil,
	}

	_, err := ImportCharacterQuestState(context.Background(), failingQuestStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterQuestStateExport) {
		t.Fatalf("ImportCharacterQuestState(nil flags) error = %v, want %v", err, ErrInvalidCharacterQuestStateExport)
	}
}

func TestImportCharacterQuestStateRejectsTooManyOptions(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            []CharacterQuestFlagRow{},
	}
	_, err := ImportCharacterQuestState(
		context.Background(),
		failingQuestStateImportExecutor{},
		export,
		ImportCharacterQuestStateOptions{Replace: true},
		ImportCharacterQuestStateOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportCharacterQuestState(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineCharacterQuestStateExportMergesDeclaredCharacterIDs(t *testing.T) {
	export := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		CharacterIDs:     []uint32{11},
		Flags:            []CharacterQuestFlagRow{},
	}
	canonical, summary, err := QuarantineCharacterQuestStateExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.CharacterCount != 1 || len(summary.CharacterIDs) != 1 || summary.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected declared wipe summary: %#v", summary)
	}
	if len(canonical.CharacterIDs) != 1 || canonical.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected canonical character_ids: %#v", canonical.CharacterIDs)
	}
	if summary.FlagCount != 0 {
		t.Fatalf("declared wipe should keep zero flag counts: %#v", summary)
	}
}

func TestQuarantineCharacterQuestStateExportRejectsInvalidDeclaredCharacterIDs(t *testing.T) {
	base := CharacterQuestStateExport{
		MigrationVersion: CharacterQuestStateMigrationVersion,
		MigrationName:    CharacterQuestStateMigrationName,
		Flags:            []CharacterQuestFlagRow{},
	}

	zeroID := base
	zeroID.CharacterIDs = []uint32{0}
	if _, _, err := QuarantineCharacterQuestStateExport(zeroID); err == nil || !errors.Is(err, ErrInvalidCharacterQuestStateExport) {
		t.Fatalf("zero character_ids error = %v, want invalid export", err)
	}

	dupID := base
	dupID.CharacterIDs = []uint32{7, 7}
	if _, _, err := QuarantineCharacterQuestStateExport(dupID); err == nil || !errors.Is(err, ErrInvalidCharacterQuestStateExport) {
		t.Fatalf("duplicate character_ids error = %v, want invalid export", err)
	}
}

type failingQuestStateImportExecutor struct{}

func (failingQuestStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid quest-state exports")
}
