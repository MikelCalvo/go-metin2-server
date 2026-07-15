package accountstore

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
)

func TestFileStoreRejectsZeroCountInventoryItem(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:        1,
			Name:      "MkmkWar",
			Inventory: []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 0, Slot: 8}},
		}},
	}

	if err := store.Save(account); err == nil {
		t.Fatal("expected zero-count inventory item snapshot to be rejected")
	}
}

func TestFileStoreLoadRejectsZeroCountInventoryItem(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[{"id":1001,"vnum":27001,"count":0,"slot":8}],"equipment":[],"quickslots":[]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write invalid zero-count account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for loaded zero-count inventory item, got %v", err)
	}
}

func TestFileStoreRejectsDuplicateEquipmentSlots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:   1,
			Name: "MkmkWar",
			Equipment: []inventory.ItemInstance{
				{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
				{ID: 2002, Vnum: 29, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
			},
		}},
	}

	if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate equipment slot, got %v", err)
	}
}

func TestFileStoreLoadRejectsDuplicateEquipmentSlots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[],"equipment":[{"id":2001,"vnum":19,"count":1,"equipped":true,"equip_slot":2},{"id":2002,"vnum":29,"count":1,"equipped":true,"equip_slot":2}],"quickslots":[]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write duplicate-equipment account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate equipment slot, got %v", err)
	}
}

func TestFileStoreRejectsDuplicateQuickslotPositions(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:   1,
			Name: "MkmkWar",
			Quickslots: []loginticket.Quickslot{
				{Position: 3, Type: 1, Slot: 8},
				{Position: 3, Type: 2, Slot: 9},
			},
		}},
	}

	if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate quickslot position, got %v", err)
	}
}

func TestFileStoreLoadRejectsDuplicateQuickslotPositions(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[],"equipment":[],"quickslots":[{"position":3,"type":1,"slot":8},{"position":3,"type":2,"slot":9}]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write duplicate-quickslot account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for duplicate quickslot position, got %v", err)
	}
}

func TestFileStoreRejectsInvalidQuickslotTuples(t *testing.T) {
	store := NewFileStore(t.TempDir())
	cases := []struct {
		name      string
		quickslot loginticket.Quickslot
	}{
		{name: "position out of range", quickslot: loginticket.Quickslot{Position: 36, Type: 1, Slot: 8}},
		{name: "unknown type", quickslot: loginticket.Quickslot{Position: 3, Type: 9, Slot: 8}},
		{name: "item slot out of range", quickslot: loginticket.Quickslot{Position: 3, Type: 1, Slot: 180}},
		{name: "skill slot out of range", quickslot: loginticket.Quickslot{Position: 3, Type: 2, Slot: 200}},
		{name: "command slot out of range", quickslot: loginticket.Quickslot{Position: 3, Type: 3, Slot: 60}},
		{name: "none keeps stale slot", quickslot: loginticket.Quickslot{Position: 3, Type: 0, Slot: 8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{Login: "mkmk", Empire: 2, Characters: []loginticket.Character{{ID: 1, Name: "MkmkWar", Quickslots: []loginticket.Quickslot{tc.quickslot}}}}
			if err := store.Save(account); !errors.Is(err, ErrInvalidAccount) {
				t.Fatalf("expected ErrInvalidAccount, got %v", err)
			}
		})
	}
}

func TestFileStoreLoadAllowsDuplicateCarriedSlotsForRuntimeRecovery(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[{"id":1,"name":"MkmkWar","inventory":[{"id":1001,"vnum":27001,"count":5,"slot":8},{"id":1002,"vnum":27002,"count":3,"slot":8}],"equipment":[],"quickslots":[]}]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write duplicate-slot account snapshot: %v", err)
	}

	got, err := store.Load("mkmk")
	if err != nil {
		t.Fatalf("expected duplicate carried-slot snapshot to remain loadable for runtime recovery tests, got %v", err)
	}
	if len(got.Characters) != 1 || len(got.Characters[0].Inventory) != 2 {
		t.Fatalf("unexpected duplicate-slot account load result: %#v", got)
	}
}

func TestFileStoreLoadRejectsUnknownAccountFields(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"schema_version":99,"characters":[]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write unknown-field account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for unknown account field, got %v", err)
	}
}

func TestFileStoreLoadRejectsTrailingJSONValue(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","empire":2,"characters":[]} {"login":"shadow","empire":1,"characters":[]}`)
	if err := os.WriteFile(store.accountPath("mkmk"), raw, 0o644); err != nil {
		t.Fatalf("write trailing-json account snapshot: %v", err)
	}

	_, err := store.Load("mkmk")
	if !errors.Is(err, ErrInvalidAccount) {
		t.Fatalf("expected ErrInvalidAccount for trailing JSON value, got %v", err)
	}
}

