package worldruntime

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

func TestImportBootstrapGroundItemStateRejectsTooManyOptions(t *testing.T) {
	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems:      []BootstrapGroundItemStateRow{},
	}
	_, err := ImportBootstrapGroundItemState(
		context.Background(),
		failingGroundItemStateImportExecutor{},
		export,
		ImportBootstrapGroundItemStateOptions{Replace: true},
		ImportBootstrapGroundItemStateOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportBootstrapGroundItemState(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineBootstrapGroundItemStateExportMergesDeclaredVIDs(t *testing.T) {
	export := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		VIDs:             []uint32{0x0700002c},
		GroundItems:      []BootstrapGroundItemStateRow{},
	}
	canonical, summary, err := QuarantineBootstrapGroundItemStateExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.GroundItemCount != 0 || summary.ItemShapedCount != 0 || summary.GoldShapedCount != 0 {
		t.Fatalf("declared wipe should keep zero ground counts: %#v", summary)
	}
	if len(summary.VIDs) != 1 || summary.VIDs[0] != 0x0700002c {
		t.Fatalf("unexpected declared wipe summary vids: %#v", summary)
	}
	if len(canonical.VIDs) != 1 || canonical.VIDs[0] != 0x0700002c {
		t.Fatalf("unexpected canonical vids: %#v", canonical.VIDs)
	}
}

func TestQuarantineBootstrapGroundItemStateExportRejectsInvalidDeclaredVIDs(t *testing.T) {
	base := BootstrapGroundItemStateExport{
		MigrationVersion: BootstrapGroundItemStateMigrationVersion,
		MigrationName:    BootstrapGroundItemStateMigrationName,
		GroundItems:      []BootstrapGroundItemStateRow{},
	}

	zeroVID := base
	zeroVID.VIDs = []uint32{0}
	if _, _, err := QuarantineBootstrapGroundItemStateExport(zeroVID); err == nil || !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
		t.Fatalf("zero vids error = %v, want invalid export", err)
	}

	dupVID := base
	dupVID.VIDs = []uint32{0x0700002c, 0x0700002c}
	if _, _, err := QuarantineBootstrapGroundItemStateExport(dupVID); err == nil || !errors.Is(err, ErrInvalidBootstrapGroundItemStateExport) {
		t.Fatalf("duplicate vids error = %v, want invalid export", err)
	}
}

type failingGroundItemStateImportExecutor struct{}

func (failingGroundItemStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid ground-item-state exports")
}
