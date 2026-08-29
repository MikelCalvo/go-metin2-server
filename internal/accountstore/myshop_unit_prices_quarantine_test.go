package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestValidateCharacterMyShopUnitPricesExportAcceptsCanonicalExport(t *testing.T) {
	character := rosterExportCharacter(11, "AlphaWar")
	character.MyShopUnitPrices = []loginticket.MyShopUnitPrice{
		{Vnum: 27002, UnitPrice: 200},
		{Vnum: 27001, UnitPrice: 500},
	}
	export, err := ExportCharacterMyShopUnitPrices([]Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{character}},
	})
	if err != nil {
		t.Fatalf("seed export: %v", err)
	}

	summary, err := ValidateCharacterMyShopUnitPricesExport(export)
	if err != nil {
		t.Fatalf("validate character myshop unit-prices export: %v", err)
	}
	want := CharacterMyShopUnitPricesQuarantineSummary{
		CharacterCount: 1,
		PriceRowCount:  2,
		CharacterIDs:   []uint32{11},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateCharacterMyShopUnitPricesExportRejectsWrongMigrationBoundary(t *testing.T) {
	export := CharacterMyShopUnitPricesExport{
		MigrationVersion: 2,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices:       []CharacterMyShopUnitPriceRow{},
	}
	if _, err := ValidateCharacterMyShopUnitPricesExport(export); !errors.Is(err, ErrInvalidCharacterMyShopUnitPricesExport) {
		t.Fatalf("expected ErrInvalidCharacterMyShopUnitPricesExport, got %v", err)
	}
}

func TestValidateCharacterMyShopUnitPricesExportRejectsMalformedRows(t *testing.T) {
	cases := []struct {
		name   string
		export CharacterMyShopUnitPricesExport
	}{
		{
			name: "zero vnum",
			export: CharacterMyShopUnitPricesExport{
				MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
				MigrationName:    CharacterMyShopUnitPricesMigrationName,
				UnitPrices:       []CharacterMyShopUnitPriceRow{{CharacterID: 11, Vnum: 0, UnitPrice: 1}},
			},
		},
		{
			name: "duplicate vnum",
			export: CharacterMyShopUnitPricesExport{
				MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
				MigrationName:    CharacterMyShopUnitPricesMigrationName,
				UnitPrices: []CharacterMyShopUnitPriceRow{
					{CharacterID: 11, Vnum: 27001, UnitPrice: 1},
					{CharacterID: 11, Vnum: 27001, UnitPrice: 2},
				},
			},
		},
		{
			name: "zero character id",
			export: CharacterMyShopUnitPricesExport{
				MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
				MigrationName:    CharacterMyShopUnitPricesMigrationName,
				UnitPrices:       []CharacterMyShopUnitPriceRow{{CharacterID: 0, Vnum: 1, UnitPrice: 1}},
			},
		},
		{
			name: "nil unit prices",
			export: CharacterMyShopUnitPricesExport{
				MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
				MigrationName:    CharacterMyShopUnitPricesMigrationName,
			},
		},
		{
			name: "too many rows",
			export: CharacterMyShopUnitPricesExport{
				MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
				MigrationName:    CharacterMyShopUnitPricesMigrationName,
				UnitPrices: func() []CharacterMyShopUnitPriceRow {
					rows := make([]CharacterMyShopUnitPriceRow, loginticket.MyShopUnitPriceMax+1)
					for i := range rows {
						rows[i] = CharacterMyShopUnitPriceRow{CharacterID: 11, Vnum: uint32(i + 1), UnitPrice: 1}
					}
					return rows
				}(),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateCharacterMyShopUnitPricesExport(tc.export); !errors.Is(err, ErrInvalidCharacterMyShopUnitPricesExport) {
				t.Fatalf("expected ErrInvalidCharacterMyShopUnitPricesExport, got %v", err)
			}
		})
	}
}

func TestQuarantineCharacterMyShopUnitPricesExportCanonicalizesRowOrder(t *testing.T) {
	export := CharacterMyShopUnitPricesExport{
		MigrationVersion: CharacterMyShopUnitPricesMigrationVersion,
		MigrationName:    CharacterMyShopUnitPricesMigrationName,
		UnitPrices: []CharacterMyShopUnitPriceRow{
			{CharacterID: 22, Vnum: 2, UnitPrice: 20},
			{CharacterID: 11, Vnum: 27002, UnitPrice: 200},
			{CharacterID: 22, Vnum: 1, UnitPrice: 0},
			{CharacterID: 11, Vnum: 27001, UnitPrice: 500},
		},
	}

	quarantined, summary, err := QuarantineCharacterMyShopUnitPricesExport(export)
	if err != nil {
		t.Fatalf("quarantine character myshop unit-prices export: %v", err)
	}
	wantSummary := CharacterMyShopUnitPricesQuarantineSummary{
		CharacterCount: 2,
		PriceRowCount:  4,
		CharacterIDs:   []uint32{11, 22},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}
	want := []CharacterMyShopUnitPriceRow{
		{CharacterID: 11, Vnum: 27001, UnitPrice: 500},
		{CharacterID: 11, Vnum: 27002, UnitPrice: 200},
		{CharacterID: 22, Vnum: 1, UnitPrice: 0},
		{CharacterID: 22, Vnum: 2, UnitPrice: 20},
	}
	if !reflect.DeepEqual(quarantined.UnitPrices, want) {
		t.Fatalf("unexpected canonical unit prices:\n got: %#v\nwant: %#v", quarantined.UnitPrices, want)
	}
}
