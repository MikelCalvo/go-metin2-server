//go:build sqlite_harness

package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestSQLiteHarnessItemStateImportInsertsInventoryEquipmentAndQuickslots(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	accounts := []Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				func() loginticket.Character {
					character := rosterExportCharacter(11, "AlphaWar")
					active := inventory.SocketValues{1, 0, 7}
					zero := inventory.SocketValues{}
					zeroAttrs := inventory.AttributeValues{}
					activeAttrs := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 4, Value: -5}}
					character.Inventory = []inventory.ItemInstance{
						{ID: 1002, Vnum: 27002, Count: 2, Slot: 9, Sockets: &zero, Attributes: &zeroAttrs},
						{ID: 1001, Vnum: 27001, Count: 3, Slot: 5, Locked: true, Sockets: &active},
					}
					character.Equipment = []inventory.ItemInstance{
						{ID: 2002, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody, Locked: true, Sockets: &active, Attributes: &activeAttrs},
						{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
					}
					character.Quickslots = []loginticket.Quickslot{
						{Position: 4, Type: quickslotproto.TypeSkill, Slot: 9},
						{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
					}
					return character
				}(),
			},
		},
		{
			Login:  "Bravo",
			Empire: 2,
			Characters: []loginticket.Character{
				{},
				func() loginticket.Character {
					character := rosterExportCharacter(22, "BravoNinja")
					character.Inventory = []inventory.ItemInstance{{ID: 3001, Vnum: 50011, Count: 1, Slot: 0}}
					character.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeCommand, Slot: 7}}
					return character
				}(),
			},
		},
	}

	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	itemExport, err := ExportCharacterItemState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterItemState: %v", err)
	}

	result, err := ImportCharacterItemState(ctx, db, itemExport)
	if err != nil {
		t.Fatalf("ImportCharacterItemState: %v", err)
	}
	if result.MigrationVersion != CharacterItemStateMigrationVersion || result.MigrationName != CharacterItemStateMigrationName {
		t.Fatalf("unexpected migration boundary in result: %+v", result)
	}
	if result.CharacterCount != 2 || result.InventoryItemCount != 3 || result.EquipmentItemCount != 2 || result.QuickslotCount != 3 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if len(result.CharacterIDs) != 2 {
		t.Fatalf("unexpected character ids: %+v", result.CharacterIDs)
	}

	var inventoryRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items`).Scan(&inventoryRows); err != nil {
		t.Fatalf("count inventory items: %v", err)
	}
	if inventoryRows != 3 {
		t.Fatalf("inventory rows = %d, want 3", inventoryRows)
	}

	var (
		gotCharacterID int64
		gotSlot        int
		gotVnum        int64
		gotCount       int
		gotLocked      int
		gotHasSockets  int
		gotSocket0     int
		gotSocket1     int
		gotSocket2     int
	)
	if err := db.QueryRowContext(ctx, `
