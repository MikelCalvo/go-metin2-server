package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestRuntimeKeepsLiveCurrencyAndItemStateSeparateFromPersistedSnapshot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	moveResult, ok := runtime.MoveInventoryItem(5, 6)
	if !ok {
		t.Fatal("expected inventory move to succeed")
	}
	if moveResult.From != 5 || moveResult.To != 6 || moveResult.FromOccupied || !moveResult.ToOccupied || moveResult.ToItem.Slot != 6 || moveResult.ToItem.ID != 11 {
		t.Fatalf("unexpected move result: %+v", moveResult)
	}
	if !runtime.EquipItem(8, inventory.EquipmentSlotBody) {
		t.Fatal("expected equip to succeed")
	}
	runtime.SetLiveGold(88000)

	gotPersisted := runtime.PersistedSnapshot()
	if gotPersisted.Gold != 125000 {
		t.Fatalf("expected persisted gold to stay 125000, got %d", gotPersisted.Gold)
	}
	if !reflect.DeepEqual(gotPersisted.Inventory, []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected persisted inventory: %#v", gotPersisted.Inventory)
	}
	if !reflect.DeepEqual(gotPersisted.Equipment, []inventory.ItemInstance{
		{ID: 21, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
	}) {
		t.Fatalf("unexpected persisted equipment: %#v", gotPersisted.Equipment)
	}
	if persisted.Gold != 125000 || persisted.Inventory[0].Slot != 5 || len(persisted.Equipment) != 1 {
		t.Fatalf("expected original persisted input to stay unchanged, got %+v", persisted)
	}

	gotLive := runtime.LiveCharacter()
	if gotLive.Gold != 88000 {
		t.Fatalf("expected live gold 88000, got %d", gotLive.Gold)
	}
	if !reflect.DeepEqual(gotLive.Inventory, []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 6},
	}) {
		t.Fatalf("unexpected live inventory: %#v", gotLive.Inventory)
	}
	if !reflect.DeepEqual(gotLive.Equipment, []inventory.ItemInstance{
		{ID: 12, Vnum: 1120, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotBody},
		{ID: 21, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
	}) {
		t.Fatalf("unexpected live equipment: %#v", gotLive.Equipment)
	}
}

func TestRuntimeAccessorsDeepCopyLiveAndPersistedItemState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	persistedSnapshot := runtime.PersistedSnapshot()
	liveInventory := runtime.LiveInventory()
	liveEquipment := runtime.LiveEquipment()

	persistedSnapshot.Inventory[0].Count = 99
	liveInventory[0].Count = 77
	liveEquipment[0].Vnum = 9999

	if got := runtime.PersistedSnapshot().Inventory[0].Count; got != 3 {
		t.Fatalf("expected persisted inventory count to stay 3, got %d", got)
	}
	if got := runtime.LiveInventory()[0].Count; got != 3 {
		t.Fatalf("expected live inventory count to stay 3, got %d", got)
	}
	if got := runtime.LiveEquipment()[0].Vnum; got != 19 {
		t.Fatalf("expected live equipment vnum to stay 19, got %d", got)
	}
}

func TestRuntimeApplyPersistedSnapshotRealignsLiveCurrencyInventoryAndEquipment(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.MoveInventoryItem(5, 6); !ok {
		t.Fatal("expected inventory move to succeed")
	}
	if !runtime.EquipItem(8, inventory.EquipmentSlotBody) {
		t.Fatal("expected equip to succeed")
	}
	runtime.SetLiveGold(1)

	updated := loginticket.Character{
		ID:       persisted.ID,
		VID:      persisted.VID,
		Name:     persisted.Name,
		MapIndex: 42,
		X:        1700,
		Y:        2800,
		Empire:   persisted.Empire,
		GuildID:  persisted.GuildID,
		Gold:     64000,
		Inventory: []inventory.ItemInstance{
			{ID: 31, Vnum: 27002, Count: 9, Slot: 2},
		},
		Equipment: []inventory.ItemInstance{
			{ID: 41, Vnum: 1300, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotShield},
		},
	}
	runtime.ApplyPersistedSnapshot(updated)

	gotPersisted := runtime.PersistedSnapshot()
	if !reflect.DeepEqual(gotPersisted.Inventory, updated.Inventory) {
		t.Fatalf("expected refreshed persisted inventory, got %#v want %#v", gotPersisted.Inventory, updated.Inventory)
	}
	if !reflect.DeepEqual(gotPersisted.Equipment, updated.Equipment) {
		t.Fatalf("expected refreshed persisted equipment, got %#v want %#v", gotPersisted.Equipment, updated.Equipment)
	}
	gotLive := runtime.LiveCharacter()
	if gotLive.MapIndex != 42 || gotLive.X != 1700 || gotLive.Y != 2800 {
		t.Fatalf("expected live location to realign with refreshed snapshot, got %+v", gotLive)
	}
	if gotLive.Gold != 64000 {
		t.Fatalf("expected live gold 64000, got %d", gotLive.Gold)
	}
	if !reflect.DeepEqual(gotLive.Inventory, updated.Inventory) {
		t.Fatalf("expected live inventory to realign, got %#v want %#v", gotLive.Inventory, updated.Inventory)
	}
	if !reflect.DeepEqual(gotLive.Equipment, updated.Equipment) {
		t.Fatalf("expected live equipment to realign, got %#v want %#v", gotLive.Equipment, updated.Equipment)
	}
}