func TestFileStoreSaveThenLoadRoundTrip(t *testing.T) {
	store := NewFileStore(t.TempDir())
	want := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:         1,
			VID:        0x01020304,
			Name:       "MkmkWar",
			Job:        0,
			RaceNum:    0,
			Level:      15,
			ST:         6,
			HT:         5,
			DX:         4,
			IQ:         3,
			MainPart:   101,
			HairPart:   201,
			X:          1000,
			Y:          2000,
			MapIndex:   21,
			Empire:     2,
			SkillGroup: 1,
			Gold:       88000,
			Inventory: []inventory.ItemInstance{
				{ID: 1001, Vnum: 27001, Count: 5, Slot: 8, Locked: true},
			},
			Equipment: []inventory.ItemInstance{
				{ID: 2002, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
			},
			Quickslots: []loginticket.Quickslot{
				{Position: 3, Type: 1, Slot: 8},
			},
		}},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("save account: %v", err)
	}
	got, err := store.Load("mkmk")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected account:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestFileStoreLoadReturnsNotFoundForUnknownLogin(t *testing.T) {
	store := NewFileStore(t.TempDir())
	_, err := store.Load("ghost")
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestFileStoreLoadNormalizesMissingItemStateFromLegacySnapshot(t *testing.T) {
	store := NewFileStore(t.TempDir())
	legacyRaw := []byte("{\"login\":\"mkmk\",\"empire\":2,\"characters\":[{\"id\":1,\"name\":\"MkmkWar\"}]}")
	if err := os.WriteFile(store.accountPath("mkmk"), legacyRaw, 0o644); err != nil {
		t.Fatalf("write legacy account: %v", err)
	}

	got, err := store.Load("mkmk")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if len(got.Characters) != 1 {
		t.Fatalf("expected one character, got %d", len(got.Characters))
	}
	character := got.Characters[0]
	if character.Gold != 0 {
		t.Fatalf("expected zero gold, got %d", character.Gold)
	}
	if character.Inventory == nil {
		t.Fatal("expected legacy inventory to normalize to an empty slice, got nil")
	}
	if len(character.Inventory) != 0 {
		t.Fatalf("expected empty inventory, got %#v", character.Inventory)
	}
	if character.Equipment == nil {
		t.Fatal("expected legacy equipment to normalize to an empty slice, got nil")
	}
	if len(character.Equipment) != 0 {
		t.Fatalf("expected empty equipment, got %#v", character.Equipment)
	}
	if character.Quickslots == nil {
		t.Fatal("expected legacy quickslots to normalize to an empty slice, got nil")
	}
	if len(character.Quickslots) != 0 {
		t.Fatalf("expected empty quickslots, got %#v", character.Quickslots)
	}
}

func TestFileStoreSaveDoesNotMutateCallerItemState(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:   1,
			Name: "MkmkWar",
		}},
	}

	if err := store.Save(account); err != nil {
		t.Fatalf("save account: %v", err)
	}
	if account.Characters[0].Inventory != nil {
		t.Fatalf("expected caller inventory slice to remain nil, got %#v", account.Characters[0].Inventory)
	}
	if account.Characters[0].Equipment != nil {
		t.Fatalf("expected caller equipment slice to remain nil, got %#v", account.Characters[0].Equipment)
	}
	if account.Characters[0].Quickslots != nil {
		t.Fatalf("expected caller quickslots slice to remain nil, got %#v", account.Characters[0].Quickslots)
	}
}

func TestFileStoreSavePersistsEmptyItemStateAsArrays(t *testing.T) {
	store := NewFileStore(t.TempDir())
	account := Account{
		Login:  "mkmk",
		Empire: 2,
		Characters: []loginticket.Character{{
			ID:   1,
			Name: "MkmkWar",
		}},
	}
	if err := store.Save(account); err != nil {
		t.Fatalf("save account: %v", err)
	}

	raw, err := os.ReadFile(store.accountPath("mkmk"))
	if err != nil {
		t.Fatalf("read account file: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "\"gold\":0") {
		t.Fatalf("expected explicit zero gold field, got %s", text)
	}
	if !strings.Contains(text, "\"inventory\":[]") {
		t.Fatalf("expected empty inventory array, got %s", text)
	}
	if !strings.Contains(text, "\"equipment\":[]") {
		t.Fatalf("expected empty equipment array, got %s", text)
	}
	if !strings.Contains(text, "\"quickslots\":[]") {
		t.Fatalf("expected empty quickslots array, got %s", text)
	}
}
