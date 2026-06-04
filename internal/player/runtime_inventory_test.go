package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestDropInventoryItemRemovesWholeStack(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 3, Slot: 5}},
	}, SessionLink{})

	result, ok := runtime.DropInventoryItem(5, 3)
	if !ok {
		t.Fatalf("expected whole-stack drop to be accepted")
	}
	if !result.Changed || result.From != 5 || result.FromOccupied {
		t.Fatalf("unexpected whole-stack drop result: %+v", result)
	}
	if got := runtime.LiveInventory(); len(got) != 0 {
		t.Fatalf("expected inventory to be empty after whole-stack drop, got %#v", got)
	}
}

func TestDropInventoryItemDecrementsStack(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 5, Slot: 5}},
	}, SessionLink{})

	result, ok := runtime.DropInventoryItem(5, 2)
	if !ok {
		t.Fatalf("expected counted drop to be accepted")
	}
	if !result.Changed || !result.FromOccupied || !result.CountOnly || result.FromItem.Count != 3 {
		t.Fatalf("unexpected counted drop result: %+v", result)
	}
	want := []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 3, Slot: 5}}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected inventory after counted drop: got %#v want %#v", got, want)
	}
}

func TestDropInventoryItemRejectsLockedOrOversizedDrop(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 5, Slot: 5, Locked: true}},
	}, SessionLink{})
	before := runtime.LiveInventory()
	if _, ok := runtime.DropInventoryItem(5, 1); ok {
		t.Fatalf("expected locked item drop to be rejected")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("locked drop mutated inventory: got %#v want %#v", got, before)
	}

	runtime = NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 5, Slot: 5}},
	}, SessionLink{})
	before = runtime.LiveInventory()
	if _, ok := runtime.DropInventoryItem(5, 6); ok {
		t.Fatalf("expected oversized item drop to be rejected")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("oversized drop mutated inventory: got %#v want %#v", got, before)
	}
}

func TestDropInventoryItemWithTemplateRejectsAntiDropAndAntiGiveWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
	}{
		{
			name:     "anti drop",
			template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiDrop: true},
		},
		{
			name:     "anti give",
			template: itemcatalog.Template{Vnum: 27001, Name: "Soulbound Potion", Stackable: true, MaxCount: 200, AntiGive: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(loginticket.Character{
				Inventory: []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 5, Slot: 5}},
			}, SessionLink{})
			before := runtime.LiveInventory()

			if _, ok := runtime.DropInventoryItemWithTemplate(5, 1, tc.template); ok {
				t.Fatalf("expected %s item drop to be rejected", tc.name)
			}
			if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
				t.Fatalf("%s drop mutated inventory: got %#v want %#v", tc.name, got, before)
			}
		})
	}
}

func TestDropInventoryItemWithTemplateRejectsMismatchedTemplateWithoutMutation(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 5, Slot: 5}},
	}, SessionLink{})
	before := runtime.LiveInventory()
	template := itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200}

	if _, ok := runtime.DropInventoryItemWithTemplate(5, 1, template); ok {
		t.Fatal("expected item drop to reject template metadata whose vnum does not match the carried item")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("mismatched-template drop mutated inventory: got %#v want %#v", got, before)
	}
}

func TestDropInventoryItemWithTemplateRejectsMalformedTemplateWithoutMutation(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 5, Slot: 5}},
	}, SessionLink{})
	before := runtime.LiveInventory()
	template := itemcatalog.Template{Vnum: 27001, Name: "Broken Potion", Stackable: true, MaxCount: 0}

	if _, ok := runtime.DropInventoryItemWithTemplate(5, 1, template); ok {
		t.Fatal("expected item drop to reject malformed template metadata")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("malformed-template drop mutated inventory: got %#v want %#v", got, before)
	}
}

func TestDropInventoryItemRejectsMalformedLiveItemWithoutMutation(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 1, Vnum: 27001, Count: 5, Slot: 5}},
	}, SessionLink{})
	before := runtime.LiveInventory()

	runtime.liveInventory[0].ID = 0
	if result, ok := runtime.DropInventoryItem(5, 2); ok {
		t.Fatalf("expected malformed drop to be rejected, got %+v", result)
	}
	runtime.liveInventory[0].ID = 1
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("malformed drop mutated inventory: got %#v want %#v", got, before)
	}
}

func TestPickupGroundItemFillsCompatibleStacksBeforePlacingRemainder(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 11, Vnum: 27001, Count: 198, Slot: 2},
			{ID: 12, Vnum: 27001, Count: 199, Slot: 5},
		},
	}, SessionLink{})

	ground := inventory.ItemInstance{ID: 31, Vnum: 27001, Count: 5, Slot: 9}
	result, ok := runtime.PickupGroundItem(ground, 9, 200)
	if !ok {
		t.Fatal("expected compatible ground pickup to fill existing stacks and place the remainder")
	}
	if !result.Split || result.Merged || result.Placed.ID != 31 || result.Placed.Count != 2 || result.Placed.Slot != 9 {
		t.Fatalf("unexpected pickup split result: %+v", result)
	}
	if !reflect.DeepEqual(result.UpdatedItems, []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 200, Slot: 2},
		{ID: 12, Vnum: 27001, Count: 200, Slot: 5},
	}) {
		t.Fatalf("unexpected pickup stack updates: %#v", result.UpdatedItems)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 200, Slot: 2},
		{ID: 12, Vnum: 27001, Count: 200, Slot: 5},
		{ID: 31, Vnum: 27001, Count: 2, Slot: 9},
	}) {
		t.Fatalf("unexpected inventory after split pickup: %#v", runtime.LiveInventory())
	}
}

func TestPickupGroundItemFailsWhenCompatibleStacksCannotFitRemainderAndNoFreshSlotExists(t *testing.T) {
	inventoryItems := make([]inventory.ItemInstance, 0, inventory.CarriedInventorySlotCount)
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		item := inventory.ItemInstance{ID: uint64(slot) + 1, Vnum: 11200, Count: 1, Slot: slot}
		if slot == 2 {
			item = inventory.ItemInstance{ID: uint64(slot) + 1, Vnum: 27001, Count: 198, Slot: slot}
		}
		if slot == 5 {
			item = inventory.ItemInstance{ID: uint64(slot) + 1, Vnum: 27001, Count: 199, Slot: slot}
		}
		inventoryItems = append(inventoryItems, item)
	}
	runtime := NewRuntime(loginticket.Character{Inventory: inventoryItems}, SessionLink{})
	before := runtime.LiveInventory()

	ground := inventory.ItemInstance{ID: 301, Vnum: 27001, Count: 5, Slot: 9}
	if _, ok := runtime.PickupGroundItem(ground, 9, 200); ok {
		t.Fatal("expected split pickup with no fresh slot for the remainder to fail closed")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), before) {
		t.Fatalf("failed split pickup mutated inventory: got %#v want %#v", runtime.LiveInventory(), before)
	}
}

func TestPickupGroundItemRejectsMismatchedTemplateMetadataWithoutMutation(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
	}, SessionLink{})
	before := runtime.LiveInventory()
	template := itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200}

	if _, ok := runtime.PickupGroundItemWithTemplate(inventory.ItemInstance{ID: 31, Vnum: 27001, Count: 2, Slot: 6}, 6, template); ok {
		t.Fatal("expected pickup to reject template metadata whose vnum does not match the ground item")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), before) {
		t.Fatalf("mismatched-template pickup mutated inventory: got %#v want %#v", runtime.LiveInventory(), before)
	}
}

func TestUseItemOnItemConsolidatesFullSourceAndKeepsTargetQuickslotStable(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 41, Vnum: 27001, Count: 2, Slot: 5},
			{ID: 42, Vnum: 27001, Count: 3, Slot: 6},
		},
		Quickslots: []loginticket.Quickslot{
			{Position: 2, Type: 1, Slot: 5},
			{Position: 3, Type: 1, Slot: 6},
			{Position: 4, Type: 2, Slot: 5},
		},
	}, SessionLink{})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	result, ok := runtime.UseItemOnItem(5, 6, template)
	if !ok {
		t.Fatal("expected compatible ITEM_USE_TO_ITEM consolidation to succeed")
	}
	if !result.Changed || result.From != 5 || result.To != 6 || result.FromOccupied || !result.ToOccupied || result.ToItem.ID != 42 || result.ToItem.Count != 5 {
		t.Fatalf("unexpected full consolidation result: %+v", result)
	}
	deletedQuickslots, ok := runtime.SyncItemQuickslotsForItemRemoval(5)
	if !ok {
		t.Fatal("expected removed source item quickslot sync to succeed")
	}
	if !reflect.DeepEqual(deletedQuickslots, []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}) {
		t.Fatalf("unexpected deleted quickslots after source removal: %#v", deletedQuickslots)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{{ID: 42, Vnum: 27001, Count: 5, Slot: 6}}) {
		t.Fatalf("unexpected live inventory after full consolidation: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.LiveQuickslots(), []loginticket.Quickslot{{Position: 3, Type: 1, Slot: 6}, {Position: 4, Type: 2, Slot: 5}}) {
		t.Fatalf("unexpected live quickslots after full consolidation: %#v", runtime.LiveQuickslots())
	}
}

func TestUseItemOnItemPartiallyConsolidatesWithoutRemovingSourceQuickslot(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 41, Vnum: 27001, Count: 5, Slot: 5},
			{ID: 42, Vnum: 27001, Count: 198, Slot: 6},
		},
		Quickslots: []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}},
	}, SessionLink{})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	result, ok := runtime.UseItemOnItem(5, 6, template)
	if !ok {
		t.Fatal("expected partial ITEM_USE_TO_ITEM consolidation to succeed")
	}
	if !result.Changed || result.From != 5 || result.To != 6 || !result.FromOccupied || !result.ToOccupied || !result.CountOnly || result.FromItem.Count != 3 || result.ToItem.Count != 200 {
		t.Fatalf("unexpected partial consolidation result: %+v", result)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 41, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 42, Vnum: 27001, Count: 200, Slot: 6},
	}) {
		t.Fatalf("unexpected live inventory after partial consolidation: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.LiveQuickslots(), []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}}) {
		t.Fatalf("partial consolidation should not remove source quickslot, got %#v", runtime.LiveQuickslots())
	}
}

