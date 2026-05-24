package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestRuntimeSeparatesLivePositionFromPersistedSnapshot(t *testing.T) {
	persisted := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "PeerTwo",
		MapIndex: 1,
		X:        1300,
		Y:        2300,
		Empire:   2,
		GuildID:  15,
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	runtime.SetLivePosition(42, 1700, 2800)

	gotPersisted := runtime.PersistedSnapshot()
	if gotPersisted.MapIndex != 1 || gotPersisted.X != 1300 || gotPersisted.Y != 2300 {
		t.Fatalf("expected persisted snapshot to stay unchanged, got %+v", gotPersisted)
	}
	gotLive := runtime.LiveCharacter()
	if gotLive.MapIndex != 42 || gotLive.X != 1700 || gotLive.Y != 2800 {
		t.Fatalf("expected live character location to change independently, got %+v", gotLive)
	}
	if gotPosition := runtime.LivePosition(); gotPosition != worldruntime.NewPosition(42, 1700, 2800) {
		t.Fatalf("expected live position value object for relocated character, got %+v", gotPosition)
	}
	if gotLive.ID != persisted.ID || gotLive.VID != persisted.VID || gotLive.Name != persisted.Name || gotLive.Empire != persisted.Empire || gotLive.GuildID != persisted.GuildID {
		t.Fatalf("expected live character identity to remain anchored to persisted snapshot, got %+v", gotLive)
	}
	if link := runtime.SessionLink(); link.Login != "peer-two" || link.CharacterIndex != 1 {
		t.Fatalf("unexpected session link: %+v", link)
	}
	if persisted.MapIndex != 1 || persisted.X != 1300 || persisted.Y != 2300 {
		t.Fatalf("expected original persisted input to stay unchanged, got %+v", persisted)
	}
}

func TestRuntimeCanRefreshPersistedAndLiveSnapshotTogether(t *testing.T) {
	persisted := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "PeerTwo",
		MapIndex: 1,
		X:        1300,
		Y:        2300,
		Empire:   2,
		GuildID:  15,
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	runtime.SetLivePosition(42, 1700, 2800)

	updated := persisted
	updated.MapIndex = 43
	updated.X = 1900
	updated.Y = 3100
	runtime.ApplyPersistedSnapshot(updated)

	if gotPersisted := runtime.PersistedSnapshot(); gotPersisted.MapIndex != 43 || gotPersisted.X != 1900 || gotPersisted.Y != 3100 {
		t.Fatalf("expected refreshed persisted snapshot, got %+v", gotPersisted)
	}
	if gotLive := runtime.LiveCharacter(); gotLive.MapIndex != 43 || gotLive.X != 1900 || gotLive.Y != 3100 {
		t.Fatalf("expected live character to realign with refreshed persisted snapshot, got %+v", gotLive)
	}
}

