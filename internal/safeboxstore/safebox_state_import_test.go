package safeboxstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

func TestImportCharacterSafeboxStateRejectsTooManyOptions(t *testing.T) {
	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}
	_, err := ImportCharacterSafeboxState(
		context.Background(),
		failingSafeboxStateImportExecutor{},
		export,
		ImportCharacterSafeboxStateOptions{Replace: true},
		ImportCharacterSafeboxStateOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportCharacterSafeboxState(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineCharacterSafeboxStateExportMergesDeclaredCharacterIDs(t *testing.T) {
	export := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		CharacterIDs:     []uint32{11},
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}
	canonical, summary, err := QuarantineCharacterSafeboxStateExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.CharacterCount != 1 || len(summary.CharacterIDs) != 1 || summary.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected declared wipe summary: %#v", summary)
	}
	if len(canonical.CharacterIDs) != 1 || canonical.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected canonical character_ids: %#v", canonical.CharacterIDs)
	}
	if summary.PasswordCount != 0 || summary.ItemCount != 0 {
		t.Fatalf("declared wipe should keep zero password/item counts: %#v", summary)
	}
}

func TestQuarantineCharacterSafeboxStateExportRejectsInvalidDeclaredCharacterIDs(t *testing.T) {
	base := CharacterSafeboxStateExport{
		MigrationVersion: CharacterSafeboxStateMigrationVersion,
		MigrationName:    CharacterSafeboxStateMigrationName,
		Passwords:        []CharacterSafeboxPasswordRow{},
		Items:            []CharacterSafeboxItemRow{},
	}

	zeroID := base
	zeroID.CharacterIDs = []uint32{0}
	if _, _, err := QuarantineCharacterSafeboxStateExport(zeroID); err == nil || !errors.Is(err, ErrInvalidCharacterSafeboxStateExport) {
		t.Fatalf("zero character_ids error = %v, want invalid export", err)
	}

	dupID := base
	dupID.CharacterIDs = []uint32{7, 7}
	if _, _, err := QuarantineCharacterSafeboxStateExport(dupID); err == nil || !errors.Is(err, ErrInvalidCharacterSafeboxStateExport) {
		t.Fatalf("duplicate character_ids error = %v, want invalid export", err)
	}
}

type failingSafeboxStateImportExecutor struct{}

func (failingSafeboxStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid safebox-state exports")
}
