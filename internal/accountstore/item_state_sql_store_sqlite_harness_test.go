//go:build sqlite_harness

package accountstore

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestSQLiteHarnessSQLItemStateStoreRoundTripPersistsInventoryEquipmentQuickslots(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	activeSockets := inventory.SocketValues{1, 0, 7}
	zeroSockets := inventory.SocketValues{}
	zeroAttrs := inventory.AttributeValues{}
	activeAttrs := inventory.AttributeValues{{Type: 1, Value: 25}, {Type: 4, Value: -5}}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.Empire = 1
	alpha.Inventory = []inventory.ItemInstance{
		{ID: 1002, Vnum: 27002, Count: 2, Slot: 9, Sockets: &zeroSockets, Attributes: &zeroAttrs},
		{ID: 1001, Vnum: 27001, Count: 3, Slot: 5, Locked: true, Sockets: &activeSockets},
	}
	alpha.Equipment = []inventory.ItemInstance{
		{ID: 2002, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody, Locked: true, Sockets: &activeSockets, Attributes: &activeAttrs},
		{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
	}
	alpha.Quickslots = []loginticket.Quickslot{
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 9},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
	}
	bravo := rosterExportCharacter(22, "BravoNinja")
	bravo.Empire = 2
	bravo.Inventory = []inventory.ItemInstance{{ID: 3001, Vnum: 50011, Count: 1, Slot: 0}}
	bravo.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeCommand, Slot: 7}}

	accounts := []Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}},
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravo}},
	}
	seedItemStateRoster(t, db, accounts)

	store := NewSQLItemStateStore(db)
	if err := store.SaveAccounts(accounts); err != nil {
		t.Fatalf("SQLItemStateStore.SaveAccounts: %v", err)
	}

	loadedAlpha, err := store.Load("Alpha")
	if err != nil {
		t.Fatalf("SQLItemStateStore.Load(Alpha): %v", err)
	}
	wantAlpha := itemStateAccountForCompare(accounts[0])
	if !reflect.DeepEqual(loadedAlpha, wantAlpha) {
		t.Fatalf("unexpected Alpha snapshot:\n got: %#v\nwant: %#v", loadedAlpha, wantAlpha)
	}
	loadedBravo, err := store.Load("bravo")
	if err != nil {
		t.Fatalf("SQLItemStateStore.Load(bravo): %v", err)
	}
	wantBravo := itemStateAccountForCompare(accounts[1])
	if !reflect.DeepEqual(loadedBravo, wantBravo) {
		t.Fatalf("unexpected Bravo snapshot:\n got: %#v\nwant: %#v", loadedBravo, wantBravo)
	}

	listed, err := store.List()
	if err != nil {
		t.Fatalf("SQLItemStateStore.List: %v", err)
	}
	wantListed := []Account{wantAlpha, wantBravo}
	if !reflect.DeepEqual(listed, wantListed) {
		t.Fatalf("unexpected SQL list:\n got: %#v\nwant: %#v", listed, wantListed)
	}

	export, err := store.ExportCharacterItemState()
	if err != nil {
		t.Fatalf("SQLItemStateStore.ExportCharacterItemState: %v", err)
	}
	wantExport, err := ExportCharacterItemState(wantListed)
	if err != nil {
		t.Fatalf("ExportCharacterItemState: %v", err)
	}
	if !reflect.DeepEqual(export, wantExport) {
		t.Fatalf("unexpected SQL export:\n got: %#v\nwant: %#v", export, wantExport)
	}
	if export.MigrationVersion != CharacterItemStateMigrationVersion || export.MigrationName != CharacterItemStateMigrationName {
		t.Fatalf("export identity = %d %q, want tip-0003", export.MigrationVersion, export.MigrationName)
	}
}

