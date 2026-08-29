package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestExportCharacterMyShopUnitPricesBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	alphaWar := rosterExportCharacter(11, "AlphaWar")
	alphaWar.MyShopUnitPrices = []loginticket.MyShopUnitPrice{
		{Vnum: 27002, UnitPrice: 200},
		{Vnum: 27001, UnitPrice: 500},
	}

	bravoNinja := rosterExportCharacter(22, "BravoNinja")
	bravoNinja.MyShopUnitPrices = []loginticket.MyShopUnitPrice{
		{Vnum: 1, UnitPrice: 0},
	}

	emptyPrices := rosterExportCharacter(33, "CharlieEmpty")

	export, err := ExportCharacterMyShopUnitPrices([]Account{
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravoNinja}},
		{Login: "Charlie", Empire: 1, Characters: []loginticket.Character{emptyPrices}},
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
	})
	if err != nil {
		t.Fatalf("export character myshop unit prices: %v", err)
	}
	if export.MigrationVersion != CharacterMyShopUnitPricesMigrationVersion || export.MigrationName != CharacterMyShopUnitPricesMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	want := []CharacterMyShopUnitPriceRow{
		{CharacterID: 11, Vnum: 27001, UnitPrice: 500},
		{CharacterID: 11, Vnum: 27002, UnitPrice: 200},
		{CharacterID: 22, Vnum: 1, UnitPrice: 0},
	}
	if !reflect.DeepEqual(export.UnitPrices, want) {
		t.Fatalf("unexpected unit price rows:\n got: %#v\nwant: %#v", export.UnitPrices, want)
	}

	exportAgain, err := ExportCharacterMyShopUnitPrices([]Account{
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravoNinja}},
		{Login: "Charlie", Empire: 1, Characters: []loginticket.Character{emptyPrices}},
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
	})
	if err != nil {
		t.Fatalf("export character myshop unit prices again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic character myshop unit-prices export:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportCharacterMyShopUnitPricesRejectsMalformedRows(t *testing.T) {
	duplicate := rosterExportCharacter(11, "DupWar")
	duplicate.MyShopUnitPrices = []loginticket.MyShopUnitPrice{
		{Vnum: 27001, UnitPrice: 1},
		{Vnum: 27001, UnitPrice: 2},
	}
	_, err := ExportCharacterMyShopUnitPrices([]Account{{Login: "Alpha", Characters: []loginticket.Character{duplicate}}})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate vnum, got %v", err)
	}
}

func TestFileStoreExportCharacterMyShopUnitPricesReadsCommittedSnapshots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	character := rosterExportCharacter(200, "BetaWar")
	character.MyShopUnitPrices = []loginticket.MyShopUnitPrice{
		{Vnum: 27002, UnitPrice: 200},
		{Vnum: 27001, UnitPrice: 500},
	}
	if err := store.Save(Account{Login: "Beta", Empire: 2, Characters: []loginticket.Character{character}}); err != nil {
		t.Fatalf("save beta account: %v", err)
	}
	if err := store.Save(Account{Login: "Alpha", Empire: 1}); err != nil {
		t.Fatalf("save alpha account: %v", err)
	}

	export, err := store.ExportCharacterMyShopUnitPrices()
	if err != nil {
		t.Fatalf("file-store character myshop unit-prices export: %v", err)
	}
	want := []CharacterMyShopUnitPriceRow{
		{CharacterID: 200, Vnum: 27001, UnitPrice: 500},
		{CharacterID: 200, Vnum: 27002, UnitPrice: 200},
	}
	if !reflect.DeepEqual(export.UnitPrices, want) {
		t.Fatalf("unexpected file-store unit price rows:\n got: %#v\nwant: %#v", export.UnitPrices, want)
	}
}
