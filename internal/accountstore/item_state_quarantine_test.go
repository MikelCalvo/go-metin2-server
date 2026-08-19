package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestValidateCharacterItemStateExportAcceptsCanonicalExport(t *testing.T) {
	alphaWar := rosterExportCharacter(11, "AlphaWar")
	alphaWar.Inventory = []inventory.ItemInstance{
		{ID: 1002, Vnum: 27002, Count: 2, Slot: 9},
		{ID: 1001, Vnum: 27001, Count: 3, Slot: 5, Locked: true},
	}
	alphaWar.Equipment = []inventory.ItemInstance{
		{ID: 2002, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody, Locked: true},
		{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
	}
	alphaWar.Quickslots = []loginticket.Quickslot{
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 9},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
	}

	export, err := ExportCharacterItemState([]Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
	})
	if err != nil {
		t.Fatalf("seed export: %v", err)
	}

	summary, err := ValidateCharacterItemStateExport(export)
	if err != nil {
		t.Fatalf("validate character item-state export: %v", err)
	}
	want := CharacterItemStateQuarantineSummary{
		CharacterCount:     1,
		InventoryItemCount: 2,
		EquipmentItemCount: 2,
		QuickslotCount:     2,
		CharacterIDs:       []uint32{11},
	}
	if !reflect.DeepEqual(summary, want) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, want)
	}
}

func TestValidateCharacterItemStateExportRejectsWrongMigrationBoundary(t *testing.T) {
	export := CharacterItemStateExport{
		MigrationVersion: 11,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}
	if _, err := ValidateCharacterItemStateExport(export); !errors.Is(err, ErrInvalidCharacterItemStateExport) {
		t.Fatalf("expected ErrInvalidCharacterItemStateExport, got %v", err)
	}
}