SELECT character_id, slot, vnum, count, locked, has_sockets, socket0, socket1, socket2
FROM character_inventory_items WHERE id = ?`, 1001).Scan(
		&gotCharacterID, &gotSlot, &gotVnum, &gotCount, &gotLocked, &gotHasSockets, &gotSocket0, &gotSocket1, &gotSocket2,
	); err != nil {
		t.Fatalf("select inventory item 1001: %v", err)
	}
	if gotCharacterID != 11 || gotSlot != 5 || gotVnum != 27001 || gotCount != 3 || gotLocked != 1 || gotHasSockets != 1 || gotSocket0 != 1 || gotSocket1 != 0 || gotSocket2 != 7 {
		t.Fatalf("inventory 1001 row mismatch: character=%d slot=%d vnum=%d count=%d locked=%d has_sockets=%d sockets=(%d,%d,%d)",
			gotCharacterID, gotSlot, gotVnum, gotCount, gotLocked, gotHasSockets, gotSocket0, gotSocket1, gotSocket2)
	}

	var (
		gotZeroHasSockets int
		gotZeroSocket0    int
		gotZeroSocket1    int
		gotZeroSocket2    int
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_sockets, socket0, socket1, socket2
FROM character_inventory_items WHERE id = ?`, 1002).Scan(
		&gotZeroHasSockets, &gotZeroSocket0, &gotZeroSocket1, &gotZeroSocket2,
	); err != nil {
		t.Fatalf("select inventory item 1002 sockets: %v", err)
	}
	if gotZeroHasSockets != 1 || gotZeroSocket0 != 0 || gotZeroSocket1 != 0 || gotZeroSocket2 != 0 {
		t.Fatalf("inventory 1002 sockets mismatch: has_sockets=%d sockets=(%d,%d,%d)", gotZeroHasSockets, gotZeroSocket0, gotZeroSocket1, gotZeroSocket2)
	}

	var (
		gotZeroHasAttributes int
		gotZeroAttr0Type     int
		gotZeroAttr0Value    int
		gotZeroAttr1Type     int
		gotZeroAttr1Value    int
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_attributes, attr0_type, attr0_value, attr1_type, attr1_value
FROM character_inventory_items WHERE id = ?`, 1002).Scan(
		&gotZeroHasAttributes, &gotZeroAttr0Type, &gotZeroAttr0Value, &gotZeroAttr1Type, &gotZeroAttr1Value,
	); err != nil {
		t.Fatalf("select inventory item 1002 attributes: %v", err)
	}
	if gotZeroHasAttributes != 1 || gotZeroAttr0Type != 0 || gotZeroAttr0Value != 0 || gotZeroAttr1Type != 0 || gotZeroAttr1Value != 0 {
		t.Fatalf("inventory 1002 attributes mismatch: has_attributes=%d attrs=(%d/%d,%d/%d)", gotZeroHasAttributes, gotZeroAttr0Type, gotZeroAttr0Value, gotZeroAttr1Type, gotZeroAttr1Value)
	}

	var (
		gotEquipCharacterID int64
		gotEquipSlot        string
		gotEquipVnum        int64
		gotEquipCount       int
		gotEquipLocked      int
		gotEquipHasSockets  int
		gotEquipSocket0     int
		gotEquipSocket1     int
		gotEquipSocket2     int
	)
	if err := db.QueryRowContext(ctx, `
SELECT character_id, equip_slot, vnum, count, locked, has_sockets, socket0, socket1, socket2
FROM character_equipment_items WHERE id = ?`, 2002).Scan(
		&gotEquipCharacterID, &gotEquipSlot, &gotEquipVnum, &gotEquipCount, &gotEquipLocked, &gotEquipHasSockets, &gotEquipSocket0, &gotEquipSocket1, &gotEquipSocket2,
	); err != nil {
		t.Fatalf("select equipment item 2002: %v", err)
	}
	if gotEquipCharacterID != 11 || gotEquipSlot != "body" || gotEquipVnum != 12200 || gotEquipCount != 1 || gotEquipLocked != 1 || gotEquipHasSockets != 1 || gotEquipSocket0 != 1 || gotEquipSocket1 != 0 || gotEquipSocket2 != 7 {
		t.Fatalf("equipment 2002 row mismatch: character=%d slot=%q vnum=%d count=%d locked=%d has_sockets=%d sockets=(%d,%d,%d)",
			gotEquipCharacterID, gotEquipSlot, gotEquipVnum, gotEquipCount, gotEquipLocked, gotEquipHasSockets, gotEquipSocket0, gotEquipSocket1, gotEquipSocket2)
	}

	var (
		gotEquipHasAttributes int
		gotEquipAttr0Type     int
		gotEquipAttr0Value    int
		gotEquipAttr1Type     int
		gotEquipAttr1Value    int
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_attributes, attr0_type, attr0_value, attr1_type, attr1_value
FROM character_equipment_items WHERE id = ?`, 2002).Scan(
		&gotEquipHasAttributes, &gotEquipAttr0Type, &gotEquipAttr0Value, &gotEquipAttr1Type, &gotEquipAttr1Value,
	); err != nil {
		t.Fatalf("select equipment item 2002 attributes: %v", err)
	}
	if gotEquipHasAttributes != 1 || gotEquipAttr0Type != 1 || gotEquipAttr0Value != 25 || gotEquipAttr1Type != 4 || gotEquipAttr1Value != -5 {
		t.Fatalf("equipment 2002 attributes mismatch: has_attributes=%d attrs=(%d/%d,%d/%d)", gotEquipHasAttributes, gotEquipAttr0Type, gotEquipAttr0Value, gotEquipAttr1Type, gotEquipAttr1Value)
	}

	var (
		gotQSCharacterID int64
		gotQSPosition    int
		gotQSType        int
		gotQSSlot        int
	)
	if err := db.QueryRowContext(ctx, `
SELECT character_id, position, type, slot
FROM character_quickslots WHERE character_id = ? AND position = ?`, 11, 2).Scan(
		&gotQSCharacterID, &gotQSPosition, &gotQSType, &gotQSSlot,
	); err != nil {
		t.Fatalf("select quickslot character 11 position 2: %v", err)
	}
	if gotQSCharacterID != 11 || gotQSPosition != 2 || gotQSType != int(quickslotproto.TypeItem) || gotQSSlot != 5 {
		t.Fatalf("quickslot row mismatch: character=%d position=%d type=%d slot=%d",
			gotQSCharacterID, gotQSPosition, gotQSType, gotQSSlot)
	}

	var unlocked int
	if err := db.QueryRowContext(ctx, `SELECT locked FROM character_inventory_items WHERE id = ?`, 1002).Scan(&unlocked); err != nil {
		t.Fatalf("select unlocked inventory item 1002: %v", err)
	}
	if unlocked != 0 {
		t.Fatalf("inventory 1002 locked = %d, want 0", unlocked)
	}
}

