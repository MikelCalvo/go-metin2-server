package accountstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestExportCharacterItemStateBuildsDeterministicRowsMatchingMigrationShape(t *testing.T) {
	activeSockets := inventory.SocketValues{1, 0, 7}
	zeroSockets := inventory.SocketValues{}

	alphaWar := rosterExportCharacter(11, "AlphaWar")
	alphaWar.Inventory = []inventory.ItemInstance{
		{ID: 1002, Vnum: 27002, Count: 2, Slot: 9, Sockets: &zeroSockets},
		{ID: 1001, Vnum: 27001, Count: 3, Slot: 5, Locked: true, Sockets: &activeSockets},
	}
	alphaWar.Equipment = []inventory.ItemInstance{
		{ID: 2002, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody, Locked: true, Sockets: &activeSockets},
		{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
	}
	alphaWar.Quickslots = []loginticket.Quickslot{
		{Position: 4, Type: quickslotproto.TypeSkill, Slot: 9},
		{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
	}

	bravoNinja := rosterExportCharacter(22, "BravoNinja")
	bravoNinja.Inventory = []inventory.ItemInstance{{ID: 3001, Vnum: 50011, Count: 1, Slot: 0}}
	bravoNinja.Quickslots = []loginticket.Quickslot{{Position: 1, Type: quickslotproto.TypeCommand, Slot: 7}}

	export, err := ExportCharacterItemState([]Account{
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravoNinja}},
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
	})
	if err != nil {
		t.Fatalf("export character item state: %v", err)
	}

	if export.MigrationVersion != CharacterItemStateMigrationVersion || export.MigrationName != CharacterItemStateMigrationName {
		t.Fatalf("unexpected migration boundary: version=%d name=%q", export.MigrationVersion, export.MigrationName)
	}
	wantInventory := []CharacterInventoryItemRow{
		{ID: 1001, CharacterID: 11, Slot: 5, Vnum: 27001, Count: 3, Locked: true, HasSockets: true, Socket0: 1, Socket2: 7},
		{ID: 1002, CharacterID: 11, Slot: 9, Vnum: 27002, Count: 2, HasSockets: true},
		{ID: 3001, CharacterID: 22, Slot: 0, Vnum: 50011, Count: 1},
	}
	if !reflect.DeepEqual(export.InventoryItems, wantInventory) {
		t.Fatalf("unexpected inventory item rows:\n got: %#v\nwant: %#v", export.InventoryItems, wantInventory)
	}
	wantEquipment := []CharacterEquipmentItemRow{
		{ID: 2002, CharacterID: 11, EquipSlot: "body", Vnum: 12200, Count: 1, Locked: true, HasSockets: true, Socket0: 1, Socket2: 7},
		{ID: 2001, CharacterID: 11, EquipSlot: "weapon", Vnum: 19, Count: 1},
	}
	if !reflect.DeepEqual(export.EquipmentItems, wantEquipment) {
		t.Fatalf("unexpected equipment item rows:\n got: %#v\nwant: %#v", export.EquipmentItems, wantEquipment)
	}
	wantQuickslots := []CharacterQuickslotRow{
		{CharacterID: 11, Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		{CharacterID: 11, Position: 4, Type: quickslotproto.TypeSkill, Slot: 9},
		{CharacterID: 22, Position: 1, Type: quickslotproto.TypeCommand, Slot: 7},
	}
	if !reflect.DeepEqual(export.Quickslots, wantQuickslots) {
		t.Fatalf("unexpected quickslot rows:\n got: %#v\nwant: %#v", export.Quickslots, wantQuickslots)
	}

	exportAgain, err := ExportCharacterItemState([]Account{
		{Login: "Bravo", Empire: 2, Characters: []loginticket.Character{{}, bravoNinja}},
		{Login: "Alpha", Empire: 1, Characters: []loginticket.Character{alphaWar}},
	})
	if err != nil {
		t.Fatalf("export character item state again: %v", err)
	}
	if !reflect.DeepEqual(export, exportAgain) {
		t.Fatalf("expected deterministic character item-state export:\n first: %#v\nsecond: %#v", export, exportAgain)
	}
}

func TestExportCharacterItemStateRejectsStateThatWouldViolateMigrationSchema(t *testing.T) {
	const aboveSignedBigInt = uint64(1 << 63)

	oversizedItemID := rosterExportCharacter(1, "OversizedItem")
	oversizedItemID.Inventory = []inventory.ItemInstance{{ID: aboveSignedBigInt, Vnum: 27001, Count: 1, Slot: 0}}

	duplicateInventoryItemIDLeft := rosterExportCharacter(1, "LeftWar")
	duplicateInventoryItemIDLeft.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 0}}
	duplicateInventoryItemIDRight := rosterExportCharacter(2, "RightWar")
	duplicateInventoryItemIDRight.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27002, Count: 1, Slot: 1}}

	duplicateEquipmentItemIDLeft := rosterExportCharacter(1, "EquipLeft")
	duplicateEquipmentItemIDLeft.Equipment = []inventory.ItemInstance{{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	duplicateEquipmentItemIDRight := rosterExportCharacter(2, "EquipRight")
	duplicateEquipmentItemIDRight.Equipment = []inventory.ItemInstance{{ID: 2001, Vnum: 12200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotBody}}

	invalidQuickslot := rosterExportCharacter(1, "BadQuickslot")
	invalidQuickslot.Quickslots = []loginticket.Quickslot{{Position: 40, Type: quickslotproto.TypeSkill, Slot: 1}}

	cases := []struct {
		name     string
		accounts []Account
	}{
		{
			name:     "item id outside signed bigint",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{oversizedItemID}}},
		},
		{
			name: "duplicate inventory item ids across characters",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{
				duplicateInventoryItemIDLeft,
				duplicateInventoryItemIDRight,
			}}},
		},
		{
			name: "duplicate equipment item ids across characters",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{
				duplicateEquipmentItemIDLeft,
				duplicateEquipmentItemIDRight,
			}}},
		},
		{
			name:     "invalid quickslot tuple",
			accounts: []Account{{Login: "Alpha", Characters: []loginticket.Character{invalidQuickslot}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ExportCharacterItemState(tc.accounts)
			if !errors.Is(err, ErrInvalidAccount) {
				t.Fatalf("expected ErrInvalidAccount, got %v", err)
			}
		})
	}
}

