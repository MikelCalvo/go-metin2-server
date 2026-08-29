package accountstore

import (
	"context"
	"errors"
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