func TestSQLiteHarnessItemStateImportRejectsDuplicatePrimaryKey(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	accounts := []Account{
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
	}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}

	itemExport, err := ExportCharacterItemState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterItemState: %v", err)
	}
	if _, err := ImportCharacterItemState(ctx, db, itemExport); err != nil {
		t.Fatalf("first ImportCharacterItemState: %v", err)
	}

	_, err = ImportCharacterItemState(ctx, db, itemExport)
	if err == nil {
		t.Fatal("second ImportCharacterItemState succeeded, want unique conflict")
	}

	var inventoryRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items`).Scan(&inventoryRows); err != nil {
		t.Fatalf("count inventory after failed reimport: %v", err)
	}
	if inventoryRows != 1 {
		t.Fatalf("inventory rows after failed reimport = %d, want 1 (no partial second import)", inventoryRows)
	}
}

func TestSQLiteHarnessItemStateImportRejectsMissingSchema(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	export := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}

	_, err := ImportCharacterItemState(ctx, db, export)
	if !errors.Is(err, ErrCharacterItemStateImportSchemaRequired) {
		t.Fatalf("ImportCharacterItemState on empty DB error = %v, want %v", err, ErrCharacterItemStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessItemStateImportRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	export := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems: []CharacterInventoryItemRow{
			{ID: 1001, CharacterID: 11, Slot: 5, Vnum: 27001, Count: 1},
		},
		EquipmentItems: []CharacterEquipmentItemRow{},
		Quickslots:     []CharacterQuickslotRow{},
	}

	_, err := ImportCharacterItemState(ctx, db, export)
	if err == nil {
		t.Fatal("ImportCharacterItemState without parent character succeeded, want FK failure")
	}

	var inventoryRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items`).Scan(&inventoryRows); err != nil {
		t.Fatalf("count inventory after FK failure: %v", err)
	}
	if inventoryRows != 0 {
		t.Fatalf("inventory rows after FK failure = %d, want 0", inventoryRows)
	}
}

func TestSQLiteHarnessItemStateImportAcceptsEmptyExport(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	export := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}
	result, err := ImportCharacterItemState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportCharacterItemState(empty): %v", err)
	}
	if result.CharacterCount != 0 || result.InventoryItemCount != 0 || result.EquipmentItemCount != 0 || result.QuickslotCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
	}
}

func TestSQLiteHarnessItemStateImportRejectsTipThreeOnlyLedger(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemStateMigrationVersion, err)
	}

	export := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}
	_, err := ImportCharacterItemState(ctx, db, export)
	if !errors.Is(err, ErrCharacterItemStateImportSchemaRequired) {
		t.Fatalf("ImportCharacterItemState tip-3-only error = %v, want %v", err, ErrCharacterItemStateImportSchemaRequired)
	}
	if err == nil || !strings.Contains(err.Error(), "24") || !strings.Contains(err.Error(), CharacterItemInstanceSocketsMigrationName) {
		t.Fatalf("expected tip-3-only error to name additive 24, got %v", err)
	}
}

