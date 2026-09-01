package accountstore

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestImportCharacterMyShopUnitPricesRequiresExecutor(t *testing.T) {
	_, err := ImportCharacterMyShopUnitPrices(context.Background(), nil, CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	})
	if !errors.Is(err, ErrCharacterMyShopUnitPricesImportExecutorRequired) {
		t.Fatalf("expected ErrCharacterMyShopUnitPricesImportExecutorRequired, got %v", err)
	}
}

func TestImportCharacterMyShopUnitPricesRejectsInvalidExportBeforeOpening(t *testing.T) {
	_, err := ImportCharacterMyShopUnitPrices(context.Background(), failingPointStateImportExecutor{}, CharacterMyShopUnitPricesExport{
		MigrationVersion: 2,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	})
	if !errors.Is(err, ErrInvalidCharacterMyShopUnitPricesExport) {
		t.Fatalf("expected ErrInvalidCharacterMyShopUnitPricesExport, got %v", err)
	}
}

func TestImportCharacterMyShopUnitPricesRejectsTooManyOptions(t *testing.T) {
	export := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	}
	_, err := ImportCharacterMyShopUnitPrices(
		context.Background(),
		failingPointStateImportExecutor{},
		export,
		ImportCharacterMyShopUnitPricesOptions{Replace: true},
		ImportCharacterMyShopUnitPricesOptions{},
	)
	if err == nil || !strings.Contains(err.Error(), "at most one options") {
		t.Fatalf("ImportCharacterMyShopUnitPrices(too many options) error = %v, want at most one options", err)
	}
}

func TestQuarantineCharacterMyShopUnitPricesExportMergesDeclaredCharacterIDs(t *testing.T) {
	export := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		CharacterIDs:     []uint32{11},
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	}
	canonical, summary, err := QuarantineCharacterMyShopUnitPricesExport(export)
	if err != nil {
		t.Fatalf("quarantine declared wipe export: %v", err)
	}
	if summary.CharacterCount != 1 || len(summary.CharacterIDs) != 1 || summary.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected declared wipe summary: %#v", summary)
	}
	if len(canonical.CharacterIDs) != 1 || canonical.CharacterIDs[0] != 11 {
		t.Fatalf("unexpected canonical character_ids: %#v", canonical.CharacterIDs)
	}
	if summary.PriceRowCount != 0 {
		t.Fatalf("declared wipe should keep zero price counts: %#v", summary)
	}
}

func TestQuarantineCharacterMyShopUnitPricesExportRejectsInvalidDeclaredCharacterIDs(t *testing.T) {
	base := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	}

	zeroID := base
	zeroID.CharacterIDs = []uint32{0}
	if _, _, err := QuarantineCharacterMyShopUnitPricesExport(zeroID); err == nil || !errors.Is(err, ErrInvalidCharacterMyShopUnitPricesExport) {
		t.Fatalf("zero character_ids error = %v, want invalid export", err)
	}

	dupID := base
	dupID.CharacterIDs = []uint32{7, 7}
	if _, _, err := QuarantineCharacterMyShopUnitPricesExport(dupID); err == nil || !errors.Is(err, ErrInvalidCharacterMyShopUnitPricesExport) {
		t.Fatalf("duplicate character_ids error = %v, want invalid export", err)
	}
}