func TestRuntimeCanRefreshPersistedSnapshotWithoutClobberingLiveState(t *testing.T) {
	persisted := loginticket.Character{
		ID:       0x01030102,
		VID:      0x02040102,
		Name:     "PeerTwo",
		MapIndex: 1,
		X:        1300,
		Y:        2300,
		Empire:   2,
		GuildID:  15,
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	runtime.SetLivePosition(42, 1700, 2800)
	if _, ok := runtime.ApplyPointDelta(1, 1, -50); !ok {
		t.Fatal("expected live point delta to succeed before persisted-only refresh")
	}

	updated := persisted
	updated.MapIndex = 43
	updated.X = 1900
	updated.Y = 3100
	runtime.SetPersistedSnapshot(updated)

	if gotPersisted := runtime.PersistedSnapshot(); gotPersisted.MapIndex != 43 || gotPersisted.X != 1900 || gotPersisted.Y != 3100 {
		t.Fatalf("expected persisted snapshot to refresh without clobbering live state, got %+v", gotPersisted)
	}
	gotLive := runtime.LiveCharacter()
	if gotLive.MapIndex != 42 || gotLive.X != 1700 || gotLive.Y != 2800 {
		t.Fatalf("expected live position to stay unchanged after persisted-only refresh, got %+v", gotLive)
	}
	if gotLive.Points[1] != 650 {
		t.Fatalf("expected live points[1] to stay at 650 after persisted-only refresh, got %d", gotLive.Points[1])
	}
}

func TestNilRuntimeReturnsZeroLiveCharacter(t *testing.T) {
	var runtime *Runtime
	if got := runtime.LiveCharacter(); !reflect.DeepEqual(got, loginticket.Character{}) {
		t.Fatalf("expected nil runtime to return zero live character, got %+v", got)
	}
}

func bootstrapConsumableTemplate(vnum uint32, pointType uint8, pointIndex uint8, pointDelta int32, message string) itemcatalog.Template {
	return itemcatalog.Template{
		Vnum:      vnum,
		Name:      "Template Potion",
		Stackable: true,
		MaxCount:  200,
		UseEffect: &itemcatalog.UseEffect{
			PointType:  pointType,
			PointIndex: pointIndex,
			PointDelta: pointDelta,
			Message:    message,
		},
	}
}

func bootstrapEquipmentPointTemplate(vnum uint32, equipSlot inventory.EquipmentSlot, pointType uint8, pointIndex uint8, pointDelta int32) itemcatalog.Template {
	return itemcatalog.Template{
		Vnum:      vnum,
		Name:      "Template Blade",
		Stackable: false,
		MaxCount:  1,
		EquipSlot: equipSlot.String(),
		EquipEffect: &itemcatalog.PointEffect{
			PointType:  pointType,
			PointIndex: pointIndex,
			PointDelta: pointDelta,
		},
	}
}

func TestRuntimeItemUseConsumesBootstrapConsumableWithoutMutatingPersistedPoints(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected bootstrap consumable use to succeed")
	}
	if result.ItemRemoved {
		t.Fatal("expected stacked consumable to remain in inventory after one use")
	}
	if result.Item.ID != 11 || result.Item.Vnum != 27001 || result.Item.Count != 2 || result.Item.Slot != 5 {
		t.Fatalf("unexpected updated inventory item: %+v", result.Item)
	}
	if result.PointAmount != 50 || result.PointValue != 750 || result.PointType != 1 {
		t.Fatalf("unexpected point change result: %+v", result)
	}
	if result.EffectMessage != "consume:27001:+50" {
		t.Fatalf("unexpected effect message: %q", result.EffectMessage)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 750 {
		t.Fatalf("expected live points[1] to be incremented to 750, got %d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected live inventory after use: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(persisted.Inventory, []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}}) {
		t.Fatalf("expected original persisted inventory input to stay unchanged, got %#v", persisted.Inventory)
	}
}

func TestRuntimeItemUseRemovesTheLastConsumableStack(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 1, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected final-stack consumable use to succeed")
	}
	if !result.ItemRemoved {
		t.Fatal("expected final-stack consumable use to remove the inventory slot")
	}
	if result.PointAmount != 50 || result.PointValue != 750 || result.PointType != 1 {
		t.Fatalf("unexpected point change result: %+v", result)
	}
	live := runtime.LiveCharacter()
	if len(live.Inventory) != 0 {
		t.Fatalf("expected live inventory to be empty after consuming the last stack, got %#v", live.Inventory)
	}
	if live.Points[1] != 750 {
		t.Fatalf("expected live points[1] to be incremented to 750, got %d", live.Points[1])
	}
}

func TestRuntimeUseItemOnItemMergesCompatibleStacksWithoutPointEffect(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27001, Count: 4, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItemOnItem(5, 6, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50"))
	if !ok {
		t.Fatal("expected compatible stack use-to-item merge to succeed")
	}
	if !result.Changed || result.CountOnly || result.FromOccupied {
		t.Fatalf("unexpected use-to-item merge result metadata: %+v", result)
	}
	if result.ToItem.ID != 12 || result.ToItem.Vnum != 27001 || result.ToItem.Count != 7 || result.ToItem.Slot != 6 {
		t.Fatalf("unexpected destination item after use-to-item merge: %+v", result.ToItem)
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 700 {
		t.Fatalf("expected use-to-item merge to avoid point effects, got points[1]=%d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, []inventory.ItemInstance{{ID: 12, Vnum: 27001, Count: 7, Slot: 6}}) {
		t.Fatalf("unexpected live inventory after use-to-item merge: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to remain unchanged before save-back, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemMergesPartialStackWhenTargetHasLimitedRoom(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 7, Slot: 5}, {ID: 12, Vnum: 27001, Count: 8, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	template := bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")
	template.MaxCount = 10
	result, ok := runtime.UseItemOnItem(5, 6, template)
	if !ok {
		t.Fatal("expected partial compatible stack use-to-item merge to succeed")
	}
	if !result.Changed || !result.CountOnly || !result.FromOccupied || !result.ToOccupied {
		t.Fatalf("unexpected partial use-to-item metadata: %+v", result)
	}
	if result.FromItem.ID != 11 || result.FromItem.Count != 5 || result.FromItem.Slot != 5 {
		t.Fatalf("unexpected source remainder after partial merge: %+v", result.FromItem)
	}
	if result.ToItem.ID != 12 || result.ToItem.Count != 10 || result.ToItem.Slot != 6 {
		t.Fatalf("unexpected target stack after partial merge: %+v", result.ToItem)
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 700 {
		t.Fatalf("expected partial use-to-item merge to avoid point effects, got points[1]=%d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 5, Slot: 5}, {ID: 12, Vnum: 27001, Count: 10, Slot: 6}}) {
		t.Fatalf("unexpected live inventory after partial use-to-item merge: %#v", live.Inventory)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to remain unchanged before save-back, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsPointUseTemplateWithoutCompatibleTarget(t *testing.T) {
	persisted := loginticket.Character{
		ID:        0x01030102,
		VID:       0x02040102,
		Name:      "PeerTwo",
		Points:    [255]int32{1: 700},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}, {ID: 12, Vnum: 27002, Count: 4, Slot: 6}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.UseItemOnItem(5, 6, bootstrapConsumableTemplate(27001, 1, 1, 50, "consume:27001:+50")); ok {
		t.Fatal("expected use-to-item to reject incompatible target instead of falling back to normal item use")
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 700 {
		t.Fatalf("expected rejected use-to-item to avoid point effects, got points[1]=%d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, persisted.Inventory) {
		t.Fatalf("expected rejected use-to-item to leave inventory unchanged, got %#v", live.Inventory)
	}
}

func TestRuntimeItemUseResolvesPointEffectFromTemplateMetadata(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27002, Count: 3, Slot: 5}},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.UseItem(5, bootstrapConsumableTemplate(27002, 7, 1, 25, "consume:27002:+25"))
	if !ok {
		t.Fatal("expected template-defined consumable use to succeed")
	}
	if result.ItemRemoved {
		t.Fatal("expected stacked template-defined consumable to remain in inventory after one use")
	}
	if result.PointType != 7 || result.PointAmount != 25 || result.PointValue != 725 {
		t.Fatalf("expected template-defined point change, got %+v", result)
	}
	if result.EffectMessage != "consume:27002:+25" {
		t.Fatalf("unexpected template-defined effect message: %q", result.EffectMessage)
	}
	if result.Item.ID != 11 || result.Item.Vnum != 27002 || result.Item.Count != 2 || result.Item.Slot != 5 {
		t.Fatalf("unexpected updated template-defined inventory item: %+v", result.Item)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	live := runtime.LiveCharacter()
	if live.Points[1] != 725 {
		t.Fatalf("expected live points[1] to follow template-defined delta, got %d", live.Points[1])
	}
	if !reflect.DeepEqual(live.Inventory, []inventory.ItemInstance{{ID: 11, Vnum: 27002, Count: 2, Slot: 5}}) {
		t.Fatalf("unexpected live inventory after template-defined use: %#v", live.Inventory)
	}
}

func TestRuntimePickupGroundItemDistributesStackAcrossCompatibleStacksBeforeFreshSlot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 198, Slot: 0},
			{ID: 12, Vnum: 27001, Count: 199, Slot: 2},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 5, Slot: 6}, 6, 200)
	if !ok {
		t.Fatal("expected pickup to fill compatible stacks and place the remainder")
	}
	if !result.Split || result.Merged || len(result.UpdatedItems) != 2 {
		t.Fatalf("expected split pickup result with two updated stacks, got %+v", result)
	}
	wantUpdated := []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 200, Slot: 0},
		{ID: 12, Vnum: 27001, Count: 200, Slot: 2},
	}
	if !reflect.DeepEqual(result.UpdatedItems, wantUpdated) {
		t.Fatalf("unexpected split pickup updated stacks: got %#v want %#v", result.UpdatedItems, wantUpdated)
	}
	if result.Placed != (inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 2, Slot: 6}) {
		t.Fatalf("unexpected split pickup remainder placement: %+v", result.Placed)
	}
	wantLive := []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 200, Slot: 0},
		{ID: 12, Vnum: 27001, Count: 200, Slot: 2},
		{ID: 13, Vnum: 27001, Count: 2, Slot: 6},
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, wantLive) {
		t.Fatalf("unexpected live inventory after split pickup: got %#v want %#v", got, wantLive)
	}
}

func TestRuntimePickupGroundItemMergesIntoCompatibleStackBeforeFreshSlot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 4, Slot: 0},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 3, Slot: 6}, 6, 200)
	if !ok {
		t.Fatal("expected pickup to merge into the compatible stack")
	}
	if !result.Merged || result.Split || result.Placed.ID != 0 {
		t.Fatalf("expected pure merge result, got %+v", result)
	}
	if result.Updated != (inventory.ItemInstance{ID: 11, Vnum: 27001, Count: 7, Slot: 0}) {
		t.Fatalf("unexpected merged item: %+v", result.Updated)
	}
	wantLive := []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 7, Slot: 0}}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, wantLive) {
		t.Fatalf("unexpected live inventory after merge pickup: got %#v want %#v", got, wantLive)
	}
}