func TestSQLiteHarnessItemStateImportRejectsTipTwentyFourOnlyLedger(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceSocketsMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceSocketsMigrationVersion, err)
	}

	export := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}
	_, err := ImportCharacterItemState(ctx, db, export)
	if !errors.Is(err, ErrCharacterItemStateImportSchemaRequired) {
		t.Fatalf("ImportCharacterItemState tip-24-only error = %v, want %v", err, ErrCharacterItemStateImportSchemaRequired)
	}
	if err == nil || !strings.Contains(err.Error(), "27") || !strings.Contains(err.Error(), CharacterItemInstanceAttributesMigrationName) {
		t.Fatalf("expected tip-24-only error to name additive 27, got %v", err)
	}
}

func TestSQLiteHarnessItemStateImportReplaceReimportsSameExport(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	accounts := []Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				func() loginticket.Character {
					character := rosterExportCharacter(11, "AlphaWar")
					active := inventory.SocketValues{1, 0, 7}
					zero := inventory.SocketValues{}
					zeroAttrs := inventory.AttributeValues{}
					activeAttrs := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 4, Value: -5}}
					character.Inventory = []inventory.ItemInstance{
						{ID: 1002, Vnum: 27002, Count: 2, Slot: 9, Sockets: &zero, Attributes: &zeroAttrs},
						{ID: 1001, Vnum: 27001, Count: 3, Slot: 5, Locked: true, Sockets: &active},
					}
					character.Equipment = []inventory.ItemInstance{
						{ID: 2002, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody, Locked: true, Sockets: &active, Attributes: &activeAttrs},
						{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
					}
					character.Quickslots = []loginticket.Quickslot{
						{Position: 4, Type: quickslotproto.TypeSkill, Slot: 9},
						{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
					}
					return character
				}(),
			},
		},
	}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
	itemExport, err := ExportCharacterItemState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterItemState: %v", err)
	}
	if _, err := ImportCharacterItemState(ctx, db, itemExport); err != nil {
		t.Fatalf("first insert-only ImportCharacterItemState: %v", err)
	}
	if _, err := ImportCharacterItemState(ctx, db, itemExport); err == nil {
		t.Fatal("second insert-only ImportCharacterItemState succeeded, want unique conflict")
	}

	result, err := ImportCharacterItemState(ctx, db, itemExport, ImportCharacterItemStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("replace ImportCharacterItemState: %v", err)
	}
	if !result.Replaced {
		t.Fatalf("replace result.Replaced = false, want true")
	}
	if result.CharacterCount != 1 || result.InventoryItemCount != 2 || result.EquipmentItemCount != 2 || result.QuickslotCount != 2 {
		t.Fatalf("unexpected replace counts: %+v", result)
	}

	var inventoryRows, equipmentRows, quickslotRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items`).Scan(&inventoryRows); err != nil {
		t.Fatalf("count inventory after replace: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_equipment_items`).Scan(&equipmentRows); err != nil {
		t.Fatalf("count equipment after replace: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_quickslots`).Scan(&quickslotRows); err != nil {
		t.Fatalf("count quickslots after replace: %v", err)
	}
	if inventoryRows != 2 || equipmentRows != 2 || quickslotRows != 2 {
		t.Fatalf("after replace counts inventory=%d equipment=%d quickslots=%d, want 2/2/2", inventoryRows, equipmentRows, quickslotRows)
	}

	var (
		gotHasSockets    int
		gotSocket0       int
		gotSocket1       int
		gotSocket2       int
		gotHasAttributes int
		gotAttr0Type     int
		gotAttr0Value    int
		gotAttr1Type     int
		gotAttr1Value    int
	)
	if err := db.QueryRowContext(ctx, `
SELECT has_sockets, socket0, socket1, socket2, has_attributes, attr0_type, attr0_value, attr1_type, attr1_value
FROM character_equipment_items WHERE id = ?`, 2002).Scan(
		&gotHasSockets, &gotSocket0, &gotSocket1, &gotSocket2, &gotHasAttributes, &gotAttr0Type, &gotAttr0Value, &gotAttr1Type, &gotAttr1Value,
	); err != nil {
		t.Fatalf("select equipment 2002 after replace: %v", err)
	}
	if gotHasSockets != 1 || gotSocket0 != 1 || gotSocket1 != 0 || gotSocket2 != 7 || gotHasAttributes != 1 || gotAttr0Type != 1 || gotAttr0Value != 25 || gotAttr1Type != 4 || gotAttr1Value != -5 {
		t.Fatalf("equipment 2002 after replace mismatch: sockets=(%d,%d,%d,%d) attrs=(%d,%d/%d,%d/%d)",
			gotHasSockets, gotSocket0, gotSocket1, gotSocket2, gotHasAttributes, gotAttr0Type, gotAttr0Value, gotAttr1Type, gotAttr1Value)
	}
}

