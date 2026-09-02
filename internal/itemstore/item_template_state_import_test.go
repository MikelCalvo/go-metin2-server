package itemstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

func TestImportItemTemplateStateRejectsTooManyOptions(t *testing.T) {
	export := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates:        []ItemTemplateRow{},
		Sockets:          []ItemTemplateSocketRow{},
		Attributes:       []ItemTemplateAttributeRow{},
		UseEffects:       []ItemTemplateUseEffectRow{},
		EquipEffects:     []ItemTemplateEquipEffectRow{},
		RefineInfos:      []ItemTemplateRefineInfoRow{},
		RefineMaterials:  []ItemTemplateRefineMaterialRow{},
	}
	_, err := ImportItemTemplateState(
		context.Background(),
		failingItemTemplateStateImportExecutor{},
		export,
		ImportItemTemplateStateOptions{Replace: true},
		ImportItemTemplateStateOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportItemTemplateState(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineItemTemplateStateExportMergesDeclaredVnums(t *testing.T) {
	export := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Vnums:            []uint32{27001},
		Templates:        []ItemTemplateRow{},
		Sockets:          []ItemTemplateSocketRow{},
		Attributes:       []ItemTemplateAttributeRow{},
		UseEffects:       []ItemTemplateUseEffectRow{},
		EquipEffects:     []ItemTemplateEquipEffectRow{},
		RefineInfos:      []ItemTemplateRefineInfoRow{},
		RefineMaterials:  []ItemTemplateRefineMaterialRow{},
	}
	canonical, summary, err := QuarantineItemTemplateStateExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.TemplateCount != 0 || summary.SocketCount != 0 || summary.AttributeCount != 0 ||
		summary.UseEffectCount != 0 || summary.EquipEffectCount != 0 ||
		summary.RefineInfoCount != 0 || summary.RefineMaterialCount != 0 {
		t.Fatalf("declared wipe should keep zero template counts: %#v", summary)
	}
	if len(summary.Vnums) != 1 || summary.Vnums[0] != 27001 {
		t.Fatalf("unexpected declared wipe summary vnums: %#v", summary)
	}
	if len(canonical.Vnums) != 1 || canonical.Vnums[0] != 27001 {
		t.Fatalf("unexpected canonical vnums: %#v", canonical.Vnums)
	}
}

func TestQuarantineItemTemplateStateExportRejectsInvalidDeclaredVnums(t *testing.T) {
	base := ItemTemplateStateExport{
		MigrationVersion: ItemTemplateStateMigrationVersion,
		MigrationName:    ItemTemplateStateMigrationName,
		Templates:        []ItemTemplateRow{},
		Sockets:          []ItemTemplateSocketRow{},
		Attributes:       []ItemTemplateAttributeRow{},
		UseEffects:       []ItemTemplateUseEffectRow{},
		EquipEffects:     []ItemTemplateEquipEffectRow{},
		RefineInfos:      []ItemTemplateRefineInfoRow{},
		RefineMaterials:  []ItemTemplateRefineMaterialRow{},
	}

	zeroVnum := base
	zeroVnum.Vnums = []uint32{0}
	if _, _, err := QuarantineItemTemplateStateExport(zeroVnum); err == nil || !errors.Is(err, ErrInvalidItemTemplateStateExport) {
		t.Fatalf("zero vnums error = %v, want invalid export", err)
	}

	dupVnum := base
	dupVnum.Vnums = []uint32{27001, 27001}
	if _, _, err := QuarantineItemTemplateStateExport(dupVnum); err == nil || !errors.Is(err, ErrInvalidItemTemplateStateExport) {
		t.Fatalf("duplicate vnums error = %v, want invalid export", err)
	}
}