func TestUseItemOnItemFullConsolidationDoesNotMutateWhenTargetRewriteFails(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 41, Vnum: 27001, Count: 2, Slot: 5},
			{ID: 42, Vnum: 27001, Count: 3, Slot: 6},
		},
	}, SessionLink{})
	before := runtime.LiveInventory()
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if _, ok := runtime.useItemOnItem(5, 6, template, func(item inventory.ItemInstance) inventory.ItemInstance {
		if item.Slot == 6 {
			item.Count = 0
		}
		return item
	}); ok {
		t.Fatal("expected invalid target rewrite to fail full ITEM_USE_TO_ITEM consolidation")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed full consolidation mutated live inventory: got %#v want %#v", got, before)
	}
}

func TestUseItemOnItemPartialConsolidationDoesNotMutateWhenSourceRewriteFails(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 41, Vnum: 27001, Count: 5, Slot: 5},
			{ID: 42, Vnum: 27001, Count: 198, Slot: 6},
		},
	}, SessionLink{})
	before := runtime.LiveInventory()
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if _, ok := runtime.useItemOnItem(5, 6, template, func(item inventory.ItemInstance) inventory.ItemInstance {
		if item.Slot == 5 {
			item.Count = 0
		}
		return item
	}); ok {
		t.Fatal("expected invalid source rewrite to fail partial ITEM_USE_TO_ITEM consolidation")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed partial consolidation mutated live inventory: got %#v want %#v", got, before)
	}
}

func TestUseItemOnItemFailsClosedWhenTargetUpdateValidationFails(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 41, Vnum: 27001, Count: 2, Slot: 5},
			{ID: 42, Vnum: 27001, Count: 1, Slot: 6, EquipSlot: inventory.EquipmentSlotBody},
		},
	}, SessionLink{})
	before := runtime.LiveInventory()
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected malformed target stack to fail closed")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed target validation mutated live inventory: got %#v want %#v", got, before)
	}
}

func TestUseItemOnItemRejectsDuplicateInstanceIDsWithoutMutation(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 41, Vnum: 27001, Count: 2, Slot: 5},
			{ID: 41, Vnum: 27001, Count: 3, Slot: 6},
		},
		Quickslots: []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}},
	}, SessionLink{})
	beforeInventory := runtime.LiveInventory()
	beforeQuickslots := runtime.LiveQuickslots()
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected duplicate item instance IDs to fail closed")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, beforeInventory) {
		t.Fatalf("duplicate-instance use-to-item mutated live inventory: got %#v want %#v", got, beforeInventory)
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, beforeQuickslots) {
		t.Fatalf("duplicate-instance use-to-item mutated quickslots: got %#v want %#v", got, beforeQuickslots)
	}
}

func TestUseItemOnItemFailsClosedWhenSourceRemainderValidationFails(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 41, Vnum: 27001, Count: 5, Slot: 5, EquipSlot: inventory.EquipmentSlotBody},
			{ID: 42, Vnum: 27001, Count: 198, Slot: 6},
		},
	}, SessionLink{})
	before := runtime.LiveInventory()
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected malformed source remainder to fail closed")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed source validation mutated live inventory: got %#v want %#v", got, before)
	}
}

func TestUseItemOnItemFailsClosedWhenFullSourceValidationFails(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{
			{ID: 41, Vnum: 27001, Count: 2, Slot: 5, EquipSlot: inventory.EquipmentSlotBody},
			{ID: 42, Vnum: 27001, Count: 3, Slot: 6},
		},
	}, SessionLink{})
	before := runtime.LiveInventory()
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
		t.Fatal("expected malformed full-source stack to fail closed")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed full-source validation mutated live inventory: got %#v want %#v", got, before)
	}
}

func TestUseItemOnItemRejectsIncompatibleAndGuardedTargetsWithoutMutation(t *testing.T) {
	base := loginticket.Character{Inventory: []inventory.ItemInstance{
		{ID: 41, Vnum: 27001, Count: 2, Slot: 5},
		{ID: 42, Vnum: 27001, Count: 3, Slot: 6},
	}}
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	foreignTemplate := itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200}
	cases := []struct {
		name      string
		character loginticket.Character
		template  itemcatalog.Template
	}{
		{name: "same slot", character: base},
		{name: "empty source", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 42, Vnum: 27001, Count: 3, Slot: 6}}}},
		{name: "empty target", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}}}},
		{name: "different vnum target", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27002, Count: 3, Slot: 6}}}},
		{name: "locked source", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5, Locked: true}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}}},
		{name: "locked target", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6, Locked: true}}}},
		{name: "template vnum mismatch", character: base, template: foreignTemplate},
		{name: "non stackable template", character: base, template: itemcatalog.Template{Vnum: 27001, Name: "Single Potion", Stackable: false, MaxCount: 1}},
		{name: "anti stack template", character: base, template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiStack: true}},
		{name: "anti drop template", character: base, template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiDrop: true}},
		{name: "anti give template", character: base, template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiGive: true}},
		{name: "anti sell template", character: base, template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiSell: true}},
		{name: "zero-count source", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 0, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}}},
		{name: "zero-count target", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 0, Slot: 6}}}},
		{name: "over max source", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 201, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}}},
		{name: "over max target", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 201, Slot: 6}}}},
		{name: "already full target", character: loginticket.Character{Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 200, Slot: 6}}}},
		{name: "template max above refresh count range", character: base, template: itemcatalog.Template{Vnum: 27001, Name: "Huge Stack Potion", Stackable: true, MaxCount: 256}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			character := cloneCharacter(tc.character)
			runtime := NewRuntime(character, SessionLink{})
			beforeInventory := runtime.LiveInventory()
			activeTemplate := tc.template
			if activeTemplate.Vnum == 0 {
				activeTemplate = template
			}
			source := inventory.SlotIndex(5)
			if tc.name == "same slot" {
				source = 6
			}
			if _, ok := runtime.UseItemOnItem(source, 6, activeTemplate); ok {
				t.Fatalf("expected %s ITEM_USE_TO_ITEM consolidation to fail", tc.name)
			}
			if !reflect.DeepEqual(runtime.LiveInventory(), beforeInventory) {
				t.Fatalf("%s mutated inventory: got %#v want %#v", tc.name, runtime.LiveInventory(), beforeInventory)
			}
		})
	}
}

func TestUseItemOnItemRejectsAntiFlagTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
	}{
		{name: "anti sell", template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiSell: true}},
		{name: "anti drop", template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiDrop: true}},
		{name: "anti give", template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiGive: true}},
		{name: "anti stack", template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, AntiStack: true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(loginticket.Character{
				Inventory: []inventory.ItemInstance{
					{ID: 41, Vnum: 27001, Count: 2, Slot: 5},
					{ID: 42, Vnum: 27001, Count: 3, Slot: 6},
				},
				Quickslots: []loginticket.Quickslot{{Position: 2, Type: 1, Slot: 5}},
			}, SessionLink{})
			beforeInventory := runtime.LiveInventory()
			beforeQuickslots := runtime.LiveQuickslots()

			if _, ok := runtime.UseItemOnItem(5, 6, tc.template); ok {
				t.Fatalf("expected %s ITEM_USE_TO_ITEM consolidation to fail closed", tc.name)
			}
			if got := runtime.LiveInventory(); !reflect.DeepEqual(got, beforeInventory) {
				t.Fatalf("%s use-to-item mutated inventory: got %#v want %#v", tc.name, got, beforeInventory)
			}
			if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, beforeQuickslots) {
				t.Fatalf("%s use-to-item mutated quickslots: got %#v want %#v", tc.name, got, beforeQuickslots)
			}
		})
	}
}

func TestMerchantTemplateMutationsRejectSelectedCharacterAntiFlagTemplates(t *testing.T) {
	cases := []struct {
		name      string
		character loginticket.Character
		template  itemcatalog.Template
	}{
		{
			name:      "anti warrior",
			character: loginticket.Character{Job: 0},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiWarrior: true},
		},
		{
			name:      "anti assassin",
			character: loginticket.Character{Job: 1},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiAssassin: true},
		},
		{
			name:      "anti sura",
			character: loginticket.Character{Job: 2},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiSura: true},
		},
		{
			name:      "anti shaman",
			character: loginticket.Character{Job: 3},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiShaman: true},
		},
		{
			name:      "anti male",
			character: loginticket.Character{RaceNum: 0},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiMale: true},
		},
		{
			name:      "anti female",
			character: loginticket.Character{RaceNum: 1},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiFemale: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.character.Gold = 100
			tc.character.Inventory = []inventory.ItemInstance{{ID: 41, Vnum: tc.template.Vnum, Count: 2, Slot: 5}}
			runtime := NewRuntime(tc.character, SessionLink{})
			beforeInventory := runtime.LiveInventory()
			if failure := runtime.ValidateMerchantBuy(tc.template, 1, 50); failure != MerchantBuyFailureInvalid {
				t.Fatalf("expected restricted merchant buy to be invalid, got %q", failure)
			}
			if result, ok := runtime.BuyMerchantItem(tc.template, 1, 50); ok {
				t.Fatalf("expected restricted merchant buy mutation to fail, got %+v", result)
			}
			if result, ok := runtime.SellMerchantItemWithTemplate(5, 1, tc.template); ok {
				t.Fatalf("expected restricted merchant sell mutation to fail, got %+v", result)
			}
			if got := runtime.LiveCharacter().Gold; got != 100 {
				t.Fatalf("restricted merchant mutation changed gold: got %d want 100", got)
			}
			if got := runtime.LiveInventory(); !reflect.DeepEqual(got, beforeInventory) {
				t.Fatalf("restricted merchant mutation changed inventory: got %#v want %#v", got, beforeInventory)
			}
		})
	}
}

