package loginticket

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
)

func TestFileStoreIssueThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	issuedAt := time.Date(2026, 4, 17, 10, 21, 0, 0, time.UTC)
	want := Ticket{
		Login:    "mkmk",
		LoginKey: 0x01020304,
		Empire:   2,
		IssuedAt: issuedAt,
		Characters: []Character{
			{
				ID:          1,
				VID:         0x11111111,
				Name:        "MkmkWar",
				Job:         0,
				RaceNum:     0,
				Level:       15,
				PlayMinutes: 120,
				ST:          6,
				HT:          5,
				DX:          4,
				IQ:          3,
				MainPart:    101,
				ChangeName:  0,
				HairPart:    201,
				Dummy:       [4]byte{1, 2, 3, 4},
				X:           1000,
				Y:           2000,
				Z:           0,
				MapIndex:    21,
				Empire:      2,
				SkillGroup:  1,
				GuildID:     10,
				GuildName:   "Alpha",
				Gold:        125000,
				Inventory: []inventory.ItemInstance{
					{ID: 11, Vnum: 27001, Count: 3, Slot: 5, Locked: true},
				},
				Equipment: []inventory.ItemInstance{
					{ID: 22, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
				},
				Quickslots: []Quickslot{
					{Position: 3, Type: 1, Slot: 5},
				},
			},
		},
	}
	want.Characters[0].Points[0] = 15
	want.Characters[0].Points[1] = 1234

	if err := store.Issue(want); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}

	got, err := store.Load("mkmk", 0x01020304)
	if err != nil {
		t.Fatalf("load ticket: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ticket:\n got: %#v\nwant: %#v", got, want)
	}

	loadedAgain, err := store.Load("mkmk", 0x01020304)
	if err != nil {
		t.Fatalf("load ticket again: %v", err)
	}
	if !reflect.DeepEqual(loadedAgain, want) {
		t.Fatalf("unexpected ticket after second load:\n got: %#v\nwant: %#v", loadedAgain, want)
	}
}

func TestFileStoreIssueRejectsZeroCountInventoryItem(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ticket := Ticket{
		Login:    "mkmk",
		LoginKey: 0x01020304,
		Characters: []Character{{
			ID:        1,
			Name:      "MkmkWar",
			Inventory: []inventory.ItemInstance{{ID: 1001, Vnum: 27001, Count: 0, Slot: 8}},
		}},
	}

	if err := store.Issue(ticket); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for zero-count inventory item, got %v", err)
	}
}

func TestFileStoreLoadRejectsZeroCountInventoryItem(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","login_key":16909060,"issued_at":"2026-04-17T10:21:00Z","characters":[{"id":1,"name":"MkmkWar","inventory":[{"id":1001,"vnum":27001,"count":0,"slot":8}],"equipment":[],"quickslots":[]}]}`)
	if err := os.WriteFile(store.ticketPath(0x01020304), raw, 0o644); err != nil {
		t.Fatalf("write invalid zero-count ticket snapshot: %v", err)
	}

	_, err := store.Load("mkmk", 0x01020304)
	if !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for loaded zero-count inventory item, got %v", err)
	}
}

func TestFileStoreIssueRejectsDuplicateEquipmentSlots(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ticket := Ticket{
		Login:    "mkmk",
		LoginKey: 0x01020304,
		Characters: []Character{{
			ID:   1,
			Name: "MkmkWar",
			Equipment: []inventory.ItemInstance{
				{ID: 2001, Vnum: 19, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
				{ID: 2002, Vnum: 29, Count: 1, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
			},
		}},
	}

	if err := store.Issue(ticket); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for duplicate equipment slot, got %v", err)
	}
}

func TestFileStoreLoadRejectsDuplicateQuickslotPositions(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := []byte(`{"login":"mkmk","login_key":16909060,"issued_at":"2026-04-17T10:21:00Z","characters":[{"id":1,"name":"MkmkWar","inventory":[],"equipment":[],"quickslots":[{"position":3,"type":1,"slot":8},{"position":3,"type":2,"slot":9}]}]}`)
	if err := os.WriteFile(store.ticketPath(0x01020304), raw, 0o644); err != nil {
		t.Fatalf("write duplicate-quickslot ticket snapshot: %v", err)
	}

	_, err := store.Load("mkmk", 0x01020304)
	if !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for duplicate quickslot position, got %v", err)
	}
}