func TestRuntimePickupGroundItemSkipsLockedCompatibleStacks(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 4, Slot: 0, Locked: true},
			{ID: 12, Vnum: 27001, Count: 6, Slot: 2},
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 3, Slot: 6}, 6, 200)
	if !ok {
		t.Fatal("expected pickup to skip locked stack and merge into unlocked compatible stack")
	}
	if !result.Merged || result.Split || result.Placed.ID != 0 {
		t.Fatalf("expected locked-stack pickup to remain a pure merge, got %+v", result)
	}
	if result.Updated != (inventory.ItemInstance{ID: 12, Vnum: 27001, Count: 9, Slot: 2}) {
		t.Fatalf("unexpected locked-stack pickup merge result: %+v", result.Updated)
	}
	wantLive := []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 4, Slot: 0, Locked: true},
		{ID: 12, Vnum: 27001, Count: 9, Slot: 2},
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, wantLive) {
		t.Fatalf("unexpected live inventory after locked-stack pickup: got %#v want %#v", got, wantLive)
	}
}

func TestRuntimePickupGroundItemFailsWhenOnlyCompatibleCapacityIsLocked(t *testing.T) {
	persisted := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo"}
	persisted.Inventory = []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 4, Slot: 0, Locked: true}}
	for slot := inventory.SlotIndex(1); slot < inventory.CarriedInventorySlotCount; slot++ {
		persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{ID: uint64(1000 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 3, Slot: 6}, 6, 200); ok {
		t.Fatalf("expected pickup to fail when only compatible capacity is locked and no fresh slot exists, got %+v", result)
	}
	want := append([]inventory.ItemInstance(nil), persisted.Inventory...)
	sortInventoryItems(want)
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected failed locked-stack pickup to leave live inventory unchanged, got %#v want %#v", got, want)
	}
}

