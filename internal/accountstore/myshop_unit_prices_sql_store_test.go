package accountstore

import (
	"errors"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLMyShopUnitPricesStoreRejectsNilExecutor(t *testing.T) {
	if _, err := (*SQLMyShopUnitPricesStore)(nil).List(); !errors.Is(err, ErrCharacterMyShopUnitPricesImportExecutorRequired) {
		t.Fatalf("nil List: %v", err)
	}
	if err := NewSQLMyShopUnitPricesStore(nil).Save(Account{Login: "Alpha"}); !errors.Is(err, ErrCharacterMyShopUnitPricesImportExecutorRequired) {
		t.Fatalf("nil Save: %v", err)
	}
	if _, err := NewSQLMyShopUnitPricesStore(nil).ExportCharacterMyShopUnitPrices(); !errors.Is(err, ErrCharacterMyShopUnitPricesImportExecutorRequired) {
		t.Fatalf("nil Export: %v", err)
	}
}

func TestSQLMyShopUnitPricesStoreRejectsInvalidPriceBeforeOpening(t *testing.T) {
	store := NewSQLMyShopUnitPricesStore(failingPointStateImportExecutor{})
	character := rosterExportCharacter(11, "AlphaWar")
	character.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 0, UnitPrice: 42}}
	if err := store.Save(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}); err == nil {
		t.Fatal("Save invalid vnum succeeded")
	}
	if _, err := store.Load(" Alpha "); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("Load padded login: %v", err)
	}
}