func TestValidateCharacterItemStateExportRejectsInvalidRows(t *testing.T) {
	cases := []struct {
		name   string
		export CharacterItemStateExport
	}{
		{
			name: "nil inventory",
			export: CharacterItemStateExport{
				MigrationVersion: CharacterItemStateMigrationVersion,
				MigrationName:    CharacterItemStateMigrationName,
				EquipmentItems:   []CharacterEquipmentItemRow{},
				Quickslots:       []CharacterQuickslotRow{},
			},
		},
		{
			name: "duplicate inventory slot",
			export: CharacterItemStateExport{
				MigrationVersion: CharacterItemStateMigrationVersion,
				MigrationName:    CharacterItemStateMigrationName,
				InventoryItems: []CharacterInventoryItemRow{
					{ID: 1, CharacterID: 11, Slot: 5, Vnum: 27001, Count: 1},
					{ID: 2, CharacterID: 11, Slot: 5, Vnum: 27002, Count: 1},
				},
				EquipmentItems: []CharacterEquipmentItemRow{},
				Quickslots:     []CharacterQuickslotRow{},
			},
		},
		{
			name: "duplicate item ids across surfaces",
			export: CharacterItemStateExport{
				MigrationVersion: CharacterItemStateMigrationVersion,
				MigrationName:    CharacterItemStateMigrationName,
				InventoryItems: []CharacterInventoryItemRow{
					{ID: 1001, CharacterID: 11, Slot: 5, Vnum: 27001, Count: 1},
				},
				EquipmentItems: []CharacterEquipmentItemRow{
					{ID: 1001, CharacterID: 11, EquipSlot: "weapon", Vnum: 19, Count: 1},
				},
				Quickslots: []CharacterQuickslotRow{},
			},
		},
		{
			name: "invalid equip slot",
			export: CharacterItemStateExport{
				MigrationVersion: CharacterItemStateMigrationVersion,
				MigrationName:    CharacterItemStateMigrationName,
				InventoryItems:   []CharacterInventoryItemRow{},
				EquipmentItems: []CharacterEquipmentItemRow{
					{ID: 2001, CharacterID: 11, EquipSlot: "none", Vnum: 19, Count: 1},
				},
				Quickslots: []CharacterQuickslotRow{},
			},
		},
		{
			name: "invalid quickslot",
			export: CharacterItemStateExport{
				MigrationVersion: CharacterItemStateMigrationVersion,
				MigrationName:    CharacterItemStateMigrationName,
				InventoryItems:   []CharacterInventoryItemRow{},
				EquipmentItems:   []CharacterEquipmentItemRow{},
				Quickslots: []CharacterQuickslotRow{
					{CharacterID: 11, Position: 40, Type: quickslotproto.TypeSkill, Slot: 1},
				},
			},
		},
		{
			name: "zero character id",
			export: CharacterItemStateExport{
				MigrationVersion: CharacterItemStateMigrationVersion,
				MigrationName:    CharacterItemStateMigrationName,
				InventoryItems: []CharacterInventoryItemRow{
					{ID: 1, CharacterID: 0, Slot: 0, Vnum: 27001, Count: 1},
				},
				EquipmentItems: []CharacterEquipmentItemRow{},
				Quickslots:     []CharacterQuickslotRow{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateCharacterItemStateExport(tc.export); !errors.Is(err, ErrInvalidCharacterItemStateExport) {
				t.Fatalf("expected ErrInvalidCharacterItemStateExport, got %v", err)
			}
		})
	}
}

func TestQuarantineCharacterItemStateExportCanonicalizesRowOrder(t *testing.T) {
	export := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems: []CharacterInventoryItemRow{
			{ID: 3001, CharacterID: 22, Slot: 0, Vnum: 50011, Count: 1},
			{ID: 1002, CharacterID: 11, Slot: 9, Vnum: 27002, Count: 2},
			{ID: 1001, CharacterID: 11, Slot: 5, Vnum: 27001, Count: 3, Locked: true},
		},
		EquipmentItems: []CharacterEquipmentItemRow{
			{ID: 2001, CharacterID: 11, EquipSlot: "weapon", Vnum: 19, Count: 1},
			{ID: 2002, CharacterID: 11, EquipSlot: "body", Vnum: 12200, Count: 1, Locked: true},
		},
		Quickslots: []CharacterQuickslotRow{
			{CharacterID: 22, Position: 1, Type: quickslotproto.TypeCommand, Slot: 7},
			{CharacterID: 11, Position: 4, Type: quickslotproto.TypeSkill, Slot: 9},
			{CharacterID: 11, Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		},
	}

	quarantined, summary, err := QuarantineCharacterItemStateExport(export)
	if err != nil {
		t.Fatalf("quarantine character item-state export: %v", err)
	}
	wantSummary := CharacterItemStateQuarantineSummary{
		CharacterCount:     2,
		InventoryItemCount: 3,
		EquipmentItemCount: 2,
		QuickslotCount:     3,
		CharacterIDs:       []uint32{11, 22},
	}
	if !reflect.DeepEqual(summary, wantSummary) {
		t.Fatalf("unexpected quarantine summary:\n got: %#v\nwant: %#v", summary, wantSummary)
	}

	wantInventory := []CharacterInventoryItemRow{
		{ID: 1001, CharacterID: 11, Slot: 5, Vnum: 27001, Count: 3, Locked: true},
		{ID: 1002, CharacterID: 11, Slot: 9, Vnum: 27002, Count: 2},
		{ID: 3001, CharacterID: 22, Slot: 0, Vnum: 50011, Count: 1},
	}
	if !reflect.DeepEqual(quarantined.InventoryItems, wantInventory) {
		t.Fatalf("unexpected canonical inventory rows:\n got: %#v\nwant: %#v", quarantined.InventoryItems, wantInventory)
	}
	wantEquipment := []CharacterEquipmentItemRow{
		{ID: 2002, CharacterID: 11, EquipSlot: "body", Vnum: 12200, Count: 1, Locked: true},
		{ID: 2001, CharacterID: 11, EquipSlot: "weapon", Vnum: 19, Count: 1},
	}
	if !reflect.DeepEqual(quarantined.EquipmentItems, wantEquipment) {
		t.Fatalf("unexpected canonical equipment rows:\n got: %#v\nwant: %#v", quarantined.EquipmentItems, wantEquipment)
	}
	wantQuickslots := []CharacterQuickslotRow{
		{CharacterID: 11, Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{CharacterID: 11, Position: 4, Type: quickslotproto.TypeSkill, Slot: 9},
		{CharacterID: 22, Position: 1, Type: quickslotproto.TypeCommand, Slot: 7},
	}
	if !reflect.DeepEqual(quarantined.Quickslots, wantQuickslots) {
		t.Fatalf("unexpected canonical quickslot rows:\n got: %#v\nwant: %#v", quarantined.Quickslots, wantQuickslots)
	}
}