func TestFileStoreConsumeRejectsInvalidCharacterPayloadWithoutDeletingTicket(t *testing.T) {
	store := NewFileStore(t.TempDir())
	path := store.ticketPath(0x01020304)
	raw := []byte(`{"login":"mkmk","login_key":16909060,"issued_at":"2026-04-17T10:21:00Z","characters":[{"id":1,"name":"MkmkWar","inventory":[{"id":1001,"vnum":27001,"count":0,"slot":8}],"equipment":[],"quickslots":[]}]}`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write invalid ticket: %v", err)
	}

	_, err := store.Consume("mkmk", 0x01020304)
	if !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for invalid consume, got %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected invalid ticket to remain for inspection after failed consume, got %v", statErr)
	}
}

func TestFileStoreIssueThenConsumeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	issued := Ticket{Login: "mkmk", LoginKey: 0x01020304}
	if err := store.Issue(issued); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}

	got, err := store.Consume("mkmk", 0x01020304)
	if err != nil {
		t.Fatalf("consume ticket: %v", err)
	}
	if got.IssuedAt.IsZero() {
		t.Fatal("expected consumed ticket to include an issued_at timestamp")
	}
	issued.IssuedAt = got.IssuedAt
	if !reflect.DeepEqual(got, issued) {
		t.Fatalf("unexpected consumed ticket:\n got: %#v\nwant: %#v", got, issued)
	}
}

func TestFileStoreConsumeRejectsWrongLoginWithoutDeletingTheTicket(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	issued := Ticket{Login: "mkmk", LoginKey: 0x01020304}
	if err := store.Issue(issued); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}

	_, err := store.Consume("other", 0x01020304)
	if !errors.Is(err, ErrTicketLoginMismatch) {
		t.Fatalf("expected ErrTicketLoginMismatch, got %v", err)
	}

	got, err := store.Consume("mkmk", 0x01020304)
	if err != nil {
		t.Fatalf("consume ticket after mismatch: %v", err)
	}
	if got.Login != "mkmk" || got.LoginKey != 0x01020304 {
		t.Fatalf("unexpected ticket after mismatch: %#v", got)
	}
}

func TestFileStoreReturnsNotFoundAfterSuccessfulConsume(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	issued := Ticket{Login: "mkmk", LoginKey: 0x01020304}
	if err := store.Issue(issued); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}

	if _, err := store.Consume("mkmk", 0x01020304); err != nil {
		t.Fatalf("consume ticket: %v", err)
	}

	_, err := store.Consume("mkmk", 0x01020304)
	if !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("expected ErrTicketNotFound, got %v", err)
	}
}

func TestCloneCharactersDeepCopiesItemStateSlices(t *testing.T) {
	source := []Character{{
		ID:         1,
		Inventory:  []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
		Equipment:  []inventory.ItemInstance{{ID: 22, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}},
		Quickslots: []Quickslot{{Position: 3, Type: 1, Slot: 5}},
	}}

	cloned := CloneCharacters(source)
	cloned[0].Inventory[0].Count = 7
	cloned[0].Equipment[0].Vnum = 29
	cloned[0].Quickslots[0].Slot = 6

	if source[0].Inventory[0].Count != 3 {
		t.Fatalf("expected source inventory to stay unchanged, got %#v", source[0].Inventory)
	}
	if source[0].Equipment[0].Vnum != 19 {
		t.Fatalf("expected source equipment to stay unchanged, got %#v", source[0].Equipment)
	}
	if source[0].Quickslots[0].Slot != 5 {
		t.Fatalf("expected source quickslots to stay unchanged, got %#v", source[0].Quickslots)
	}
}