func TestUseItemOnItemRejectsSelectedCharacterAntiFlagTemplatesWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		character loginticket.Character
		template  itemcatalog.Template
	}{
		{
			name:      "anti warrior",
			character: loginticket.Character{Job: 0, Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiWarrior: true},
		},
		{
			name:      "anti assassin",
			character: loginticket.Character{Job: 1, Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiAssassin: true},
		},
		{
			name:      "anti sura",
			character: loginticket.Character{Job: 2, Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiSura: true},
		},
		{
			name:      "anti shaman",
			character: loginticket.Character{Job: 3, Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiShaman: true},
		},
		{
			name:      "anti male",
			character: loginticket.Character{RaceNum: 0, Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiMale: true},
		},
		{
			name:      "anti female",
			character: loginticket.Character{RaceNum: 1, Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}, {ID: 42, Vnum: 27001, Count: 3, Slot: 6}}},
			template:  itemcatalog.Template{Vnum: 27001, Name: "Restricted Potion", Stackable: true, MaxCount: 200, AntiFemale: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(tc.character, SessionLink{})
			beforeInventory := runtime.LiveInventory()
			beforeQuickslots := runtime.LiveQuickslots()

			if _, ok := runtime.UseItemOnItem(5, 6, tc.template); ok {
				t.Fatalf("expected %s ITEM_USE_TO_ITEM consolidation to fail closed", tc.name)
			}
			if got := runtime.LiveInventory(); !reflect.DeepEqual(got, beforeInventory) {
				t.Fatalf("%s use-to-item mutated inventory: got %#v want %#v", tc.name, got, beforeInventory)
			}
			if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, beforeQuickslots) {
				t.Fatalf("%s use-to-item mutated quickslots: got %#v want %#v", tc.name, got, beforeQuickslots)
			}
		})
	}
}

func TestUseItemOnItemRejectsOutOfRangeSourceOrTargetWithoutMutation(t *testing.T) {
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	cases := []struct {
		name      string
		character loginticket.Character
		source    inventory.SlotIndex
		target    inventory.SlotIndex
	}{
		{
			name: "source outside carried inventory",
			character: loginticket.Character{Inventory: []inventory.ItemInstance{
				{ID: 41, Vnum: 27001, Count: 2, Slot: inventory.CarriedInventorySlotCount},
				{ID: 42, Vnum: 27001, Count: 3, Slot: 6},
			}},
			source: inventory.CarriedInventorySlotCount,
			target: 6,
		},
		{
			name: "target outside carried inventory",
			character: loginticket.Character{Inventory: []inventory.ItemInstance{
				{ID: 41, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 42, Vnum: 27001, Count: 3, Slot: inventory.CarriedInventorySlotCount},
			}},
			source: 5,
			target: inventory.CarriedInventorySlotCount,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(tc.character, SessionLink{})
			before := runtime.LiveInventory()
			if _, ok := runtime.UseItemOnItem(tc.source, tc.target, template); ok {
				t.Fatalf("expected %s ITEM_USE_TO_ITEM consolidation to fail", tc.name)
			}
			if got := runtime.LiveInventory(); !reflect.DeepEqual(got, before) {
				t.Fatalf("%s mutated live inventory: got %#v want %#v", tc.name, got, before)
			}
		})
	}
}

func TestUseItemOnItemRejectsNilRuntime(t *testing.T) {
	var runtime *Runtime
	if _, ok := runtime.UseItemOnItem(5, 6, itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}); ok {
		t.Fatal("expected nil runtime ITEM_USE_TO_ITEM consolidation to fail")
	}
}

func TestUseItemRejectsOutOfRangeSlotWithoutMutation(t *testing.T) {
	runtime := NewRuntime(loginticket.Character{
		Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: inventory.CarriedInventorySlotCount}},
		Points:    [255]int32{1: 25},
	}, SessionLink{})
	beforeInventory := runtime.LiveInventory()
	beforePoints := runtime.LiveCharacter().Points
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "consume:27001:+50"}}

	if result, ok := runtime.UseItem(inventory.CarriedInventorySlotCount, template); ok {
		t.Fatalf("expected out-of-range ITEM_USE slot to fail closed, got %+v", result)
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, beforeInventory) {
		t.Fatalf("out-of-range ITEM_USE mutated inventory: got %#v want %#v", got, beforeInventory)
	}
	if got := runtime.LiveCharacter().Points; got != beforePoints {
		t.Fatalf("out-of-range ITEM_USE mutated points: got %#v want %#v", got, beforePoints)
	}
}

func TestUseItemRejectsMalformedTemplateUseEffectWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
	}{
		{
			name:     "zero point type",
			template: itemcatalog.Template{Vnum: 27001, Name: "Broken Red Potion", Stackable: true, MaxCount: 200, UseEffect: &itemcatalog.UseEffect{PointType: 0, PointIndex: 1, PointDelta: 50, Message: "consume:27001:+50"}},
		},
		{
			name:     "out-of-range point index",
			template: itemcatalog.Template{Vnum: 27001, Name: "Broken Red Potion", Stackable: true, MaxCount: 200, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 255, PointDelta: 50, Message: "consume:27001:+50"}},
		},
		{
			name:     "zero point delta",
			template: itemcatalog.Template{Vnum: 27001, Name: "Broken Red Potion", Stackable: true, MaxCount: 200, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 0, Message: "consume:27001:+0"}},
		},
		{
			name:     "empty effect message",
			template: itemcatalog.Template{Vnum: 27001, Name: "Broken Red Potion", Stackable: true, MaxCount: 200, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: ""}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(loginticket.Character{
				Inventory: []inventory.ItemInstance{{ID: 41, Vnum: 27001, Count: 2, Slot: 5}},
				Points:    [255]int32{1: 25},
			}, SessionLink{})
			beforeInventory := runtime.LiveInventory()
			beforePoints := runtime.LiveCharacter().Points

			if result, ok := runtime.UseItem(5, tc.template); ok {
				t.Fatalf("expected malformed %s ITEM_USE template to fail closed, got %+v", tc.name, result)
			}
			if got := runtime.LiveInventory(); !reflect.DeepEqual(got, beforeInventory) {
				t.Fatalf("malformed %s ITEM_USE mutated inventory: got %#v want %#v", tc.name, got, beforeInventory)
			}
			if got := runtime.LiveCharacter().Points; got != beforePoints {
				t.Fatalf("malformed %s ITEM_USE mutated points: got %#v want %#v", tc.name, got, beforePoints)
			}
		})
	}
}

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
	equippedItem, ok := runtime.EquipItem(8, inventory.EquipmentSlotBody)
	if !ok {
		t.Fatal("expected equip to succeed")
	}
	if !equippedItem.Equipped || equippedItem.EquipSlot != inventory.EquipmentSlotBody || equippedItem.Slot != 0 || equippedItem.ID != 12 {
		t.Fatalf("unexpected equipped item result: %+v", equippedItem)
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

func TestRuntimeAccessorsDeepCopyLivePersistedAndQuickslotState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Quickslots = []loginticket.Quickslot{{Position: 3, Type: 1, Slot: 5}}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	persistedSnapshot := runtime.PersistedSnapshot()
	liveInventory := runtime.LiveInventory()
	liveEquipment := runtime.LiveEquipment()
	liveQuickslots := runtime.LiveQuickslots()

	persistedSnapshot.Inventory[0].Count = 99
	persistedSnapshot.Quickslots[0].Slot = 9
	liveInventory[0].Count = 77
	liveEquipment[0].Vnum = 9999
	liveQuickslots[0].Slot = 8

	if got := runtime.PersistedSnapshot().Inventory[0].Count; got != 3 {
		t.Fatalf("expected persisted inventory count to stay 3, got %d", got)
	}
	if got := runtime.PersistedSnapshot().Quickslots[0].Slot; got != 5 {
		t.Fatalf("expected persisted quickslot slot to stay 5, got %d", got)
	}
	if got := runtime.LiveInventory()[0].Count; got != 3 {
		t.Fatalf("expected live inventory count to stay 3, got %d", got)
	}
	if got := runtime.LiveEquipment()[0].Vnum; got != 19 {
		t.Fatalf("expected live equipment vnum to stay 19, got %d", got)
	}
	if got := runtime.LiveQuickslots()[0].Slot; got != 5 {
		t.Fatalf("expected live quickslot slot to stay 5, got %d", got)
	}
}

func TestRuntimeCanSetDeleteAndSwapQuickslots(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{ID: 13, Vnum: 27002, Count: 1, Slot: 6})
	persisted.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 5},
		{Position: 7, Type: 2, Slot: 9},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	set, ok := runtime.SetQuickslot(4, loginticket.Quickslot{Type: 1, Slot: 6})
	if !ok {
		t.Fatal("expected quickslot set to succeed")
	}
	if set.Position != 4 || set.Type != 1 || set.Slot != 6 {
		t.Fatalf("unexpected set quickslot result: %+v", set)
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 5},
		{Position: 4, Type: 1, Slot: 6},
		{Position: 7, Type: 2, Slot: 9},
	}) {
		t.Fatalf("unexpected live quickslots after set: %#v", got)
	}

	duplicate, ok := runtime.SetQuickslot(8, loginticket.Quickslot{Type: 1, Slot: 6})
	if !ok || duplicate.Position != 8 {
		t.Fatalf("expected duplicate quickslot target to move to position 8, got %+v ok=%v", duplicate, ok)
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 5},
		{Position: 7, Type: 2, Slot: 9},
		{Position: 8, Type: 1, Slot: 6},
	}) {
		t.Fatalf("unexpected live quickslots after duplicate move: %#v", got)
	}

	deleted, ok := runtime.DeleteQuickslot(3)
	if !ok || deleted.Position != 3 {
		t.Fatalf("expected quickslot delete to return position 3, got %+v ok=%v", deleted, ok)
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 7, Type: 2, Slot: 9},
		{Position: 8, Type: 1, Slot: 6},
	}) {
		t.Fatalf("unexpected live quickslots after delete: %#v", got)
	}

	swap, ok := runtime.SwapQuickslots(8, 7)
	if !ok || swap.Position != 8 || swap.TargetPosition != 7 {
		t.Fatalf("expected quickslot swap 8<->7, got %+v ok=%v", swap, ok)
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 7, Type: 1, Slot: 6},
		{Position: 8, Type: 2, Slot: 9},
	}) {
		t.Fatalf("unexpected live quickslots after swap: %#v", got)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Quickslots, persisted.Quickslots) {
		t.Fatalf("expected persisted quickslots to remain unchanged, got %#v", runtime.PersistedSnapshot().Quickslots)
	}
}

func TestRuntimeSetQuickslotRejectsOutOfRangeSkillAndCommandSlots(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Quickslots = []loginticket.Quickslot{{Position: 7, Type: 2, Slot: 9}}
	invalid := []struct {
		name string
		slot loginticket.Quickslot
	}{
		{name: "skill", slot: loginticket.Quickslot{Type: 2, Slot: 255}},
		{name: "command", slot: loginticket.Quickslot{Type: 3, Slot: 255}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			before := runtime.LiveQuickslots()
			if _, ok := runtime.SetQuickslot(3, tc.slot); ok {
				t.Fatalf("expected out-of-range %s quickslot to fail closed", tc.name)
			}
			if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, before) {
				t.Fatalf("rejected %s quickslot mutated live state: got %#v want %#v", tc.name, got, before)
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Quickslots, persisted.Quickslots) {
				t.Fatalf("rejected %s quickslot mutated persisted state: got %#v want %#v", tc.name, runtime.PersistedSnapshot().Quickslots, persisted.Quickslots)
			}
		})
	}
}