func TestRuntimePickupGroundItemFailsWhenPartialStacksNeedRemainderButNoFreshSlot(t *testing.T) {
	persisted := loginticket.Character{ID: 0x01030102, VID: 0x02040102, Name: "PeerTwo"}
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 198, Slot: 0},
		{ID: 12, Vnum: 27001, Count: 199, Slot: 2},
	}
	for slot := inventory.SlotIndex(1); slot < inventory.CarriedInventorySlotCount; slot++ {
		if slot == 2 {
			continue
		}
		persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{ID: uint64(1000 + slot), Vnum: 28000 + uint32(slot), Count: 1, Slot: slot})
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if result, ok := runtime.PickupGroundItem(inventory.ItemInstance{ID: 13, Vnum: 27001, Count: 5, Slot: 6}, 6, 200); ok {
		t.Fatalf("expected split pickup with no fresh remainder slot to fail closed, got %+v", result)
	}
	want := append([]inventory.ItemInstance(nil), persisted.Inventory...)
	sortInventoryItems(want)
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected failed split pickup to leave live inventory unchanged, got %#v want %#v", got, want)
	}
}

func TestRuntimeApplyEquipTemplateEffectAdjustsLivePointsWithoutMutatingPersistedSnapshot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.ApplyEquipTemplateEffect(bootstrapEquipmentPointTemplate(12200, inventory.EquipmentSlotWeapon, 1, 1, 10))
	if !ok {
		t.Fatal("expected equip template effect to succeed")
	}
	if result.PointType != 1 || result.PointAmount != 10 || result.PointValue != 710 {
		t.Fatalf("unexpected equip point change result: %+v", result)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	if got := runtime.LiveCharacter().Points[1]; got != 710 {
		t.Fatalf("expected live points[1] to be incremented to 710, got %d", got)
	}
}

