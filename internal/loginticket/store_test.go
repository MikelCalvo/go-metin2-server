package loginticket

import (
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
					{ID: 11, Vnum: 27001, Count: 3, Slot: 5},
				},
				Equipment: []inventory.ItemInstance{
					{ID: 22, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon},
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
		ID:        1,
		Inventory: []inventory.ItemInstance{{ID: 11, Vnum: 27001, Count: 3, Slot: 5}},
		Equipment: []inventory.ItemInstance{{ID: 22, Vnum: 19, Count: 1, Slot: 0, Equipped: true, EquipSlot: inventory.EquipmentSlotWeapon}},
	}}

	cloned := CloneCharacters(source)
	cloned[0].Inventory[0].Count = 7
	cloned[0].Equipment[0].Vnum = 29

	if source[0].Inventory[0].Count != 3 {
		t.Fatalf("expected source inventory to stay unchanged, got %#v", source[0].Inventory)
	}
	if source[0].Equipment[0].Vnum != 19 {
		t.Fatalf("expected source equipment to stay unchanged, got %#v", source[0].Equipment)
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
}