func TestRuntimeSetQuickslotRejectsInvalidInputs(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	invalid := []struct {
		name     string
		position uint8
		slot     loginticket.Quickslot
	}{
		{name: "quickslot position", position: 36, slot: loginticket.Quickslot{Type: 1, Slot: 5}},
		{name: "type", position: 3, slot: loginticket.Quickslot{Type: 4, Slot: 5}},
		{name: "item slot", position: 3, slot: loginticket.Quickslot{Type: 1, Slot: 90}},
		{name: "missing item", position: 3, slot: loginticket.Quickslot{Type: 1, Slot: 6}},
		{name: "nil runtime", position: 3, slot: loginticket.Quickslot{Type: 1, Slot: 5}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			if tc.name == "nil runtime" {
				runtime = nil
			}
			if _, ok := runtime.SetQuickslot(tc.position, tc.slot); ok {
				t.Fatalf("expected invalid %s quickslot to fail closed", tc.name)
			}
			if runtime != nil {
				if got := runtime.LiveQuickslots(); len(got) != 0 {
					t.Fatalf("expected rejected %s quickslot to leave live state empty, got %#v", tc.name, got)
				}
			}
		})
	}
}

func TestRuntimeQuickslotDeletionAndSwapRejectInvalidInputsWithoutMutation(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 5},
		{Position: 7, Type: 2, Slot: 9},
	}

	t.Run("delete invalid position", func(t *testing.T) {
		runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
		before := runtime.LiveQuickslots()
		if _, ok := runtime.DeleteQuickslot(36); ok {
			t.Fatal("expected invalid quickslot delete to fail closed")
		}
		if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, before) {
			t.Fatalf("invalid quickslot delete mutated live state: got %#v want %#v", got, before)
		}
	})

	t.Run("delete nil runtime", func(t *testing.T) {
		var runtime *Runtime
		if _, ok := runtime.DeleteQuickslot(3); ok {
			t.Fatal("expected nil runtime quickslot delete to fail closed")
		}
	})

	t.Run("swap invalid source position", func(t *testing.T) {
		runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
		before := runtime.LiveQuickslots()
		if _, ok := runtime.SwapQuickslots(36, 7); ok {
			t.Fatal("expected invalid quickslot source swap to fail closed")
		}
		if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, before) {
			t.Fatalf("invalid source quickslot swap mutated live state: got %#v want %#v", got, before)
		}
	})

	t.Run("swap invalid target position", func(t *testing.T) {
		runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
		before := runtime.LiveQuickslots()
		if _, ok := runtime.SwapQuickslots(3, 36); ok {
			t.Fatal("expected invalid quickslot target swap to fail closed")
		}
		if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, before) {
			t.Fatalf("invalid target quickslot swap mutated live state: got %#v want %#v", got, before)
		}
	})

	t.Run("swap nil runtime", func(t *testing.T) {
		var runtime *Runtime
		if _, ok := runtime.SwapQuickslots(3, 7); ok {
			t.Fatal("expected nil runtime quickslot swap to fail closed")
		}
	})
}

func TestRuntimeSyncItemQuickslotsForInventoryMoveUpdatesMatchingItemSlots(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 5},
		{Position: 4, Type: 2, Slot: 5},
		{Position: 7, Type: 1, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	changed, deleted, ok := runtime.SyncItemQuickslotsForInventoryMove(5, 6)
	if !ok {
		t.Fatal("expected item quickslot sync to succeed")
	}
	if !reflect.DeepEqual(changed, []loginticket.Quickslot{{Position: 3, Type: 1, Slot: 6}}) {
		t.Fatalf("unexpected changed quickslots: %#v", changed)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected no deleted quickslots, got %#v", deleted)
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 6},
		{Position: 4, Type: 2, Slot: 5},
		{Position: 7, Type: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live quickslots after item sync: %#v", got)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Quickslots, persisted.Quickslots) {
		t.Fatalf("expected persisted quickslots to remain unchanged, got %#v", runtime.PersistedSnapshot().Quickslots)
	}
}

