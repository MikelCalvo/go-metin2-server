package safeboxstore

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestImportCharacterSafeboxStateRejectsNilExecutor(t *testing.T) {
	export, err := ExportCharacterSafeboxState(Snapshot{Characters: []CharacterRow{{
		Login:       "Alpha",
		CharacterID: 7,
		Password:    "secret",
		Money:       1500,
		Cells:       []Cell{{Cell: 0, ID: 1001, Vnum: 27001, Count: 2, Locked: true}},
	}}})
	if err != nil {
		t.Fatalf("export sample safebox state: %v", err)
	}

	_, err = ImportCharacterSafeboxState(context.Background(), nil, export)
	if !errors.Is(err, ErrCharacterSafeboxStateImportExecutorRequired) {
		t.Fatalf("ImportCharacterSafeboxState(nil) error = %v, want %v", err, ErrCharacterSafeboxStateImportExecutorRequired)
	}
}

func TestImportCharacterSafeboxStateRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := CharacterSafeboxStateExport{
		MigrationVersion: 99,
		MigrationName:    "not-safebox-state",
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}

	_, err := ImportCharacterSafeboxState(context.Background(), failingSafeboxStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterSafeboxStateExport) {
		t.Fatalf("ImportCharacterSafeboxState(invalid) error = %v, want %v", err, ErrInvalidCharacterSafeboxStateExport)
	}
}

func TestImportCharacterSafeboxStateRejectsNilPasswordsBeforeOpeningTransaction(t *testing.T) {
	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        nil,
		Items:            []CharacterSafeboxItemRow{},
	}

	_, err := ImportCharacterSafeboxState(context.Background(), failingSafeboxStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterSafeboxStateExport) {
		t.Fatalf("ImportCharacterSafeboxState(nil passwords) error = %v, want %v", err, ErrInvalidCharacterSafeboxStateExport)
	}
}

func TestImportCharacterSafeboxStateRejectsNilItemsBeforeOpeningTransaction(t *testing.T) {
	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            nil,
	}

	_, err := ImportCharacterSafeboxState(context.Background(), failingSafeboxStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterSafeboxStateExport) {
		t.Fatalf("ImportCharacterSafeboxState(nil items) error = %v, want %v", err, ErrInvalidCharacterSafeboxStateExport)
	}
}

type failingSafeboxStateImportExecutor struct{}

func (failingSafeboxStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid safebox-state exports")
}
