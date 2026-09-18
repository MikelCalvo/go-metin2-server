package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

func TestUnique1ItemUseWireCellIsLegacyCombinedUnique1(t *testing.T) {
	if Unique1ItemUseWearIndex != 7 {
		t.Fatalf("expected unique1 wear index 7, got %d", Unique1ItemUseWearIndex)
	}
	if Unique1ItemUseWireCell != 97 {
		t.Fatalf("expected unique1 wire cell 97, got %d", Unique1ItemUseWireCell)
	}
}

func TestRuntimeUseItemConsumesUnique1WornCellWithoutMutatingPersistedPoints(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x01030197,
		VID:    0x02040197,
		Name:   "UniqueOneUse",
		Points: [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		},
		Equipment: []inventory.ItemInstance{
			{ID: 21, Vnum: 27001, Count: 3, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
		},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "unique1-use", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")

	result, ok := runtime.UseItem(Unique1ItemUseWireCell, template)
	if !ok {
		t.Fatal("expected unique1 worn-cell use to succeed")
	}
	if result.ItemRemoved {
		t.Fatal("expected stacked unique1 consumable to remain after one use")
	}
	if result.Slot != Unique1ItemUseWireCell {
		t.Fatalf("expected unique1 result slot 97, got %d", result.Slot)
	}
	if result.Item.ID != 21 || result.Item.Vnum != 27001 || result.Item.Count != 2 || !result.Item.Equipped || result.Item.EquipSlot != inventory.EquipmentSlotUnique1 {
		t.Fatalf("unexpected unique1 remainder: %+v", result.Item)
	}
	if result.PointAmount != 50 || result.PointValue != 750 || result.PointType != 1 || result.EffectMessage != "consume:27001:+50" {
		t.Fatalf("unexpected unique1 point-change result: %+v", result)
	}
	if got := runtime.PersistedSnapshot(); got.Points[1] != 700 || !reflect.DeepEqual(got.Equipment, persisted.Equipment) || !reflect.DeepEqual(got.Inventory, persisted.Inventory) {
		t.Fatalf("unique1 use mutated persisted snapshot: inventory=%#v equipment=%#v points[1]=%d", got.Inventory, got.Equipment, got.Points[1])
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 750 {
		t.Fatalf("expected live points[1] 750 after unique1 use, got %d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, persisted.Inventory) {
		t.Fatalf("unique1 use mutated carried inventory: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(live.Equipment, []inventory.ItemInstance{{ID: 21, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1}}) {
		t.Fatalf("unexpected live unique1 remainder: %#v", live.Equipment)
	}
	if !reflect.DeepEqual(live.Quickslots, persisted.Quickslots) {
		t.Fatalf("unique1 partial consume mutated quickslots: %#v", live.Quickslots)
	}
}

func TestRuntimeUseItemConsumesLastUnique1StackWithoutCarriedQuickslotCleanup(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x01030198,
		VID:    0x02040198,
		Name:   "UniqueOneLast",
		Points: [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		},
		Equipment: []inventory.ItemInstance{
			{ID: 21, Vnum: 27001, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
		},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: quickslotproto.TypeItem, Slot: 5},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "unique1-last", CharacterIndex: 1})

	result, ok := runtime.UseItem(Unique1ItemUseWireCell, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected last unique1 stack use to succeed")
	}
	if !result.ItemRemoved {
		t.Fatal("expected last unique1 stack to be removed")
	}
	if result.Slot != Unique1ItemUseWireCell || result.Vnum != 27001 || result.PointValue != 750 {
		t.Fatalf("unexpected last unique1 result: %+v", result)
	}
	live := runtime.LiveCharacter()
	if len(live.Equipment) != 0 {
		t.Fatalf("expected unique1 last stack to clear equipment, got %#v", live.Equipment)
	}
	if !reflect.DeepEqual(live.Inventory, persisted.Inventory) {
		t.Fatalf("unique1 last stack mutated carried inventory: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(live.Quickslots, persisted.Quickslots) {
		t.Fatalf("unique1 last stack must not delete carried item quickslots: %#v", live.Quickslots)
	}
}

func TestRuntimeUseItemRejectsEmptyUnique1WithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030199,
		VID:       0x02040199,
		Name:      "UniqueOneEmpty",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "unique1-empty", CharacterIndex: 1})
	before := runtime.LiveCharacter()

	if result, ok := runtime.UseItem(Unique1ItemUseWireCell, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
		t.Fatalf("expected empty unique1 use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("empty unique1 use mutated live character: got %#v want %#v", got, before)
	}
}

func TestRuntimeUseItemRejectsOtherWornCellsWithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x0103019a,
		VID:    0x0204019a,
		Name:   "OtherWornUse",
		Points: [255]int32{1: 700},
		Equipment: []inventory.ItemInstance{
			{ID: 21, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
			{ID: 22, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotHead},
			{ID: 23, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique2},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "other-worn-use", CharacterIndex: 1})
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	before := runtime.LiveCharacter()

	for _, slot := range []inventory.SlotIndex{
		inventory.CarriedInventorySlotCount,
		inventory.CarriedInventorySlotCount + 1,
		inventory.CarriedInventorySlotCount + 4,
	} {
		if result, ok := runtime.UseItem(slot, template); ok {
			t.Fatalf("expected worn cell %d use to fail closed, got %+v", slot, result)
		}
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("other worn-cell use mutated live character: got %#v want %#v", got, before)
	}
}

func TestRuntimeUseItemRejectsEquippableTemplateOnUnique1WithoutMutation(t *testing.T) {
	persisted := loginticket.Character{
		ID:     0x0103019b,
		VID:    0x0204019b,
		Name:   "UniqueOneEquipMeta",
		Points: [255]int32{1: 700},
		Equipment: []inventory.ItemInstance{
			{ID: 21, Vnum: 11200, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "unique1-equip-meta", CharacterIndex: 1})
	before := runtime.LiveCharacter()
	template := itemcatalog.Template{
		Vnum:      11200,
		Name:      "Equippable Unique1",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: inventory.EquipmentSlotUnique1.String(),
		UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "consume:11200:+50"},
	}

	if result, ok := runtime.UseItem(Unique1ItemUseWireCell, template); ok {
		t.Fatalf("expected equippable unique1 template use to fail closed, got %+v", result)
	}
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
		t.Fatalf("equippable unique1 template use mutated live character: got %#v want %#v", got, before)
	}
}

func TestRuntimeUseItemRejectsLockedAndDuplicateUnique1WithoutMutation(t *testing.T) {
	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	cases := []struct {
		name      string
		equipment []inventory.ItemInstance
	}{
		{
			name: "locked",
			equipment: []inventory.ItemInstance{
				{ID: 21, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1, Locked: true},
			},
		},
		{
			name: "duplicate occupancy",
			equipment: []inventory.ItemInstance{
				{ID: 21, Vnum: 27001, Count: 2, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
				{ID: 22, Vnum: 27001, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotUnique1},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := loginticket.Character{
				ID:        0x0103019c,
				VID:       0x0204019c,
				Name:      "UniqueOneGuard",
				Points:    [255]int32{1: 700},
				Equipment: tc.equipment,
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "unique1-guard", CharacterIndex: 1})
			before := runtime.LiveCharacter()
			if result, ok := runtime.UseItem(Unique1ItemUseWireCell, template); ok {
				t.Fatalf("expected unique1 %s use to fail closed, got %+v", tc.name, result)
			}
			if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, before) {
				t.Fatalf("unique1 %s use mutated live character: got %#v want %#v", tc.name, got, before)
			}
		})
	}
}