func TestRuntimeSyncItemQuickslotsForInventoryMoveDeletesOnlyDestinationItemSlots(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Quickslots = []loginticket.Quickslot{
		{Position: 6, Type: 1, Slot: 8},
		{Position: 4, Type: 1, Slot: 6},
		{Position: 3, Type: 1, Slot: 5},
		{Position: 5, Type: 2, Slot: 6},
		{Position: 7, Type: 1, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	changed, deleted, ok := runtime.SyncItemQuickslotsForInventoryMove(5, 6)
	if !ok {
		t.Fatal("expected item quickslot sync to succeed")
	}
	if !reflect.DeepEqual(changed, []loginticket.Quickslot{{Position: 3, Type: 1, Slot: 6}}) {
		t.Fatalf("unexpected changed quickslots: %#v", changed)
	}
	if !reflect.DeepEqual(deleted, []loginticket.Quickslot{{Position: 4, Type: 1, Slot: 6}}) {
		t.Fatalf("unexpected deleted quickslots: %#v", deleted)
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 6},
		{Position: 5, Type: 2, Slot: 6},
		{Position: 6, Type: 1, Slot: 8},
		{Position: 7, Type: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live quickslots after item sync: %#v", got)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Quickslots, persisted.Quickslots) {
		t.Fatalf("expected persisted quickslots to remain unchanged, got %#v", runtime.PersistedSnapshot().Quickslots)
	}
}

func TestRuntimeSyncItemQuickslotsForItemRemovalDeletesMatchingItemSlots(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 5},
		{Position: 4, Type: 2, Slot: 5},
		{Position: 7, Type: 1, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	deleted, ok := runtime.SyncItemQuickslotsForItemRemoval(5)
	if !ok {
		t.Fatal("expected item quickslot removal sync to succeed")
	}
	if !reflect.DeepEqual(deleted, []loginticket.Quickslot{{Position: 3, Type: 1, Slot: 5}}) {
		t.Fatalf("unexpected deleted quickslots: %#v", deleted)
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, []loginticket.Quickslot{
		{Position: 4, Type: 2, Slot: 5},
		{Position: 7, Type: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live quickslots after item removal sync: %#v", got)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Quickslots, persisted.Quickslots) {
		t.Fatalf("expected persisted quickslots to remain unchanged, got %#v", runtime.PersistedSnapshot().Quickslots)
	}
}

func TestRuntimeSyncItemQuickslotsForItemRemovalRejectsInvalidSkillOrCommandCellsWithoutMutation(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Quickslots = []loginticket.Quickslot{
		{Position: 3, Type: 1, Slot: 5},
		{Position: 4, Type: 2, Slot: 5},
		{Position: 8, Type: 3, Slot: 5},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	for _, slot := range []inventory.SlotIndex{inventory.CarriedInventorySlotCount, inventory.CarriedInventorySlotCount + 1, ^inventory.SlotIndex(0)} {
		deleted, ok := runtime.SyncItemQuickslotsForItemRemoval(slot)
		if ok {
			t.Fatalf("expected invalid removal slot %d to fail closed, got deleted=%#v", slot, deleted)
		}
	}
	if got := runtime.LiveQuickslots(); !reflect.DeepEqual(got, persisted.Quickslots) {
		t.Fatalf("invalid removal slots should not mutate live quickslots, got %#v want %#v", got, persisted.Quickslots)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Quickslots, persisted.Quickslots) {
		t.Fatalf("invalid removal slots should not mutate persisted quickslots, got %#v", runtime.PersistedSnapshot().Quickslots)
	}
}

func TestRuntimeApplyPersistedSnapshotRealignsLiveCurrencyInventoryAndEquipment(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.MoveInventoryItem(5, 6); !ok {
		t.Fatal("expected inventory move to succeed")
	}
	equippedItem, ok := runtime.EquipItem(8, inventory.EquipmentSlotBody)
	if !ok {
		t.Fatal("expected equip to succeed")
	}
	if !equippedItem.Equipped || equippedItem.EquipSlot != inventory.EquipmentSlotBody || equippedItem.ID != 12 {
		t.Fatalf("unexpected equipped item result: %+v", equippedItem)
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

	item, ok := runtime.UnequipItem(inventory.EquipmentSlotWeapon, 4)
	if !ok {
		t.Fatal("expected unequip to succeed")
	}
	if item.Equipped || item.EquipSlot != inventory.EquipmentSlotNone || item.Slot != 4 || item.ID != 21 {
		t.Fatalf("unexpected unequipped item result: %+v", item)
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

func TestRuntimeMoveInventoryItemRejectsSameSlotMovesWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.MoveInventoryItem(5, 5); ok {
		t.Fatal("expected same-slot full-stack inventory move to fail closed")
	}
	if _, ok := runtime.MoveInventoryItemBounded(5, 5, 200); ok {
		t.Fatal("expected same-slot bounded inventory move to fail closed")
	}
	if _, ok := runtime.MoveInventoryItemCount(5, 5, 1); ok {
		t.Fatal("expected same-slot counted inventory move to fail closed")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected same-slot move attempts to leave live inventory unchanged, got %#v want %#v", runtime.LiveInventory(), persisted.Inventory)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected same-slot move attempts to leave persisted inventory unchanged, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeMoveInventoryItemSwapsIncompatibleOccupiedDestinationWithoutCount(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.MoveInventoryItem(5, 8)
	if !ok {
		t.Fatal("expected incompatible occupied destination without count to swap full stacks")
	}
	if !result.Changed || !result.FromOccupied || !result.ToOccupied || result.CountOnly {
		t.Fatalf("expected occupied-destination swap to refresh both slots, got %+v", result)
	}
	if result.FromItem.ID != 12 || result.FromItem.Vnum != 1120 || result.FromItem.Count != 1 || result.FromItem.Slot != 5 {
		t.Fatalf("expected destination item to move into source slot, got %+v", result.FromItem)
	}
	if result.ToItem.ID != 11 || result.ToItem.Vnum != 27001 || result.ToItem.Count != 3 || result.ToItem.Slot != 8 {
		t.Fatalf("expected source item to move into destination slot, got %+v", result.ToItem)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 12, Vnum: 1120, Count: 1, Slot: 5},
		{ID: 11, Vnum: 27001, Count: 3, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after occupied-destination swap: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeMoveInventoryItemBoundedZeroCountMergesCompatibleOccupiedDestination(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 198, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.MoveInventoryItemBounded(5, 8, 200)
	if !ok {
		t.Fatal("expected zero-count compatible occupied destination move to merge up to template max")
	}
	if !result.Changed || !result.CountOnly || !result.FromOccupied || !result.ToOccupied {
		t.Fatalf("expected zero-count bounded merge to refresh both counts, got %+v", result)
	}
	if result.FromItem.ID != 11 || result.FromItem.Count != 1 || result.FromItem.Slot != 5 {
		t.Fatalf("expected source remainder of one at slot 5, got %+v", result.FromItem)
	}
	if result.ToItem.ID != 12 || result.ToItem.Count != 200 || result.ToItem.Slot != 8 {
		t.Fatalf("expected destination capped at 200 in slot 8, got %+v", result.ToItem)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 200, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after zero-count bounded merge: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after zero-count bounded merge, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsZeroCountStacksWithoutMutatingState(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name: "zero-count source",
			inventory: []inventory.ItemInstance{
				{ID: 1801, Vnum: 27001, Count: 0, Slot: 5},
				{ID: 1802, Vnum: 27001, Count: 3, Slot: 8},
			},
		},
		{
			name: "zero-count target",
			inventory: []inventory.ItemInstance{
				{ID: 1801, Vnum: 27001, Count: 2, Slot: 5},
				{ID: 1802, Vnum: 27001, Count: 0, Slot: 8},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := inventoryRuntimeCharacterFixture()
			persisted.Inventory = tc.inventory
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

			if _, ok := runtime.UseItemOnItem(5, 8, template); ok {
				t.Fatalf("expected %s use-to-item stack consolidation to fail closed", tc.name)
			}
			if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
				t.Fatalf("expected %s use-to-item to leave live inventory unchanged, got %#v want %#v", tc.name, runtime.LiveInventory(), persisted.Inventory)
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
				t.Fatalf("expected %s use-to-item to leave persisted inventory unchanged, got %#v want %#v", tc.name, runtime.PersistedSnapshot().Inventory, persisted.Inventory)
			}
		})
	}
}

func TestRuntimeMoveInventoryItemCountSplitsPartialStackIntoEmptySlot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.MoveInventoryItemCount(5, 6, 2)
	if !ok {
		t.Fatal("expected partial stack move count into empty carried slot to succeed")
	}
	if !result.Changed || !result.FromOccupied || !result.ToOccupied {
		t.Fatalf("expected split result to refresh both source and destination slots, got %+v", result)
	}
	if result.From != 5 || result.To != 6 {
		t.Fatalf("unexpected split slots: %+v", result)
	}
	if result.FromItem.ID != 11 || result.FromItem.Vnum != 27001 || result.FromItem.Count != 1 || result.FromItem.Slot != 5 {
		t.Fatalf("expected source stack to retain one item at slot 5, got %+v", result.FromItem)
	}
	if result.ToItem.ID == 0 || result.ToItem.ID == result.FromItem.ID || result.ToItem.Vnum != 27001 || result.ToItem.Count != 2 || result.ToItem.Slot != 6 {
		t.Fatalf("expected split stack clone with a fresh instance id at slot 6, got %+v", result.ToItem)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 1, Slot: 5},
		{ID: result.ToItem.ID, Vnum: 27001, Count: 2, Slot: 6},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after partial split: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after partial split, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeUseItemOnItemMergesCompatibleStacksWithTemplateGuards(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 31, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 32, Vnum: 27001, Count: 198, Slot: 6},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Template Potion", Stackable: true, MaxCount: 200}

	result, ok := runtime.UseItemOnItem(5, 6, template)
	if !ok {
		t.Fatal("expected use-to-item stack consolidation to succeed")
	}
	if !result.Changed || !result.FromOccupied || !result.ToOccupied || !result.CountOnly {
		t.Fatalf("expected partial use-to-item consolidation to refresh both stacks, got %+v", result)
	}
	if result.FromItem != (inventory.ItemInstance{ID: 31, Vnum: 27001, Count: 1, Slot: 5}) {
		t.Fatalf("unexpected source remainder after use-to-item: %+v", result.FromItem)
	}
	if result.ToItem != (inventory.ItemInstance{ID: 32, Vnum: 27001, Count: 200, Slot: 6}) {
		t.Fatalf("unexpected target stack after use-to-item: %+v", result.ToItem)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 31, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 32, Vnum: 27001, Count: 200, Slot: 6},
	}) {
		t.Fatalf("unexpected live inventory after use-to-item: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after use-to-item runtime mutation, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeUseItemOnItemRejectsTemplateGuardEdgesWithoutMutation(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
	}{
		{
			name:     "non-stackable",
			template: itemcatalog.Template{Vnum: 27001, Name: "Single Potion", Stackable: false, MaxCount: 1},
		},
		{
			name:     "anti-stack",
			template: itemcatalog.Template{Vnum: 27001, Name: "Bound Stack", Stackable: true, MaxCount: 200, AntiStack: true},
		},
		{
			name:     "anti-drop",
			template: itemcatalog.Template{Vnum: 27001, Name: "No Drop Stack", Stackable: true, MaxCount: 200, AntiDrop: true},
		},
		{
			name:     "anti-give",
			template: itemcatalog.Template{Vnum: 27001, Name: "No Give Stack", Stackable: true, MaxCount: 200, AntiGive: true},
		},
		{
			name:     "over-uint8 max count",
			template: itemcatalog.Template{Vnum: 27001, Name: "Wide Stack", Stackable: true, MaxCount: 300},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := inventoryRuntimeCharacterFixture()
			persisted.Inventory = []inventory.ItemInstance{
				{ID: 41, Vnum: 27001, Count: 3, Slot: 5},
				{ID: 42, Vnum: 27001, Count: 3, Slot: 6},
			}
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			if _, ok := runtime.UseItemOnItem(5, 6, tc.template); ok {
				t.Fatalf("expected %s template use-to-item to fail closed", tc.name)
			}
			if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
				t.Fatalf("expected %s rejection to leave live inventory unchanged, got %#v", tc.name, runtime.LiveInventory())
			}
			if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
				t.Fatalf("expected %s rejection to leave persisted inventory unchanged, got %#v", tc.name, runtime.PersistedSnapshot().Inventory)
			}
		})
	}
}

func TestRuntimeUseItemOnItemRejectsLockedSourceOrTargetWithoutMutation(t *testing.T) {
	cases := []struct {
		name      string
		inventory []inventory.ItemInstance
	}{
		{
			name:      "locked source",
			inventory: []inventory.ItemInstance{{ID: 51, Vnum: 27001, Count: 3, Slot: 5, Locked: true}, {ID: 52, Vnum: 27001, Count: 3, Slot: 6}},
		},
		{
			name:      "locked target",
			inventory: []inventory.ItemInstance{{ID: 51, Vnum: 27001, Count: 3, Slot: 5}, {ID: 52, Vnum: 27001, Count: 3, Slot: 6, Locked: true}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := inventoryRuntimeCharacterFixture()
			persisted.Inventory = append([]inventory.ItemInstance(nil), tc.inventory...)
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			template := itemcatalog.Template{Vnum: 27001, Name: "Template Potion", Stackable: true, MaxCount: 200}
			if _, ok := runtime.UseItemOnItem(5, 6, template); ok {
				t.Fatalf("expected %s use-to-item to fail closed", tc.name)
			}
			if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
				t.Fatalf("expected %s rejection to leave live inventory unchanged, got %#v", tc.name, runtime.LiveInventory())
			}
		})
	}
}

func TestRuntimeMoveInventoryItemCountMergesPartialStackIntoCompatibleDestination(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 2, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.MoveInventoryItemCount(5, 8, 2)
	if !ok {
		t.Fatal("expected partial stack move count into compatible occupied destination to succeed")
	}
	if !result.Changed || !result.FromOccupied || !result.ToOccupied {
		t.Fatalf("expected merge result to refresh both slots, got %+v", result)
	}
	if result.FromItem.ID != 11 || result.FromItem.Vnum != 27001 || result.FromItem.Count != 1 || result.FromItem.Slot != 5 {
		t.Fatalf("expected source stack to retain one item at slot 5, got %+v", result.FromItem)
	}
	if result.ToItem.ID != 12 || result.ToItem.Vnum != 27001 || result.ToItem.Count != 4 || result.ToItem.Slot != 8 {
		t.Fatalf("expected destination stack to grow in slot 8, got %+v", result.ToItem)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 4, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after partial merge: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after partial merge, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeMoveInventoryItemCountBoundedMergesExactFullStackIntoCompatibleDestination(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 2, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.MoveInventoryItemCountBounded(5, 8, 3, 200)
	if !ok {
		t.Fatal("expected exact counted full-stack merge into compatible destination to succeed")
	}
	if !result.Changed || !result.CountOnly || result.FromOccupied || !result.ToOccupied {
		t.Fatalf("expected exact counted merge to delete source and update destination as count-only result, got %+v", result)
	}
	if result.From != 5 || result.To != 8 {
		t.Fatalf("unexpected exact counted merge slots: %+v", result)
	}
	if result.ToItem.ID != 12 || result.ToItem.Vnum != 27001 || result.ToItem.Count != 5 || result.ToItem.Slot != 8 {
		t.Fatalf("expected destination stack to absorb source count, got %+v", result.ToItem)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 12, Vnum: 27001, Count: 5, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after exact counted full-stack merge: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after exact counted full-stack merge, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeMoveInventoryItemCountBoundedRejectsDestinationAboveTemplateMaxWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: 199, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.MoveInventoryItemCountBounded(5, 8, 2, 200); ok {
		t.Fatal("expected partial stack merge above template max_count to fail closed")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected template-bounded merge rejection to leave live inventory unchanged, got %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected template-bounded merge rejection to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}

	result, ok := runtime.MoveInventoryItemCountBounded(5, 8, 1, 200)
	if !ok {
		t.Fatal("expected partial stack merge up to template max_count to succeed")
	}
	if result.FromItem.Count != 2 || result.ToItem.Count != 200 {
		t.Fatalf("unexpected bounded merge result: %+v", result)
	}
}

func TestRuntimeMoveInventoryItemCountRejectsOverflowingDestinationStackWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 27001, Count: ^uint16(0), Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.MoveInventoryItemCount(5, 8, 1); ok {
		t.Fatal("expected partial stack merge into overflowing destination to fail closed")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected overflowing merge rejection to leave live inventory unchanged, got %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected overflowing merge rejection to leave persisted inventory unchanged, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeMoveInventoryItemCountRejectsIncompatibleOccupiedDestinationAndOversizedStackMoves(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.MoveInventoryItemCount(5, 8, 2); ok {
		t.Fatal("expected partial stack move count into incompatible occupied destination to fail closed until swap-with-count semantics are owned")
	}
	result, ok := runtime.MoveInventoryItemCount(5, 8, 3)
	if !ok {
		t.Fatal("expected exact counted full-stack move into incompatible occupied destination to behave as full-stack move")
	}
	if !result.Changed || !result.FromOccupied || !result.ToOccupied || result.FromItem.ID != 12 || result.FromItem.Slot != 5 || result.ToItem.ID != 11 || result.ToItem.Slot != 8 {
		t.Fatalf("unexpected exact counted incompatible-destination full-stack move result: %+v", result)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 12, Vnum: 1120, Count: 1, Slot: 5},
		{ID: 11, Vnum: 27001, Count: 3, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after exact counted incompatible-destination full-stack move: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after exact counted incompatible-destination full-stack move, got %#v", runtime.PersistedSnapshot().Inventory)
	}

	persisted = inventoryRuntimeCharacterFixture()
	runtime = NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	if _, ok := runtime.MoveInventoryItemCount(5, 6, 4); ok {
		t.Fatal("expected oversized stack move count to fail closed")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected oversized counted move to leave live inventory unchanged, got %#v", runtime.LiveInventory())
	}

	result, ok = runtime.MoveInventoryItemCount(5, 6, 3)
	if !ok {
		t.Fatal("expected exact counted full-stack move into empty destination to succeed")
	}
	if !result.Changed || result.ToItem.Slot != 6 || result.ToItem.Count != 3 {
		t.Fatalf("unexpected exact counted move result: %+v", result)
	}
}

func TestRuntimeRejectsLockedInventoryItemMutationWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{{ID: 31, Vnum: 27001, Count: 3, Slot: 5, Locked: true}}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 50, Message: "used"}}

	if _, ok := runtime.MoveInventoryItem(5, 6); ok {
		t.Fatal("expected locked carried-slot item to reject inventory move")
	}
	if _, ok := runtime.MoveInventoryItemCount(5, 6, 3); ok {
		t.Fatal("expected locked carried-slot item to reject counted inventory move")
	}
	if _, ok := runtime.EquipItem(5, inventory.EquipmentSlotBody); ok {
		t.Fatal("expected locked carried-slot item to reject equip")
	}
	if _, ok := runtime.UseItem(5, template); ok {
		t.Fatal("expected locked carried-slot item to reject use")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected locked item mutation attempts to leave live inventory unchanged, got %#v want %#v", runtime.LiveInventory(), persisted.Inventory)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after locked item mutation attempts, got %#v", runtime.PersistedSnapshot().Inventory)
	}
}

func TestRuntimeEquipItemWithTemplateRejectsMismatchedAuthoredSlotWithoutMutatingState(t *testing.T) {
	character := loginticket.Character{
		ID:        1,
		Name:      "TemplateGuard",
		Inventory: []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}},
	}
	runtime := NewRuntime(character, SessionLink{Login: "template-guard", CharacterIndex: 0})
	template := itemcatalog.Template{Vnum: 0x11223344, Name: "Practice Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotWeapon.String()}

	if _, ok := runtime.EquipItemWithTemplate(8, inventory.EquipmentSlotBody, template); ok {
		t.Fatal("expected mismatched authored equip slot to reject equip")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, character.Inventory) {
		t.Fatalf("expected inventory unchanged after mismatched template equip, got %#v want %#v", got, character.Inventory)
	}
	if got := runtime.LiveEquipment(); len(got) != 0 {
		t.Fatalf("expected no live equipment after mismatched template equip, got %#v", got)
	}
}

func TestRuntimeEquipItemWithTemplateAcceptsMatchingAuthoredSlot(t *testing.T) {
	character := loginticket.Character{
		ID:        1,
		Name:      "TemplateGuard",
		Inventory: []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}},
	}
	runtime := NewRuntime(character, SessionLink{Login: "template-guard", CharacterIndex: 0})
	template := itemcatalog.Template{Vnum: 0x11223344, Name: "Practice Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String()}

	equipped, ok := runtime.EquipItemWithTemplate(8, inventory.EquipmentSlotBody, template)
	if !ok {
		t.Fatal("expected matching authored equip slot to allow equip")
	}
	if equipped.Vnum != 0x11223344 || !equipped.Equipped || equipped.EquipSlot != inventory.EquipmentSlotBody {
		t.Fatalf("unexpected equipped item after matching template equip: %#v", equipped)
	}
	if got := runtime.LiveInventory(); len(got) != 0 {
		t.Fatalf("expected source inventory item removed after matching template equip, got %#v", got)
	}
}

func TestRuntimeEquipItemWithTemplateRejectsSelectedCharacterAntiFlagsWithoutMutatingState(t *testing.T) {
	character := loginticket.Character{
		ID:        1,
		Name:      "TemplateGuard",
		Job:       0,
		RaceNum:   0,
		Inventory: []inventory.ItemInstance{{ID: 1001, Vnum: 0x11223344, Count: 1, Slot: 8}},
	}
	cases := []struct {
		name      string
		template  itemcatalog.Template
		equipSlot inventory.EquipmentSlot
	}{
		{
			name:      "job restricted",
			template:  itemcatalog.Template{Vnum: 0x11223344, Name: "Warrior-Banned Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String(), AntiWarrior: true},
			equipSlot: inventory.EquipmentSlotBody,
		},
		{
			name:      "sex restricted",
			template:  itemcatalog.Template{Vnum: 0x11223344, Name: "Male-Banned Armor", Stackable: false, MaxCount: 1, EquipSlot: inventory.EquipmentSlotBody.String(), AntiMale: true},
			equipSlot: inventory.EquipmentSlotBody,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime := NewRuntime(character, SessionLink{Login: "template-guard", CharacterIndex: 0})
			if _, ok := runtime.EquipItemWithTemplate(8, tc.equipSlot, tc.template); ok {
				t.Fatalf("expected %s authored equip template to fail closed", tc.name)
			}
			if got := runtime.LiveInventory(); !reflect.DeepEqual(got, character.Inventory) {
				t.Fatalf("expected inventory unchanged after %s template equip, got %#v want %#v", tc.name, got, character.Inventory)
			}
			if got := runtime.LiveEquipment(); len(got) != 0 {
				t.Fatalf("expected no live equipment after %s template equip, got %#v", tc.name, got)
			}
		})
	}
}

func TestRuntimeRejectsLockedEquippedItemUnequipWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = nil
	persisted.Equipment = []inventory.ItemInstance{{ID: 41, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon, Locked: true}}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.UnequipItem(inventory.EquipmentSlotWeapon, 4); ok {
		t.Fatal("expected locked equipped item to reject unequip")
	}
	if !reflect.DeepEqual(runtime.LiveEquipment(), persisted.Equipment) {
		t.Fatalf("expected locked equipped item attempt to leave live equipment unchanged, got %#v want %#v", runtime.LiveEquipment(), persisted.Equipment)
	}
	if len(runtime.LiveInventory()) != 0 {
		t.Fatalf("expected locked equipped item attempt to leave live inventory empty, got %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Equipment, persisted.Equipment) {
		t.Fatalf("expected persisted equipment to stay unchanged after locked equipped item attempt, got %#v", runtime.PersistedSnapshot().Equipment)
	}
}

func TestRuntimeBuyMerchantItemDoesNotMergeIntoLockedCompatibleStack(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory[0].Locked = true
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if failure := runtime.ValidateMerchantBuy(template, 2, 50); failure != "" {
		t.Fatalf("expected locked compatible stack to be skipped in favor of fresh placement, got failure %q", failure)
	}
	result, ok := runtime.BuyMerchantItem(template, 2, 50)
	if !ok {
		t.Fatal("expected merchant buy to allocate fresh slot when only compatible stack is locked")
	}
	if len(result.ItemChanges) != 1 || !result.ItemChanges[0].Created || result.ItemChanges[0].Item.Slot != 0 || result.ItemChanges[0].Item.Count != 2 {
		t.Fatalf("expected fresh-slot placement instead of locked-stack merge, got %+v", result.ItemChanges)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 22, Vnum: 27001, Count: 2, Slot: 0},
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5, Locked: true},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after locked-stack-skipping merchant buy: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after locked-stack-skipping merchant buy, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeBuyMerchantItemMergesIntoExistingCompatibleStackBeforeAllocatingNewSlot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.BuyMerchantItem(itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}, 2, 50)
	if !ok {
		t.Fatal("expected merchant buy merge to succeed")
	}
	if result.Gold != 124950 {
		t.Fatalf("expected merged merchant buy to debit gold to 124950, got %d", result.Gold)
	}
	if len(result.Items) != 1 || result.Items[0].ID != 11 || result.Items[0].Vnum != 27001 || result.Items[0].Count != 5 || result.Items[0].Slot != 5 {
		t.Fatalf("unexpected merged merchant buy items result: %+v", result.Items)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after merged merchant buy: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after merged merchant buy, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeBuyMerchantItemAntiStackTemplateUsesFreshSlotInsteadOfMerging(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, AntiStack: true}

	if failure := runtime.ValidateMerchantBuy(template, 2, 50); failure != "" {
		t.Fatalf("expected anti-stack merchant buy to use a fresh slot, got validation failure %q", failure)
	}
	result, ok := runtime.BuyMerchantItem(template, 2, 50)
	if !ok {
		t.Fatal("expected anti-stack merchant buy to succeed through fresh-slot placement")
	}
	if len(result.ItemChanges) != 1 || !result.ItemChanges[0].Created || result.ItemChanges[0].Item.Slot != 0 || result.ItemChanges[0].Item.Count != 2 {
		t.Fatalf("expected anti-stack grant to create a fresh stack instead of merging, got %+v", result.ItemChanges)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 22, Vnum: 27001, Count: 2, Slot: 0},
		{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after anti-stack fresh-slot merchant buy: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after anti-stack merchant buy, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeValidateMerchantBuyAntiStackTemplateRejectsFullInventoryWithoutFreshSlot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		item := inventory.ItemInstance{ID: 1000 + uint64(slot), Vnum: 40000 + uint32(slot), Count: 1, Slot: slot}
		if slot == 5 {
			item = inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 3, Slot: slot}
		}
		persisted.Inventory = append(persisted.Inventory, item)
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, AntiStack: true}

	if failure := runtime.ValidateMerchantBuy(template, 2, 50); failure != MerchantBuyFailureNoValidPlacement {
		t.Fatalf("expected anti-stack merchant buy without a free slot to fail placement, got %q", failure)
	}
	if _, ok := runtime.BuyMerchantItem(template, 2, 50); ok {
		t.Fatal("expected anti-stack merchant buy without a free slot to fail")
	}
	if got := runtime.LiveInventory(); !reflect.DeepEqual(got, persisted.Inventory) {
		t.Fatalf("expected anti-stack failed buy to leave live inventory unchanged, got %#v want %#v", got, persisted.Inventory)
	}
}

