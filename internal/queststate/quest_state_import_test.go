package queststate

import (
	"context"
	"database/sql"
	"errors"
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

type failingQuestStateImportExecutor struct{}

func (failingQuestStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid quest-state exports")
}