func TestSQLiteHarnessSQLItemStateStoreLoadEmptyTablesReturnsRosterShell(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.Empire = 1
	accounts := []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}}
	seedItemStateRoster(t, db, accounts)

	store := NewSQLItemStateStore(db)
	loaded, err := store.Load("Alpha")
	if err != nil {
		t.Fatalf("SQLItemStateStore.Load empty items: %v", err)
	}
	want := itemStateAccountForCompare(accounts[0])
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("empty item snapshot:\n got: %#v\nwant: %#v", loaded, want)
	}
	if _, err := store.Load("Missing"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("Load(missing) error = %v, want %v", err, ErrAccountNotFound)
	}

	export, err := store.ExportCharacterItemState()
	if err != nil {
		t.Fatalf("SQLItemStateStore.Export empty: %v", err)
	}
	if export.MigrationVersion != CharacterItemStateMigrationVersion || len(export.InventoryItems) != 0 || len(export.EquipmentItems) != 0 || len(export.Quickslots) != 0 {
		t.Fatalf("empty SQL export = %#v", export)
	}
}

func TestSQLiteHarnessSQLItemStateStoreRejectsMissingSchema(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	store := NewSQLItemStateStore(db)
	if _, err := store.Load("Alpha"); !errors.Is(err, ErrCharacterItemStateImportSchemaRequired) {
		t.Fatalf("SQLItemStateStore.Load empty-ledger error = %v, want %v", err, ErrCharacterItemStateImportSchemaRequired)
	}
	if err := store.Save(Account{Login: "Alpha", Empire: 1}); !errors.Is(err, ErrCharacterItemStateImportSchemaRequired) {
		t.Fatalf("SQLItemStateStore.Save empty-ledger error = %v, want %v", err, ErrCharacterItemStateImportSchemaRequired)
	}
}

func TestSQLiteHarnessSQLItemStateStoreSaveReplacesWholeSnapshot(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.Empire = 1
	bravo := rosterExportCharacter(22, "BravoNinja")
	bravo.Empire = 2
	accounts := []Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}},
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravo}},
	}
	seedItemStateRoster(t, db, accounts)

	store := NewSQLItemStateStore(db)
	firstAlpha := alpha
	firstAlpha.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 1}}
	firstBravo := bravo
	firstBravo.Equipment = []inventory.ItemInstance{{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	if err := store.SaveAccounts([]Account{
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{firstAlpha}},
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, firstBravo}},
	}); err != nil {
		t.Fatalf("first SQLItemStateStore.SaveAccounts: %v", err)
	}

	secondAlpha := alpha
	secondAlpha.Inventory = []inventory.ItemInstance{{ID: 1003, Vnum: 27003, Count: 4, Slot: 2, Sockets: socketsPtr(inventory.SocketValues{0, 8, 0})}}
	secondAlpha.Quickslots = []loginticket.Quickslot{{Position: 0, Type: quickslotproto.TypeItem, Slot: 2}}
	if err := store.Save(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{secondAlpha}}); err != nil {
		t.Fatalf("second SQLItemStateStore.Save: %v", err)
	}

	loaded, err := store.List()
	if err != nil {
		t.Fatalf("SQLItemStateStore.List after replace: %v", err)
	}
	wantAlpha := itemStateAccountForCompare(Account{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{secondAlpha}})
	wantBravo := itemStateAccountForCompare(Account{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravo}})
	if !reflect.DeepEqual(loaded, []Account{wantAlpha, wantBravo}) {
		t.Fatalf("replaced snapshot:\n got: %#v\nwant alpha %#v\nwant bravo %#v", loaded, wantAlpha, wantBravo)
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
	if inventoryRows != 1 || equipmentRows != 0 || quickslotRows != 1 {
		t.Fatalf("after whole-snapshot replace counts inventory=%d equipment=%d quickslots=%d, want 1/0/1", inventoryRows, equipmentRows, quickslotRows)
	}
}

