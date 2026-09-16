package player

import (
	"reflect"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
)

func TestParseMailGrantCommandRecognizesMailAndLetterForms(t *testing.T) {
	cases := []struct {
		name       string
		message    string
		wantVnum   uint32
		wantCount  uint16
		wantParsed bool
		wantRecog  bool
	}{
		{name: "mail default count", message: "/mail_grant 27001", wantVnum: 27001, wantCount: 1, wantParsed: true, wantRecog: true},
		{name: "letter explicit count", message: "/letter_grant 27001 3", wantVnum: 27001, wantCount: 3, wantParsed: true, wantRecog: true},
		{name: "missing args", message: "/mail_grant", wantRecog: true},
		{name: "zero vnum", message: "/mail_grant 0", wantRecog: true},
		{name: "zero count", message: "/letter_grant 27001 0", wantRecog: true},
		{name: "non numeric", message: "/mail_grant potion", wantRecog: true},
		{name: "extra args", message: "/mail_grant 27001 1 extra", wantRecog: true},
		{name: "ordinary talk", message: "mail_grant 27001"},
		{name: "other slash", message: "/use_item 5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vnum, count, parsed, recognized := ParseMailGrantCommand(tc.message)
			if recognized != tc.wantRecog || parsed != tc.wantParsed || vnum != tc.wantVnum || count != tc.wantCount {
				t.Fatalf("ParseMailGrantCommand(%q)=%d %d parsed=%v recognized=%v, want %d %d parsed=%v recognized=%v",
					tc.message, vnum, count, parsed, recognized, tc.wantVnum, tc.wantCount, tc.wantParsed, tc.wantRecog)
			}
		})
	}
}

func TestRuntimeGrantCarriedMailItemPlacesIntoFirstFreeSlotWithoutGoldMutation(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = nil
	runtime := NewRuntime(persisted, SessionLink{Login: "mail-grant", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	beforePersistedInventory := runtime.PersistedSnapshot().Inventory

	result, ok := runtime.GrantCarriedMailItem(template, 1)
	if !ok {
		t.Fatal("expected mail grant to succeed")
	}
	if len(result.ItemChanges) != 1 || !result.ItemChanges[0].Created || result.ItemChanges[0].Item.Vnum != 27001 || result.ItemChanges[0].Item.Count != 1 || result.ItemChanges[0].Item.Slot != 0 {
		t.Fatalf("unexpected mail grant changes: %+v", result.ItemChanges)
	}
	if runtime.LiveGold() != persisted.Gold {
		t.Fatalf("expected mail grant to leave gold unchanged, got %d want %d", runtime.LiveGold(), persisted.Gold)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{{ID: 22, Vnum: 27001, Count: 1, Slot: 0}}) {
		t.Fatalf("unexpected live inventory after mail grant: %#v", runtime.LiveInventory())
	}
	if !reflect.DeepEqual(runtime.PersistedSnapshot().Inventory, beforePersistedInventory) {
		t.Fatalf("expected persisted inventory to stay unchanged until session commit, got %#v want %#v", runtime.PersistedSnapshot().Inventory, beforePersistedInventory)
	}
}

func TestRuntimeGrantCarriedMailItemMergesIntoExistingCompatibleStack(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	runtime := NewRuntime(persisted, SessionLink{Login: "mail-grant-merge", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}

	result, ok := runtime.GrantCarriedMailItem(template, 2)
	if !ok {
		t.Fatal("expected mail grant merge to succeed")
	}
	if len(result.ItemChanges) != 1 || result.ItemChanges[0].Created || result.ItemChanges[0].Item.Slot != 5 || result.ItemChanges[0].Item.Count != 5 {
		t.Fatalf("expected mail grant to merge into slot 5, got %+v", result.ItemChanges)
	}
	if runtime.LiveGold() != persisted.Gold {
		t.Fatalf("expected merged mail grant to leave gold unchanged, got %d want %d", runtime.LiveGold(), persisted.Gold)
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), []inventory.ItemInstance{
		{ID: 11, Vnum: 27001, Count: 5, Slot: 5},
		{ID: 12, Vnum: 1120, Count: 1, Slot: 8},
	}) {
		t.Fatalf("unexpected live inventory after merged mail grant: %#v", runtime.LiveInventory())
	}
}

func TestRuntimeGrantCarriedMailItemRejectsNoValidPlacementWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = make([]inventory.ItemInstance, 0, inventory.CarriedInventorySlotCount)
	for slot := inventory.SlotIndex(0); slot < inventory.CarriedInventorySlotCount; slot++ {
		persisted.Inventory = append(persisted.Inventory, inventory.ItemInstance{ID: uint64(slot) + 1, Vnum: 1120, Count: 1, Slot: slot})
	}
	runtime := NewRuntime(persisted, SessionLink{Login: "mail-grant-full", CharacterIndex: 1})
	template := itemcatalog.Template{Vnum: 27001, Name: "Small Red Potion", Stackable: true, MaxCount: 200}
	before := runtime.LiveInventory()
	beforeGold := runtime.LiveGold()

	if _, ok := runtime.GrantCarriedMailItem(template, 1); ok {
		t.Fatal("expected full inventory mail grant to fail closed")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), before) {
		t.Fatalf("full inventory mail grant mutated inventory: got %#v want %#v", runtime.LiveInventory(), before)
	}
	if runtime.LiveGold() != beforeGold {
		t.Fatalf("full inventory mail grant mutated gold: got %d want %d", runtime.LiveGold(), beforeGold)
	}
}

func TestRuntimeGrantCarriedMailItemRejectsInvalidTemplateWithoutMutatingState(t *testing.T) {
	persisted := inventoryRuntimeCharacterFixture()
	persisted.Inventory = nil
	runtime := NewRuntime(persisted, SessionLink{Login: "mail-grant-invalid", CharacterIndex: 1})
	before := runtime.LiveInventory()
	beforeGold := runtime.LiveGold()
	template := itemcatalog.Template{Vnum: 27001, Name: "Bound Mail Potion", Stackable: true, MaxCount: 200, AntiGet: true}

	if _, ok := runtime.GrantCarriedMailItem(template, 1); ok {
		t.Fatal("expected anti-get mail grant to fail closed")
	}
	if !reflect.DeepEqual(runtime.LiveInventory(), before) {
		t.Fatalf("invalid mail grant mutated inventory: got %#v want %#v", runtime.LiveInventory(), before)
	}
	if runtime.LiveGold() != beforeGold {
		t.Fatalf("invalid mail grant mutated gold: got %d want %d", runtime.LiveGold(), beforeGold)
	}
}
