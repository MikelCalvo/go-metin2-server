package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestImportCharacterItemStateRejectsNilExecutor(t *testing.T) {
	export, err := ExportCharacterItemState([]Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				func() loginticket.Character {
					character := rosterExportCharacter(11, "AlphaWar")
					character.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 5}}
					return character
				}(),
			},
		},
	})
	if err != nil {
		t.Fatalf("export sample item state: %v", err)
	}

	_, err = ImportCharacterItemState(context.Background(), nil, export)
	if !errors.Is(err, ErrCharacterItemStateImportExecutorRequired) {
		t.Fatalf("ImportCharacterItemState(nil) error = %v, want %v", err, ErrCharacterItemStateImportExecutorRequired)
	}
}

func TestImportCharacterItemStateRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := CharacterItemStateExport{
		MigrationVersion: 99,
		MigrationName:    "not-item-state",
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}

	_, err := ImportCharacterItemState(context.Background(), failingItemStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterItemStateExport) {
		t.Fatalf("ImportCharacterItemState(invalid) error = %v, want %v", err, ErrInvalidCharacterItemStateExport)
	}
}

func TestImportCharacterItemStateRejectsNilSlicesBeforeOpeningTransaction(t *testing.T) {
	export := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems:   nil,
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}

	_, err := ImportCharacterItemState(context.Background(), failingItemStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidCharacterItemStateExport) {
		t.Fatalf("ImportCharacterItemState(nil inventory) error = %v, want %v", err, ErrInvalidCharacterItemStateExport)
	}
}

type failingItemStateImportExecutor struct{}

func (failingItemStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid item-state exports")
}