func TestRuntimeBuyMerchantItemPartiallyMergesThenUsesFreshSlot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory[0].Count = 199
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.BuyMerchantItem(itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}, 2, 50)
	if !ok {
		t.Fatal("expected merchant buy partial-merge path to succeed")
	}
	if result.Gold != 124950 {
		t.Fatalf("expected partial-merge merchant buy to debit gold to 124950, got %d", result.Gold)
	}
	if len(result.Items) != 2 || result.Items[0].ID != 22 || result.Items[0].Vnum != 27001 || result.Items[0].Count != 1 || result.Items[0].Slot != 0 || result.Items[1].ID != 11 || result.Items[1].Vnum != 27001 || result.Items[1].Count != 200 || result.Items[1].Slot != 5 {
		t.Fatalf("unexpected partial-merge merchant buy items result: %+v", result.Items)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 22, Vnum: 27001, Count: 1, Slot: 0},
		{ID: 11, Vnum: 27001, Count: 200, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after partial-merge merchant buy: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after partial-merge merchant buy, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeBuyMerchantItemFansOutAcrossSeveralExistingCompatibleStacksWithoutFreshSlot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		item := inventory.ItemInstance{ID: 1000 + uint64(slot), Vnum: 40000 + uint32(slot), Count: 1, Slot: slot}
		switch slot {
		case 5:
			item = inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 199, Slot: slot}
		case 7:
			item = inventory.ItemInstance{ID: 79, Vnum: 27001, Count: 199, Slot: slot}
		}
		persisted.Inventory = append(persisted.Inventory, item)
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if failure := runtime.ValidateMerchantBuy(template, 2, 50); failure != "" {
		t.Fatalf("expected distributed-stack merchant buy validation to succeed, got %q", failure)
	}
	result, ok := runtime.BuyMerchantItem(template, 2, 50)
	if !ok {
		t.Fatal("expected distributed-stack merchant buy to succeed without a fresh slot")
	}
	if result.Gold != 124950 {
		t.Fatalf("expected distributed-stack merchant buy to debit gold to 124950, got %d", result.Gold)
	}
	if len(result.Items) != 2 || result.Items[0].ID != 77 || result.Items[0].Vnum != 27001 || result.Items[0].Count != 200 || result.Items[0].Slot != 5 || result.Items[1].ID != 79 || result.Items[1].Vnum != 27001 || result.Items[1].Count != 200 || result.Items[1].Slot != 7 {
		t.Fatalf("unexpected distributed-stack merchant buy items result: %+v", result.Items)
	}
	if gotLive := runtime.LiveInventory(); !reflect.DeepEqual(gotLive[5], inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 200, Slot: 5}) || !reflect.DeepEqual(gotLive[7], inventory.ItemInstance{ID: 79, Vnum: 27001, Count: 200, Slot: 7}) {
		t.Fatalf("unexpected live inventory after distributed-stack merchant buy at slots 5/7: %#v", gotLive)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after distributed-stack merchant buy, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeBuyMerchantItemFansOutOnlyAcrossUnlockedCompatibleStacks(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		item := inventory.ItemInstance{ID: 1000 + uint64(slot), Vnum: 40000 + uint32(slot), Count: 1, Slot: slot}
		switch slot {
		case 5:
			item = inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 199, Slot: slot}
		case 7:
			item = inventory.ItemInstance{ID: 79, Vnum: 27001, Count: 199, Slot: slot, Locked: true}
		case 9:
			item = inventory.ItemInstance{ID: 81, Vnum: 27001, Count: 199, Slot: slot}
		}
		persisted.Inventory = append(persisted.Inventory, item)
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if failure := runtime.ValidateMerchantBuy(template, 2, 50); failure != "" {
		t.Fatalf("expected locked partial stack to be skipped during fan-out validation, got %q", failure)
	}
	result, ok := runtime.BuyMerchantItem(template, 2, 50)
	if !ok {
		t.Fatal("expected merchant buy to fan out across unlocked stacks while skipping locked stack")
	}
	if len(result.Items) != 2 || result.Items[0].ID != 77 || result.Items[0].Count != 200 || result.Items[0].Slot != 5 || result.Items[1].ID != 81 || result.Items[1].Count != 200 || result.Items[1].Slot != 9 {
		t.Fatalf("expected fan-out to skip locked slot 7 and fill slots 5/9, got %+v", result.Items)
	}
	gotLive := runtime.LiveInventory()
	if !reflect.DeepEqual(gotLive[5], inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 200, Slot: 5}) || !reflect.DeepEqual(gotLive[7], inventory.ItemInstance{ID: 79, Vnum: 27001, Count: 199, Slot: 7, Locked: true}) || !reflect.DeepEqual(gotLive[9], inventory.ItemInstance{ID: 81, Vnum: 27001, Count: 200, Slot: 9}) {
		t.Fatalf("unexpected live inventory after locked-stack-skipping fan-out: %#v", gotLive)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) {
		t.Fatalf("expected persisted inventory to stay unchanged after locked-stack-skipping fan-out, got %#v want %#v", runtime.PersistedSnapshot().Inventory, persisted.Inventory)
	}
}