func TestSQLiteHarnessSQLItemStateStoreRejectsMissingParentCharacter(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}

	character := rosterExportCharacter(11, "AlphaWar")
	character.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 0}}
	err := NewSQLItemStateStore(db).Save(Account{
		Login:      "Alpha",
		Empire:     1,
		Characters: []loginticket.Character{character},
	})
	if err == nil {
		t.Fatal("SQLItemStateStore.Save without parent character succeeded, want FK failure")
	}

	var inventoryRows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_inventory_items`).Scan(&inventoryRows); err != nil {
		t.Fatalf("count inventory after FK failure: %v", err)
	}
	if inventoryRows != 0 {
		t.Fatalf("inventory rows after FK failure = %d, want 0", inventoryRows)
	}
}

func TestSQLiteHarnessSQLItemStateStoreLoadRejectsOrphanItem(t *testing.T) {
	db := openSQLiteItemStateImportDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := dbmigrations.ApplyToVersion(ctx, db, nil, CharacterItemInstanceAttributesMigrationVersion); err != nil {
		t.Fatalf("ApplyToVersion(%d): %v", CharacterItemInstanceAttributesMigrationVersion, err)
	}
	alpha := rosterExportCharacter(11, "AlphaWar")
	alpha.Empire = 1
	seedItemStateRoster(t, db, []Account{{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alpha}}})

	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable foreign_keys: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO character_inventory_items (
    id, character_id, slot, vnum, count, locked,
    has_sockets, socket0, socket1, socket2,
    has_attributes,
    attr0_type, attr0_value, attr1_type, attr1_value, attr2_type, attr2_value,
    attr3_type, attr3_value, attr4_type, attr4_value, attr5_type, attr5_value,
    attr6_type, attr6_value
) VALUES (1001, 99, 0, 27001, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)`); err != nil {
		t.Fatalf("insert orphan inventory item: %v", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable foreign_keys: %v", err)
	}

	_, err := NewSQLItemStateStore(db).Load("Alpha")
	if err == nil || !errors.Is(err, ErrInvalidAccount) || !strings.Contains(err.Error(), "no roster parent") {
		t.Fatalf("SQLItemStateStore.Load(orphan item) error = %v, want invalid account with no roster parent", err)
	}
}

func seedItemStateRoster(t *testing.T, executor dbmigrations.SQLMigrationExecutor, accounts []Account) {
	t.Helper()
	rosterExport, err := ExportAccountCharacterRoster(accounts)
	if err != nil {
		t.Fatalf("ExportAccountCharacterRoster: %v", err)
	}
	if _, err := ImportAccountCharacterRoster(context.Background(), executor, rosterExport); err != nil {
		t.Fatalf("ImportAccountCharacterRoster: %v", err)
	}
}

func itemStateAccountForCompare(account Account) Account {
	shells := make([]loginticket.Character, accountCharacterRosterPlayerSlots)
	for i, character := range normalizeAccountCharacters(account.Characters) {
		if i >= len(shells) {
			break
		}
		if character.IsEmptySlot() {
			continue
		}
		shells[i] = loginticket.Character{
			ID:         character.ID,
			Name:       character.Name,
			Level:      1,
			MapIndex:   1,
			Empire:     account.Empire,
			Inventory:  append([]inventory.ItemInstance(nil), character.Inventory...),
			Equipment:  append([]inventory.ItemInstance(nil), character.Equipment...),
			Quickslots: append([]loginticket.Quickslot(nil), character.Quickslots...),
		}
		shells[i].NormalizeItemState()
		sort.SliceStable(shells[i].Inventory, func(a, b int) bool {
			if shells[i].Inventory[a].Slot != shells[i].Inventory[b].Slot {
				return shells[i].Inventory[a].Slot < shells[i].Inventory[b].Slot
			}
			return shells[i].Inventory[a].ID < shells[i].Inventory[b].ID
		})
		sort.SliceStable(shells[i].Equipment, func(a, b int) bool {
			if shells[i].Equipment[a].EquipSlot != shells[i].Equipment[b].EquipSlot {
				return shells[i].Equipment[a].EquipSlot < shells[i].Equipment[b].EquipSlot
			}
			return shells[i].Equipment[a].ID < shells[i].Equipment[b].ID
		})
		sort.SliceStable(shells[i].Quickslots, func(a, b int) bool {
			return shells[i].Quickslots[a].Position < shells[i].Quickslots[b].Position
		})
	}
	normalized := normalizeItemStateAccounts([]Account{{
		Login:      account.Login,
		Empire:     account.Empire,
		Characters: shells,
	}})
	return normalized[0]
}

func socketsPtr(values inventory.SocketValues) *inventory.SocketValues {
	copied := values
	return &copied
}
