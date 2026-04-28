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
				{ID: 1001, Vnum: 27001, Count: 5, Slot: 8},
			},
			Equipment: []inventory.ItemInstance{
				{ID: 2002, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
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
}