func TestRuntimeBuyMerchantItemFansOutAcrossSeveralExistingCompatibleStacksThenUsesFreshSlot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 77, Vnum: 27001, Count: 199, Slot: 5},
		{ID: 79, Vnum: 27001, Count: 198, Slot: 7},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if failure := runtime.ValidateMerchantBuy(template, 4, 200); failure != "" {
		t.Fatalf("expected distributed-stack-plus-fresh merchant buy validation to succeed, got %q", failure)
	}
	result, ok := runtime.BuyMerchantItem(template, 4, 200)
	if !ok {
		t.Fatal("expected distributed-stack-plus-fresh merchant buy to succeed")
	}
	if result.Gold != 124800 {
		t.Fatalf("expected distributed-stack-plus-fresh merchant buy to debit gold to 124800, got %d", result.Gold)
	}
	if len(result.Items) != 3 || result.Items[0].ID != 80 || result.Items[0].Vnum != 27001 || result.Items[0].Count != 1 || result.Items[0].Slot != 0 || result.Items[1].ID != 77 || result.Items[1].Vnum != 27001 || result.Items[1].Count != 200 || result.Items[1].Slot != 5 || result.Items[2].ID != 79 || result.Items[2].Vnum != 27001 || result.Items[2].Count != 200 || result.Items[2].Slot != 7 {
		t.Fatalf("unexpected distributed-stack-plus-fresh merchant buy items result: %+v", result.Items)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 80, Vnum: 27001, Count: 1, Slot: 0},
		{ID: 77, Vnum: 27001, Count: 200, Slot: 5},
		{ID: 79, Vnum: 27001, Count: 200, Slot: 7},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after distributed-stack-plus-fresh merchant buy: %#v", runtime.LiveInventory())
	}
}

func TestRuntimeBuyMerchantItemMergesIntoLowestCompatibleSlot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{
		{ID: 51, Vnum: 27001, Count: 3, Slot: 9},
		{ID: 52, Vnum: 27001, Count: 4, Slot: 2},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.BuyMerchantItem(itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}, 1, 50)
	if !ok {
		t.Fatal("expected merchant buy lowest-slot merge to succeed")
	}
	if len(result.Items) != 1 || result.Items[0].ID != 52 || result.Items[0].Vnum != 27001 || result.Items[0].Count != 5 || result.Items[0].Slot != 2 {
		t.Fatalf("unexpected lowest-slot merge result: %+v", result.Items)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 52, Vnum: 27001, Count: 5, Slot: 2},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
		{ID: 51, Vnum: 27001, Count: 3, Slot: 9},
	}) {
		t.Fatalf("unexpected live inventory after lowest-slot merge: %#v", runtime.LiveInventory())
	}
}

func TestRuntimeValidateMerchantBuyRejectsInsufficientGoldWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if failure := runtime.ValidateMerchantBuy(template, 1, 125001); failure != MerchantBuyFailureInsufficientGold {
		t.Fatalf("expected insufficient-gold merchant buy failure, got %q", failure)
	}
	if _, ok := runtime.BuyMerchantItem(template, 1, 125001); ok {
		t.Fatal("expected insufficient-gold merchant buy to fail")
	}
	if got := runtime.LiveGold(); got != 125000 {
		t.Fatalf("expected live gold to stay 125000 after insufficient-gold validation, got %d", got)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected live inventory to stay unchanged after insufficient-gold validation, got %#v want %#v", runtime.LiveInventory(), persisted.Inventory)
	}
}

func TestRuntimeValidateMerchantBuyRejectsNoValidPlacementWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		item := inventory.ItemInstance{ID: 1000 + uint64(slot), Vnum: 40000 + uint32(slot), Count: 1, Slot: slot}
		if slot == 5 {
			item = inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 199, Slot: slot}
		}
		persisted.Inventory = append(persisted.Inventory, item)
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if failure := runtime.ValidateMerchantBuy(template, 2, 50); failure != MerchantBuyFailureNoValidPlacement {
		t.Fatalf("expected no-valid-placement merchant buy failure, got %q", failure)
	}
	if _, ok := runtime.BuyMerchantItem(template, 2, 50); ok {
		t.Fatal("expected no-valid-placement merchant buy to fail")
	}
	if got := runtime.LiveGold(); got != 125000 {
		t.Fatalf("expected live gold to stay 125000 after no-placement validation, got %d", got)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected live inventory to stay unchanged after no-placement validation, got %#v want %#v", runtime.LiveInventory(), persisted.Inventory)
	}
}