func TestSQLiteHarnessItemStateImportReplaceLeavesUnlistedCharactersUntouched(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	accounts := []Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				func() loginticket.Character {
					character := rosterExportCharacter(11, "AlphaWar")
					character.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 5}}
					character.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeItem, Slot: 5}}
					return character
				}(),
			},
		},
		{
			Login:  "Bravo",
			Empire: 2,
			Characters: []loginticket.Character{
				func() loginticket.Character {
					character := rosterExportCharacter(22, "BravoNinja")
					character.Inventory = []inventory.ItemInstance{{ID: 3001, Vnum: 50011, Count: 4, Slot: 0}}
					character.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeCommand, Slot: 7}}
					return character
				}(),
			},
		},
	}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
	fullExport, err := ExportCharacterItemState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterItemState: %v", err)
	}
	if _, err := ImportCharacterItemState(ctx, db, fullExport); err != nil {
		t.Fatalf("seed ImportCharacterItemState: %v", err)
	}

	alphaOnly := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		CharacterIDs:     []uint32{11},
		InventoryItems: []CharacterInventoryItemRow{
			{ID: 1100, CharacterID: 11, Slot: 1, Vnum: 27111, Count: 9, Locked: true, HasSockets: true, Socket0: 9, Socket1: 8, Socket2: 7, HasAttributes: true, Attr0Type: 2, Attr0Value: 11},
		},
		EquipmentItems: []CharacterEquipmentItemRow{},
		Quickslots:     []CharacterQuickslotRow{{CharacterID: 11, Position: 3, Type: uint8(quickslotproto.TypeSkill), Slot: 4}},
	}
	result, err := ImportCharacterItemState(ctx, db, alphaOnly, ImportCharacterItemStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("scoped replace ImportCharacterItemState: %v", err)
	}
	if !result.Replaced || result.CharacterCount != 1 || result.InventoryItemCount != 1 || result.QuickslotCount != 1 {
		t.Fatalf("unexpected scoped replace result: %+v", result)
	}

	var alphaInventory, bravoInventory, alphaQuickslots, bravoQuickslots int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items WHERE character_id = 11`).Scan(&alphaInventory); err != nil {
		t.Fatalf("count alpha inventory: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items WHERE character_id = 22`).Scan(&bravoInventory); err != nil {
		t.Fatalf("count bravo inventory: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_quickslots WHERE character_id = 11`).Scan(&alphaQuickslots); err != nil {
		t.Fatalf("count alpha quickslots: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_quickslots WHERE character_id = 22`).Scan(&bravoQuickslots); err != nil {
		t.Fatalf("count bravo quickslots: %v", err)
	}
	if alphaInventory != 1 || bravoInventory != 1 || alphaQuickslots != 1 || bravoQuickslots != 1 {
		t.Fatalf("scoped replace left unexpected counts alpha_inv=%d bravo_inv=%d alpha_qs=%d bravo_qs=%d",
			alphaInventory, bravoInventory, alphaQuickslots, bravoQuickslots)
	}

	var (
		gotID         int64
		gotVnum       int64
		gotCount      int
		gotLocked     int
		gotHasSockets int
		gotSocket0    int
	)
	if err := db.QueryRowContext(ctx, `
SELECT id, vnum, count, locked, has_sockets, socket0
FROM character_inventory_items WHERE character_id = 11`).Scan(
		&gotID, &gotVnum, &gotCount, &gotLocked, &gotHasSockets, &gotSocket0,
	); err != nil {
		t.Fatalf("select alpha inventory after replace: %v", err)
	}
	if gotID != 1100 || gotVnum != 27111 || gotCount != 9 || gotLocked != 1 || gotHasSockets != 1 || gotSocket0 != 9 {
		t.Fatalf("alpha inventory after replace mismatch: id=%d vnum=%d count=%d locked=%d sockets=(%d,%d)",
			gotID, gotVnum, gotCount, gotLocked, gotHasSockets, gotSocket0)
	}

	var bravoID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM character_inventory_items WHERE character_id = 22`).Scan(&bravoID); err != nil {
		t.Fatalf("select bravo inventory after replace: %v", err)
	}
	if bravoID != 3001 {
		t.Fatalf("bravo inventory id = %d, want 3001 untouched", bravoID)
	}
}

func TestSQLiteHarnessItemStateImportReplaceWipesListedCharacterWithEmptyRows(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	accounts := []Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				func() loginticket.Character {
					character := rosterExportCharacter(11, "AlphaWar")
					character.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 5}}
					character.Equipment = []inventory.ItemInstance{{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
					character.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeItem, Slot: 5}}
					return character
				}(),
			},
		},
	}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
	itemExport, err := ExportCharacterItemState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterItemState: %v", err)
	}
	if _, err := ImportCharacterItemState(ctx, db, itemExport); err != nil {
		t.Fatalf("seed ImportCharacterItemState: %v", err)
	}

	emptyWipe := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		CharacterIDs:     []uint32{11},
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}
	result, err := ImportCharacterItemState(ctx, db, emptyWipe, ImportCharacterItemStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty wipe replace: %v", err)
	}
	if !result.Replaced || result.CharacterCount != 1 || result.InventoryItemCount != 0 || result.EquipmentItemCount != 0 || result.QuickslotCount != 0 {
		t.Fatalf("unexpected empty wipe result: %+v", result)
	}

	var inventoryRows, equipmentRows, quickslotRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items`).Scan(&inventoryRows); err != nil {
		t.Fatalf("count inventory after wipe: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_equipment_items`).Scan(&equipmentRows); err != nil {
		t.Fatalf("count equipment after wipe: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_quickslots`).Scan(&quickslotRows); err != nil {
		t.Fatalf("count quickslots after wipe: %v", err)
	}
	if inventoryRows != 0 || equipmentRows != 0 || quickslotRows != 0 {
		t.Fatalf("after empty wipe counts inventory=%d equipment=%d quickslots=%d, want 0/0/0", inventoryRows, equipmentRows, quickslotRows)
	}
}

func TestSQLiteHarnessItemStateImportReplaceNoOpForEmptyCharacterIDs(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	accounts := []Account{
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
	}
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(ctx, db, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
	itemExport, err := ExportCharacterItemState(accounts)
	if err != nil {
		t.Fatalf("ExportCharacterItemState: %v", err)
	}
	if _, err := ImportCharacterItemState(ctx, db, itemExport); err != nil {
		t.Fatalf("seed ImportCharacterItemState: %v", err)
	}

	emptyIDs := CharacterItemStateExport{
		MigrationVersion: CharacterItemStateMigrationVersion,
		MigrationName:    CharacterItemStateMigrationName,
		CharacterIDs:     []uint32{},
		InventoryItems:   []CharacterInventoryItemRow{},
		EquipmentItems:   []CharacterEquipmentItemRow{},
		Quickslots:       []CharacterQuickslotRow{},
	}
	result, err := ImportCharacterItemState(ctx, db, emptyIDs, ImportCharacterItemStateOptions{Replace: true})
	if err != nil {
		t.Fatalf("empty character_ids replace: %v", err)
	}
	if !result.Replaced || result.CharacterCount != 0 {
		t.Fatalf("unexpected empty character_ids result: %+v", result)
	}

	var inventoryRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items`).Scan(&inventoryRows); err != nil {
		t.Fatalf("count inventory after no-op replace: %v", err)
	}
	if inventoryRows != 1 {
		t.Fatalf("inventory rows after no-op replace = %d, want 1", inventoryRows)
	}
}

func openSQLiteItemStateImportDB(t *testing.T) *sql.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "item-state-import-harness.sqlite")
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(sqlite): %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("Ping sqlite item-state import harness: %v", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		t.Fatalf("enable foreign_keys: %v", err)
	}
	return db
}
