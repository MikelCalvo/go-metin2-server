package worldruntime

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestImportBootstrapGroundItemStateRejectsNilExecutor(t *testing.T) {
	count := uint16(2)
	export, err := ExportBootstrapGroundItemState([]GroundItemSnapshot{{
		VID: 0x0700002c, Vnum: 3001, Count: count,
		OwnerName: "GroundItemOwner", OwnerLogin: "ground-item-owner",
		OwnerCharacterID: 0x0103019c, OwnerVID: 0x0204019c,
		PickupRange: 450, MapIndex: 1, X: 1100, Y: 2100, Z: 2,
	}})
	if err != nil {
		t.Fatalf("export sample ground-item state: %v", err)
	}

	_, err = ImportBootstrapGroundItemState(context.Background(), nil, export)
	if !errors.Is(err, ErrBootstrapGroundItemStateImportExecutorRequired) {
		t.Fatalf("ImportBootstrapGroundItemState(nil) error = %v, want %v", err, ErrBootstrapGroundItemStateImportExecutorRequired)
	}
}

func TestImportBootstrapGroundItemStateRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := BootstrapGroundItemStateExport{
		MigrationVersion: 99,
		MigrationName:    "not-ground-item-state",
		GroundItems:      []BootstrapGroundItemStateRow{},
	}

	_, err := ImportBootstrapGroundItemState(context.Background(), failingGroundItemStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
		t.Fatalf("ImportBootstrapGroundItemState(invalid) error = %v, want %v", err, ErrInvalidBootstrapGroundItemStateExport)
	}
}

func TestImportBootstrapGroundItemStateRejectsNilGroundItemsBeforeOpeningTransaction(t *testing.T) {
	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems:      nil,
	}

	_, err := ImportBootstrapGroundItemState(context.Background(), failingGroundItemStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
		t.Fatalf("ImportBootstrapGroundItemState(nil ground_items) error = %v, want %v", err, ErrInvalidBootstrapGroundItemStateExport)
	}
}

type failingGroundItemStateImportExecutor struct{}

func (failingGroundItemStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid ground-item-state exports")
}