func TestRuntimeApplyPointDeltaAdjustsLivePointsWithoutMutatingPersistedSnapshot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.ApplyPointDelta(1, 1, -1)
	if !ok {
		t.Fatal("expected point delta application to succeed")
	}
	if result.PointType != 1 || result.PointAmount != -1 || result.PointValue != 699 {
		t.Fatalf("unexpected point delta result: %+v", result)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	if got := runtime.LiveCharacter().Points[1]; got != 699 {
		t.Fatalf("expected live points[1] to be decremented to 699, got %d", got)
	}
}

func TestRuntimeRemoveEquipTemplateEffectRevertsLivePointsWithoutMutatingPersistedSnapshot(t *testing.T) {
	persisted := loginticket.Character{
		ID:   0x01030102,
		VID:  0x02040102,
		Name: "PeerTwo",
		Points: [255]int32{
			1: 700,
		},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.ApplyEquipTemplateEffect(bootstrapEquipmentPointTemplate(12200, inventory.EquipmentSlotWeapon, 1, 1, 10)); !ok {
		t.Fatal("expected template-backed equip effect application to succeed before removal")
	}
	result, ok := runtime.RemoveEquipTemplateEffect(bootstrapEquipmentPointTemplate(12200, inventory.EquipmentSlotWeapon, 1, 1, 10))
	if !ok {
		t.Fatal("expected template-backed equip effect removal to succeed")
	}
	if result.PointType != 1 || result.PointAmount != -10 || result.PointValue != 700 {
		t.Fatalf("unexpected equip-derived point removal result: %+v", result)
	}
	if got := runtime.PersistedSnapshot().Points[1]; got != 700 {
		t.Fatalf("expected persisted points to remain unchanged, got %d", got)
	}
	if got := runtime.LiveCharacter().Points[1]; got != 700 {
		t.Fatalf("expected live points[1] to be restored to 700, got %d", got)
	}
}
