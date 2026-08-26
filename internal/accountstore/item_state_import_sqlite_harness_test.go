//go:build sqlite_harness

package accountstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
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
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemStateMigrationVersion, err)
	}

	accounts := []Account{
		{
			Login:  "Alpha",
			Empire: 1,
			Characters: []loginticket.Character{
				func() loginticket.Character {
					character := rosterExportCharacter(11, "AlphaWar")
					character.Inventory = []inventory.ItemInstance{
						{ID: 1002, Vnum: 27002, Count: 2, Slot: 9},
						{ID: 1001, Vnum: 27001, Count: 3, Slot: 5, Locked: true},
					}
					character.Equipment = []inventory.ItemInstance{
						{ID: 2002, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody, Locked: true},
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
	)
	if err := db.QueryRowContext(ctx, `
SELECT character_id, slot, vnum, count, locked
FROM character_inventory_items WHERE id = ?`, 1001).Scan(
		&gotCharacterID, &gotSlot, &gotVnum, &gotCount, &gotLocked,
	); err != nil {
		t.Fatalf("select inventory item 1001: %v", err)
	}
	if gotCharacterID != 11 || gotSlot != 5 || gotVnum != 27001 || gotCount != 3 || gotLocked != 1 {
		t.Fatalf("inventory 1001 row mismatch: character=%d slot=%d vnum=%d count=%d locked=%d",
			gotCharacterID, gotSlot, gotVnum, gotCount, gotLocked)
	}

	var (
		gotEquipCharacterID int64
		gotEquipSlot        string
		gotEquipVnum        int64
		gotEquipCount       int
		gotEquipLocked      int
	)
	if err := db.QueryRowContext(ctx, `
SELECT character_id, equip_slot, vnum, count, locked
FROM character_equipment_items WHERE id = ?`, 2002).Scan(
		&gotEquipCharacterID, &gotEquipSlot, &gotEquipVnum, &gotEquipCount, &gotEquipLocked,
	); err != nil {
		t.Fatalf("select equipment item 2002: %v", err)
	}
	if gotEquipCharacterID != 11 || gotEquipSlot != "body" || gotEquipVnum != 12200 || gotEquipCount != 1 || gotEquipLocked != 1 {
		t.Fatalf("equipment 2002 row mismatch: character=%d slot=%q vnum=%d count=%d locked=%d",
			gotEquipCharacterID, gotEquipSlot, gotEquipVnum, gotEquipCount, gotEquipLocked)
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
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemStateMigrationVersion, err)
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
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemStateMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemStateMigrationVersion, err)
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
	result, err := ImportCharacterItemState(ctx, db, export)
	if err != nil {
		t.Fatalf("ImportCharacterItemState(empty): %v", err)
	}
	if result.CharacterCount != 0 || result.InventoryItemCount != 0 || result.EquipmentItemCount != 0 || result.QuickslotCount != 0 {
		t.Fatalf("empty import result = %+v, want zero counts", result)
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