func TestRuntimeValidateMerchantBuyRejectsDistributedRemainderWithoutFreshSlot(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = make([]inventory.ItemInstance, 0, int(inventory.CarriedInventorySlotCount))
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		item := inventory.ItemInstance{ID: 2000 + uint64(slot), Vnum: 50000 + uint32(slot), Count: 1, Slot: slot}
		switch slot {
		case 5:
			item = inventory.ItemInstance{ID: 77, Vnum: 27001, Count: 199, Slot: slot}
		case 7:
			item = inventory.ItemInstance{ID: 79, Vnum: 27001, Count: 198, Slot: slot}
		}
		persisted.Inventory = append(persisted.Inventory, item)
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	if failure := runtime.ValidateMerchantBuy(template, 4, 200); failure != MerchantBuyFailureNoValidPlacement {
		t.Fatalf("expected distributed remainder without fresh-slot merchant buy failure, got %q", failure)
	}
	if _, ok := runtime.BuyMerchantItem(template, 4, 200); ok {
		t.Fatal("expected distributed remainder without fresh-slot merchant buy to fail")
	}
	if got := runtime.LiveGold(); got != 125000 {
		t.Fatalf("expected live gold to stay 125000 after distributed remainder without fresh-slot validation, got %d", got)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected live inventory to stay unchanged after distributed remainder without fresh-slot validation, got %#v want %#v", runtime.LiveInventory(), persisted.Inventory)
	}
}

func TestRuntimeSellMerchantItemRemovesWholeStackAndCreditsGold(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.SellMerchantItem(5, 0, 10)
	if !ok {
		t.Fatal("expected whole-stack merchant sell to succeed")
	}
	if !result.ItemRemoved || result.Slot != 5 || result.Gold != 125030 {
		t.Fatalf("unexpected whole-stack sell result: %+v", result)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after whole-stack sell: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) || runtime.PersistedSnapshot().Gold != persisted.Gold {
		t.Fatalf("expected persisted state to stay unchanged after merchant sell, got %+v", runtime.PersistedSnapshot())
	}
}

func TestRuntimeSellMerchantItemDecrementsPartialStackAndCreditsGold(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	result, ok := runtime.SellMerchantItem(5, 2, 10)
	if !ok {
		t.Fatal("expected partial-stack merchant sell to succeed")
	}
	if result.ItemRemoved || result.Slot != 5 || result.Item.ID != 11 || result.Item.Count != 1 || result.Gold != 125020 {
		t.Fatalf("unexpected partial-stack sell result: %+v", result)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after partial-stack sell: %#v", runtime.LiveInventory())
	}
}

func TestMerchantSellUnitPriceFromTemplateUsesLegacyFloorAfterTax(t *testing.T) {
	price, ok := MerchantSellUnitPrice(itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 500})
	if !ok {
		t.Fatal("expected sell unit price to resolve from item template shop-buy price")
	}
	if price != 97 {
		t.Fatalf("expected legacy-compatible unit sell price 97 for shop-buy price 500, got %d", price)
	}
}

func TestMerchantSellCreditForCountPerGoldTemplateUsesLegacyCountDivision(t *testing.T) {
	credit, ok := MerchantSellCredit(itemcatalog.Template{Vnum: 80001, Name: "Bundle", Stackable: true, MaxCount: 200, ShopBuyPrice: 5, SellCountPerGold: true, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 1, Message: "metadata"}}, 25)
	if !ok {
		t.Fatal("expected count-per-gold sell credit to resolve")
	}
	if credit != 1 {
		t.Fatalf("expected count-per-gold credit 1 after legacy count division, /5 floor, and tax, got %d", credit)
	}

	_, ok = MerchantSellCredit(itemcatalog.Template{Vnum: 80001, Name: "Bundle", Stackable: true, MaxCount: 200, ShopBuyPrice: 5, SellCountPerGold: true, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 1, Message: "metadata"}}, 12)
	if ok {
		t.Fatal("expected small count-per-gold sell credit to fail closed after legacy floor/tax")
	}

	credit, ok = MerchantSellCredit(itemcatalog.Template{Vnum: 80002, Name: "Raw Bundle", Stackable: true, MaxCount: 200, SellCountPerGold: true, UseEffect: &itemcatalog.UseEffect{PointType: 1, PointIndex: 1, PointDelta: 1, Message: "metadata"}}, 12)
	if !ok {
		t.Fatal("expected zero-price count-per-gold sell credit to use sold count")
	}
	if credit != 2 {
		t.Fatalf("expected zero-price count-per-gold credit 2 after legacy floor/tax, got %d", credit)
	}
}

func TestMerchantSellCreditRejectsAntiSellTemplate(t *testing.T) {
	_, ok := MerchantSellCredit(itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 500, AntiSell: true}, 3)
	if ok {
		t.Fatal("expected anti-sell template to reject merchant sell credit")
	}
}

func TestRuntimeSellMerchantItemWithTemplateRejectsAntiSellAndMismatchedTemplateWithoutMutatingState(t *testing.T) {
	cases := []struct {
		name     string
		template itemcatalog.Template
	}{
		{
			name:     "anti sell",
			template: itemcatalog.Template{Vnum: 27001, Name: "Bound Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 500, AntiSell: true},
		},
		{
			name:     "template vnum mismatch",
			template: itemcatalog.Template{Vnum: 27002, Name: "Small Blue Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 500},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			persisted := inventoryRuntimeCharacterFixture()
			runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
			beforeInventory := runtime.LiveInventory()
			beforeGold := runtime.LiveGold()

			if _, ok := runtime.SellMerchantItemWithTemplate(5, 0, tc.template); ok {
				t.Fatalf("expected %s template-backed merchant sell to fail", tc.name)
			}
			if got := runtime.LiveGold(); got != beforeGold {
				t.Fatalf("%s template-backed sell mutated gold: got %d want %d", tc.name, got, beforeGold)
			}
			if got := runtime.LiveInventory(); !reflect.DeepEqual(got, beforeInventory) {
				t.Fatalf("%s template-backed sell mutated inventory: got %#v want %#v", tc.name, got, beforeInventory)
			}
		})
	}
}

func TestRuntimeSellMerchantItemWithTemplateUsesTemplateCredit(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200, ShopBuyPrice: 500}

	result, ok := runtime.SellMerchantItemWithTemplate(5, 2, template)
	if !ok {
		t.Fatal("expected template-backed merchant sell to succeed")
	}
	if result.ItemRemoved || result.Item.Count != 1 || result.GoldBefore != 125000 || result.Gold != 125194 {
		t.Fatalf("unexpected template-backed merchant sell result: %+v", result)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 1, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after template-backed merchant sell: %#v", runtime.LiveInventory())
	}
}

func TestRuntimeSellMerchantItemRejectsInvalidInputWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.SellMerchantItem(42, 1, 10); ok {
		t.Fatal("expected absent-slot merchant sell to fail")
	}
	if _, ok := runtime.SellMerchantItem(5, 1, 0); ok {
		t.Fatal("expected zero unit-price merchant sell to fail")
	}
	if _, ok := runtime.SellMerchantItem(5, 4, 10); ok {
		t.Fatal("expected over-count merchant sell to fail")
	}
	if _, ok := runtime.MerchantSellCount(5, 4); ok {
		t.Fatal("expected over-count merchant sell count resolution to fail")
	}
	if got := runtime.LiveGold(); got != persisted.Gold {
		t.Fatalf("expected live gold to stay unchanged after invalid sell attempts, got %d", got)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected live inventory to stay unchanged after invalid sell attempts, got %#v want %#v", runtime.LiveInventory(), persisted.Inventory)
	}
}

func TestRuntimeSellMerchantItemRejectsCarriedSlotMarkedEquippedWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{{ID: 31, Vnum: 27001, Count: 3, Slot: 5, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.SellMerchantItem(5, 1, 10); ok {
		t.Fatal("expected carried-slot item marked equipped to fail merchant sell")
	}
	if _, ok := runtime.MerchantSellCount(5, 1); ok {
		t.Fatal("expected carried-slot item marked equipped to fail merchant sell count resolution")
	}
	if got := runtime.LiveGold(); got != persisted.Gold {
		t.Fatalf("expected live gold to stay unchanged after equipped carried-slot sell attempt, got %d", got)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected live inventory to stay unchanged after equipped carried-slot sell attempt, got %#v want %#v", runtime.LiveInventory(), persisted.Inventory)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) || runtime.PersistedSnapshot().Gold != persisted.Gold {
		t.Fatalf("expected persisted state to stay unchanged after equipped carried-slot sell attempt, got %+v", runtime.PersistedSnapshot())
	}
}

func TestRuntimeSellMerchantItemRejectsLockedCarriedSlotWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = []inventory.ItemInstance{{ID: 31, Vnum: 27001, Count: 3, Slot: 5, Locked: true}}
	runtime := NewRuntime(persisted, SessionLink{Login: "peer-two", CharacterIndex: 1})

	if _, ok := runtime.SellMerchantItem(5, 1, 10); ok {
		t.Fatal("expected locked carried-slot item to fail merchant sell")
	}
	if _, ok := runtime.MerchantSellCount(5, 1); ok {
		t.Fatal("expected locked carried-slot item to fail merchant sell count resolution")
	}
	if got := runtime.LiveGold(); got != persisted.Gold {
		t.Fatalf("expected live gold to stay unchanged after locked carried-slot sell attempt, got %d", got)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), persisted.Inventory) {
		t.Fatalf("expected live inventory to stay unchanged after locked carried-slot sell attempt, got %#v want %#v", runtime.LiveInventory(), persisted.Inventory)
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, persisted.Inventory) || runtime.PersistedSnapshot().Gold != persisted.Gold {
		t.Fatalf("expected persisted state to stay unchanged after locked carried-slot sell attempt, got %+v", runtime.PersistedSnapshot())
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
	if _, ok := runtime.EquipItem(1, inventory.EquipmentSlotBody); ok {
		t.Fatal("expected nil runtime equip to fail")
	}
	if _, ok := runtime.UnequipItem(inventory.EquipmentSlotWeapon, 2); ok {
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
