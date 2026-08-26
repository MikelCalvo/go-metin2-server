package itemstore

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestImportItemTemplateStateRejectsNilExecutor(t *testing.T) {
	export, err := ExportItemTemplateState(Snapshot{Templates: []Template{{
		Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200,
	}}})
	if err != nil {
		t.Fatalf("export sample item-template state: %v", err)
	}

	_, err = ImportItemTemplateState(context.Background(), nil, export)
	if !errors.Is(err, ErrItemTemplateStateImportExecutorRequired) {
		t.Fatalf("ImportItemTemplateState(nil) error = %v, want %v", err, ErrItemTemplateStateImportExecutorRequired)
	}
}

func TestImportItemTemplateStateRejectsInvalidExportBeforeOpeningTransaction(t *testing.T) {
	export := ItemTemplateStateExport{
		MigrationVersion: 99,
		MigrationName:    "not-item-template-state",
		Templates:        []ItemTemplateRow{},
		Sockets:          []ItemTemplateSocketRow{},
		Attributes:       []ItemTemplateAttributeRow{},
		UseEffects:       []ItemTemplateUseEffectRow{},
		EquipEffects:     []ItemTemplateEquipEffectRow{},
		RefineInfos:      []ItemTemplateRefineInfoRow{},
		RefineMaterials:  []ItemTemplateRefineMaterialRow{},
	}

	_, err := ImportItemTemplateState(context.Background(), failingItemTemplateStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidItemTemplateStateExport) {
		t.Fatalf("ImportItemTemplateState(invalid) error = %v, want %v", err, ErrInvalidItemTemplateStateExport)
	}
}

func TestImportItemTemplateStateRejectsNilTemplatesBeforeOpeningTransaction(t *testing.T) {
	export := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates:        nil,
		Sockets:          []ItemTemplateSocketRow{},
		Attributes:       []ItemTemplateAttributeRow{},
		UseEffects:       []ItemTemplateUseEffectRow{},
		EquipEffects:     []ItemTemplateEquipEffectRow{},
		RefineInfos:      []ItemTemplateRefineInfoRow{},
		RefineMaterials:  []ItemTemplateRefineMaterialRow{},
	}

	_, err := ImportItemTemplateState(context.Background(), failingItemTemplateStateImportExecutor{}, export)
	if !errors.Is(err, ErrInvalidItemTemplateStateExport) {
		t.Fatalf("ImportItemTemplateState(nil templates) error = %v, want %v", err, ErrInvalidItemTemplateStateExport)
	}
}

type failingItemTemplateStateImportExecutor struct{}

func (failingItemTemplateStateImportExecutor) BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error) {
	panic("BeginTx must not be reached for invalid item-template-state exports")
}
