package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestUnique2ItemUseWireCellIsLegacyCombinedUnique2(t *testing.T) {
	if Unique2ItemUseWearIndex != 8 {
		t.Fatalf("expected unique2 wear index 8, got %d", Unique2ItemUseWearIndex)
	}
	if Unique2ItemUseWireCell != 98 {
		t.Fatalf("expected unique2 wire cell 98, got %d", Unique2ItemUseWireCell)
	}
	if !IsOwnedWornItemUseCell(Unique1ItemUseWireCell) || !IsOwnedWornItemUseCell(Unique2ItemUseWireCell) {
		t.Fatal("expected unique1 and unique2 to be owned worn-cell ITEM_USE addresses")
	}
	if IsOwnedWornItemUseCell(inventory.CarriedInventorySlotCount + 4) {
		t.Fatal("expected weapon wear cell to stay fail-closed")
	}
}

func TestRuntimeUseItemConsumesUnique2WornCellWithoutMutatingPersistedPoints(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x010301a7,
		VID:    0x020401a7,
		Name:   "UniqueTwoUse",
		Points: [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		},
		Equipment: []inventory.ItemInstance{
			{ID: 21, Vnum: 27001, Count: 3, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
			{ID: 31, Vnum: 27001, Count: 3, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique2},
		},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "unique2-use", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")

	result, ok := runtime.UseItem(Unique2ItemUseWireCell, template)
	if !ok {
		t.Fatal("expected unique2 worn-cell use to succeed")
	}
	if result.ItemRemoved {
		t.Fatal("expected stacked unique2 consumable to remain after one use")
	}
	if result.Slot != Unique2ItemUseWireCell {
		t.Fatalf("expected unique2 result slot 98, got %d", result.Slot)
	}
	if result.Item.ID != 31 || result.Item.Vnum != 27001 || result.Item.Count != 2 || !result.Item.Equipped || result.Item.EquipSlot != inventory.EquipmentSlotUnique2 {
		t.Fatalf("unexpected unique2 remainder: %+v", result.Item)
	}
	if result.PointAmount != 50 || result.PointValue != 750 || result.PointType != 1 || result.EffectMessage != "consume:27001:+50" {
		t.Fatalf("unexpected unique2 point-change result: %+v", result)
	}
	if got := runtime.PersistedSnapshot(); got.Points[1] != 700 || !reflect.DeepEqual(got.Equipment, persisted.Equipment) || !reflect.DeepEqual(got.Inventory, persisted.Inventory) {
		t.Fatalf("unique2 use mutated persisted snapshot: inventory=%#v equipment=%#v points[1]=%d", got.Inventory, got.Equipment, got.Points[1])
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 750 {
		t.Fatalf("expected live points[1] 750 after unique2 use, got %d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, persisted.Inventory) {
		t.Fatalf("unique2 use mutated carried inventory: %#v", live.Inventory)
	}
	wantEquipment := []inventory.ItemInstance{
		{ID: 21, Vnum: 27001, Count: 3, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
		{ID: 31, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique2},
	}
	if !reflect.DeepEqual(live.Equipment, wantEquipment) {
		t.Fatalf("unexpected live unique2 remainder: %#v", live.Equipment)
	}
	if !reflect.DeepEqual(live.Quickslots, persisted.Quickslots) {
		t.Fatalf("unique2 partial consume mutated quickslots: %#v", live.Quickslots)
	}
}

func TestRuntimeUseItemConsumesLastUnique2StackWithoutCarriedQuickslotCleanup(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x010301a8,
		VID:    0x020401a8,
		Name:   "UniqueTwoLast",
		Points: [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		},
		Equipment: []inventory.ItemInstance{
			{ID: 21, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
			{ID: 31, Vnum: 27001, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique2},
		},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "unique2-last", CharacterIndex: 1})

	result, ok := runtime.UseItem(Unique2ItemUseWireCell, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected last unique2 stack use to succeed")
	}
	if !result.ItemRemoved {
		t.Fatal("expected last unique2 stack to be removed")
	}
	if result.Slot != Unique2ItemUseWireCell || result.Vnum != 27001 || result.PointValue != 750 {
		t.Fatalf("unexpected last unique2 result: %+v", result)
	}
	live := runtime.LiveCharacter()
	wantEquipment := []inventory.ItemInstance{
		{ID: 21, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
	}
	if !reflect.DeepEqual(live.Equipment, wantEquipment) {
		t.Fatalf("expected unique2 last stack to leave unique1, got %#v", live.Equipment)
	}
	if !reflect.DeepEqual(live.Inventory, persisted.Inventory) {
		t.Fatalf("unique2 last stack mutated carried inventory: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(live.Quickslots, persisted.Quickslots) {
		t.Fatalf("unique2 last stack must not delete carried item quickslots: %#v", live.Quickslots)
	}
}

func TestRuntimeUseItemRejectsEmptyUnique2WithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x010301a9,
		VID:       0x020401a9,
		Name:      "UniqueTwoEmpty",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
		Equipment: []inventory.ItemInstance{
			{ID: 21, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "unique2-empty", CharacterIndex: 1})
	before := runtime.LiveCharacter()

	if result, ok := runtime.UseItem(Unique2ItemUseWireCell, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
		t.Fatalf("expected empty unique2 use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("empty unique2 use mutated live character: got %#v want %#v", got, before)
	}
}

func TestRuntimeUseItemRejectsEquippableTemplateOnUnique2WithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x010301aa,
		VID:    0x020401aa,
		Name:   "UniqueTwoEquipMeta",
		Points: [255]int32{1: 700},
		Equipment: []inventory.ItemInstance{
			{ID: 31, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique2},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "unique2-equip-meta", CharacterIndex: 1})
	before := runtime.LiveCharacter()
	template := itemcatalog.Template{
		Vnum:      11200,
		Name:      "Equippable Unique2",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: inventory.EquipmentSlotUnique2.String(),
		UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "consume:11200:+50"},
	}

	if result, ok := runtime.UseItem(Unique2ItemUseWireCell, template); ok {
		t.Fatalf("expected equippable unique2 template use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("equippable unique2 template use mutated live character: got %#v want %#v", got, before)
	}
}

func TestRuntimeUseItemRejectsLockedAndDuplicateUnique2WithoutMutation(t *testing.T) {
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	cases := []struct {
		name      string
		equipment []inventory.ItemInstance
	}{
		{
			name: "locked",
			equipment: []inventory.ItemInstance{
				{ID: 31, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique2, Locked: true},
			},
		},
		{
			name: "duplicate occupancy",
			equipment: []inventory.ItemInstance{
				{ID: 31, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique2},
				{ID: 32, Vnum: 27001, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique2},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x010301ab,
				VID:       0x020401ab,
				Name:      "UniqueTwoGuard",
				Points:    [255]int32{1: 700},
				Equipment: tc.equipment,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "unique2-guard", CharacterIndex: 1})
			before := runtime.LiveCharacter()
			if result, ok := runtime.UseItem(Unique2ItemUseWireCell, template); ok {
				t.Fatalf("expected unique2 %s use to fail closed, got %+v", tc.name, result)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
				t.Fatalf("unique2 %s use mutated live character: got %#v want %#v", tc.name, got, before)
			}
		})
	}
}
