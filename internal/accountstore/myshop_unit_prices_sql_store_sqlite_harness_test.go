//go:build sqlite_harness

package accountstore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestSQLiteHarnessSQLMyShopUnitPricesStoreRoundTripAndScopedWipe(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()
	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatal(err)
	}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 27002, UnitPrice: 0}, {Vnum: 27001, UnitPrice: 500}}
	bravo := rosterExportCharacter(22, "BravoWar")
	bravo.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 1, UnitPrice: 7}}
	seed := []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}, {Login: "Bravo", Empire: 2, Characters: []loginticket.Character{bravo}}, {Login: "Empty", Empire: 1}}
	seedItemStateRoster(t, db, seed)
	store := NewSQLMyShopUnitPricesStore(db)
	if err := store.SaveAccounts(seed); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}
	assertMyShopUnitPriceRow(t, db, 11, 27001, 500)
	assertMyShopUnitPriceRow(t, db, 11, 27002, 0)
	assertMyShopUnitPriceRow(t, db, 22, 1, 7)
	got, err := store.Load("alpha")
	if err != nil || got.Login != "Alpha" || !reflect.DeepEqual(got.Characters[0].MyShopUnitPrices, loginticket.CanonicalMyShopUnitPrices(alpha.MyShopUnitPrices)) {
		t.Fatalf("Load(alpha) prices=%v err=%v", got.Characters, err)
	}
	listed, err := store.List()
	if err != nil || len(listed) != 3 || listed[2].Login != "Empty" || len(listed[2].Characters) != 4 {
		t.Fatalf("List: %v, %v", listed, err)
	}
	export, err := store.ExportCharacterMyShopUnitPrices()
	if err != nil || export.MigrationVersion != 23 || len(export.UnitPrices) != 3 {
		t.Fatalf("Export: %+v, %v", export, err)
	}
	alpha.MyShopUnitPrices = nil
	if err := store.Save(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}); err != nil {
		t.Fatalf("Save wipe: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_myshop_unit_prices WHERE character_id = 11`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("Alpha after wipe count=%d err=%v", count, err)
	}
	assertMyShopUnitPriceRow(t, db, 22, 1, 7)
	if err := store.SaveAccounts(nil); err != nil {
		t.Fatalf("SaveAccounts nil: %v", err)
	}
	if _, err := store.Load("Missing"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("Load Missing: %v", err)
	}
}

func TestSQLiteHarnessSQLMyShopUnitPricesStoreRejectsWrongOwnerAndRollsBack(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatal(err)
	}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 1, UnitPrice: 5}}
	bravo := rosterExportCharacter(22, "BravoWar")
	bravo.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 1, UnitPrice: 7}}
	seedItemStateRoster(t, db, []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}, {Login: "Bravo", Empire: 2, Characters: []loginticket.Character{bravo}}})
	store := NewSQLMyShopUnitPricesStore(db)
	if err := store.SaveAccounts([]Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}, {Login: "Bravo", Empire: 2, Characters: []loginticket.Character{bravo}}}); err != nil {
		t.Fatal(err)
	}
	alpha.MyShopUnitPrices[0].UnitPrice = 900
	bravo.MyShopUnitPrices[0].UnitPrice = 800
	err := store.SaveAccounts([]Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}, {Login: "Impostor", Empire: 2, Characters: []loginticket.Character{bravo}}})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("wrong owner: %v", err)
	}
	assertMyShopUnitPriceRow(t, db, 11, 1, 5)
	assertMyShopUnitPriceRow(t, db, 22, 1, 7)
	// Even a valid account/character ID pair cannot target a different slot.
	bravo.MyShopUnitPrices[0].UnitPrice = 999
	err = store.Save(Account{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravo}})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("wrong slot: %v", err)
	}
	assertMyShopUnitPriceRow(t, db, 22, 1, 7)
}

func TestSQLiteHarnessSQLMyShopUnitPricesStoreRequiresSchemaAndRejectsMissingParent(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()
	store := NewSQLMyShopUnitPricesStore(db)
	if _, err := store.List(); !errors.Is(err, ErrCharacterMyShopUnitPricesImportSchemaRequired) {
		t.Fatalf("List missing schema: %v", err)
	}
	if err := store.SaveAccounts(nil); !errors.Is(err, ErrCharacterMyShopUnitPricesImportSchemaRequired) {
		t.Fatalf("Save missing schema: %v", err)
	}
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatal(err)
	}
	character := rosterExportCharacter(11, "AlphaWar")
	character.MyShopUnitPrices = []loginticket.MyShopUnitPrice{{Vnum: 1, UnitPrice: 5}}
	if err := store.Save(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("missing roster parent: %v", err)
	}
}

func TestSQLiteHarnessSQLMyShopUnitPricesStoreLoadRejectsOverLimitPrices(t *testing.T) {
	db := openSQLiteMyShopUnitPricesImportDB(t)
	defer db.Close()
	if _, err := dbmigrations.ApplyToVersion(context.Background(), db, nil, CharacterMyShopUnitPricesMigrationVersion); err != nil {
		t.Fatal(err)
	}
	character := rosterExportCharacter(11, "AlphaWar")
	seedItemStateRoster(t, db, []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}}})
	for vnum := 1; vnum <= loginticket.MyShopUnitPriceMax+1; vnum++ {
		if _, err := db.Exec(`INSERT INTO character_myshop_unit_prices (character_id, vnum, unit_price) VALUES (11, ?, 0)`, vnum); err != nil {
			t.Fatal(err)
		}
	}
	store := NewSQLMyShopUnitPricesStore(db)
	if _, err := store.Load("Alpha"); err == nil {
		t.Fatal("Load accepted over-limit price list")
	}
	if _, err := store.ExportCharacterMyShopUnitPrices(); err == nil {
		t.Fatal("Export accepted over-limit price list")
	}
}