func TestRuntimeCanUnequipLiveItemBackIntoInventory(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if !runtime.UnequipItem(inventory.EquipmentSlotWeapon, 4) {
		t.Fatal("expected unequip to succeed")
	}

	gotLive := runtime.LiveCharacter()
	if !reflect.DeepEqual(gotLive.Inventory, []inventory.ItemInstance{
		{ID: 21, Vnum: 19, Count: 1, Slot: 4},
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after unequip: %#v", gotLive.Inventory)
	}
	if len(gotLive.Equipment) != 0 {
		t.Fatalf("expected live equipment to be empty after unequip, got %#v", gotLive.Equipment)
	}
	if len(runtime.PersistedSnapshot().Equipment) != 1 {
		t.Fatalf("expected persisted equipment to stay unchanged, got %#v", runtime.PersistedSnapshot().Equipment)
	}
}

func TestRuntimeMoveInventoryItemSwapsOccupiedSlots(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.MoveInventoryItem(5, 8)
	if !ok {
		t.Fatal("expected inventory swap to succeed")
	}
	if !result.Changed || !result.FromOccupied || !result.ToOccupied {
		t.Fatalf("expected swap result to describe both occupied slots, got %+v", result)
	}
	if result.From != 5 || result.To != 8 {
		t.Fatalf("unexpected swap slots: %+v", result)
	}
	if result.FromItem.ID != 12 || result.FromItem.Slot != 5 {
		t.Fatalf("expected destination occupant to move into slot 5, got %+v", result.FromItem)
	}
	if result.ToItem.ID != 11 || result.ToItem.Slot != 8 {
		t.Fatalf("expected source item to move into slot 8, got %+v", result.ToItem)
	}

	gotLive := runtime.LiveInventory()
	if !reflect.DeepEqual(gotLive, []inventory.ItemInstance{
		{ID: 12, Vnum: 1120, Count: 1, Slot: 5},
		{ID: 11, Vnum: 27001, Count: 3, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after swap: %#v", gotLive)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestNilRuntimeInventoryEquipmentAndCurrencyHelpersAreSafe(t *testing.T) {
	var runtime *Runtime

	if got := runtime.LiveGold(); got != 0 {
		t.Fatalf("expected zero live gold, got %d", got)
	}
	if got := runtime.LiveInventory(); len(got) != 0 {
		t.Fatalf("expected empty live inventory, got %#v", got)
	}
	if got := runtime.LiveEquipment(); len(got) != 0 {
		t.Fatalf("expected empty live equipment, got %#v", got)
	}
	runtime.SetLiveGold(10)
	if _, ok := runtime.MoveInventoryItem(1, 2); ok {
		t.Fatal("expected nil runtime inventory move to fail")
	}
	if runtime.EquipItem(1, inventory.EquipmentSlotBody) {
		t.Fatal("expected nil runtime equip to fail")
	}
	if runtime.UnequipItem(inventory.EquipmentSlotWeapon, 2) {
		t.Fatal("expected nil runtime unequip to fail")
	}
}

func inventoryRuntimeCharacterFixture() loginticket.Character {
	return loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "PeerTwo",
		MapIndex: 1,
		X:        1300,
		Y:        2300,
		Empire:   2,
		GuildID:  15,
		Gold:     125000,
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
			{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
		},
		Equipment: []inventory.ItemInstance{
			{ID: 21, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
		},
	}
}