func TestFileStoreExportCharacterItemStateReadsCommittedSnapshots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	character := rosterExportCharacter(200, "BetaWar")
	character.Inventory = []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 1, Slot: 8}}
	character.Equipment = []inventory.ItemInstance{{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	character.Quickslots = []loginticket.Quickslot{{Position: 2, Type: quickslotproto.TypeItem, Slot: 8}}
	if err := store.Save(Account{Login: "Beta", Empire: 2, Characters: []loginticket.Character{character}}); err != nil {
		t.Fatalf("save beta account: %v", err)
	}
	if err := store.Save(Account{Login: "Alpha", Empire: 1}); err != nil {
		t.Fatalf("save alpha account: %v", err)
	}

	export, err := store.ExportCharacterItemState()
	if err != nil {
		t.Fatalf("file-store character item-state export: %v", err)
	}
	if len(export.InventoryItems) != 1 || export.InventoryItems[0].ID != 1001 || export.InventoryItems[0].CharacterID != 200 {
		t.Fatalf("unexpected file-store inventory export rows: %#v", export.InventoryItems)
	}
	if len(export.EquipmentItems) != 1 || export.EquipmentItems[0].EquipSlot != "weapon" || export.EquipmentItems[0].CharacterID != 200 {
		t.Fatalf("unexpected file-store equipment export rows: %#v", export.EquipmentItems)
	}
	if len(export.Quickslots) != 1 || export.Quickslots[0].Position != 2 || export.Quickslots[0].CharacterID != 200 {
		t.Fatalf("unexpected file-store quickslot export rows: %#v", export.Quickslots)
	}
}