func TestFileStoreIssueDoesNotMutateCallerItemState(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ticket := Ticket{
		Login:    "mkmk",
		LoginKey: 0x01020304,
		Characters: []Character{{
			ID:   1,
			Name: "MkmkWar",
		}},
	}

	if err := store.Issue(ticket); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}
	if ticket.Characters[0].Inventory != nil {
		t.Fatalf("expected caller inventory slice to remain nil, got %#v", ticket.Characters[0].Inventory)
	}
	if ticket.Characters[0].Equipment != nil {
		t.Fatalf("expected caller equipment slice to remain nil, got %#v", ticket.Characters[0].Equipment)
	}
	if ticket.Characters[0].Quickslots != nil {
		t.Fatalf("expected caller quickslots slice to remain nil, got %#v", ticket.Characters[0].Quickslots)
	}
}

func TestFileStoreIssuePersistsEmptyItemStateAsArrays(t *testing.T) {
	store := NewFileStore(t.TempDir())
	ticket := Ticket{
		Login:    "mkmk",
		LoginKey: 0x01020304,
		Characters: []Character{{
			ID:   1,
			Name: "MkmkWar",
		}},
	}

	if err := store.Issue(ticket); err != nil {
		t.Fatalf("issue ticket: %v", err)
	}

	raw, err := os.ReadFile(store.ticketPath(0x01020304))
	if err != nil {
		t.Fatalf("read issued ticket: %v", err)
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

func TestFileStoreLoadNormalizesMissingItemStateFromLegacySnapshot(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	legacyRaw := []byte("{\"login\":\"mkmk\",\"login_key\":16909060,\"issued_at\":\"2026-04-17T10:21:00Z\",\"characters\":[{\"id\":1,\"name\":\"MkmkWar\",\"empire\":2}]}")
	if err := os.WriteFile(store.ticketPath(0x01020304), legacyRaw, 0o644); err != nil {
		t.Fatalf("write legacy ticket: %v", err)
	}

	got, err := store.Load("mkmk", 0x01020304)
	if err != nil {
		t.Fatalf("load ticket: %v", err)
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

func TestFileStoreLoadRejectsUnknownTicketFields(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := mustJSONWithUnknownField(t, Ticket{Login: "mkmk", LoginKey: 0x01020304, IssuedAt: time.Date(2026, 4, 17, 10, 21, 0, 0, time.UTC)}, "schema_version", 99)
	if err := os.WriteFile(store.ticketPath(0x01020304), raw, 0o644); err != nil {
		t.Fatalf("write unknown-field ticket: %v", err)
	}

	_, err := store.Load("mkmk", 0x01020304)
	if !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for unknown ticket field, got %v", err)
	}
}

func TestFileStoreLoadRejectsTrailingJSONValue(t *testing.T) {
	store := NewFileStore(t.TempDir())
	raw := mustJSON(t, Ticket{Login: "mkmk", LoginKey: 0x01020304, IssuedAt: time.Date(2026, 4, 17, 10, 21, 0, 0, time.UTC)})
	raw = append(raw, ' ')
	raw = append(raw, mustJSON(t, Ticket{Login: "shadow", LoginKey: 0x01020304})...)
	if err := os.WriteFile(store.ticketPath(0x01020304), raw, 0o644); err != nil {
		t.Fatalf("write trailing-json ticket: %v", err)
	}

	_, err := store.Load("mkmk", 0x01020304)
	if !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for trailing JSON value, got %v", err)
	}
}

func TestFileStoreConsumeRejectsInvalidTicketWithoutDeletingIt(t *testing.T) {
	store := NewFileStore(t.TempDir())
	path := store.ticketPath(0x01020304)
	raw := mustJSONWithUnknownField(t, Ticket{Login: "mkmk", LoginKey: 0x01020304, IssuedAt: time.Date(2026, 4, 17, 10, 21, 0, 0, time.UTC)}, "schema_version", 99)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write invalid ticket: %v", err)
	}

	_, err := store.Consume("mkmk", 0x01020304)
	if !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expected ErrInvalidTicket for invalid consume, got %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected invalid ticket to remain for inspection after failed consume, got %v", statErr)
	}
}

func mustJSONWithUnknownField(t *testing.T, ticket Ticket, field string, value any) []byte {
	t.Helper()
	raw := mustJSON(t, ticket)
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("unmarshal ticket object: %v", err)
	}
	object[field] = value
	return mustJSON(t, object)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return raw
}
