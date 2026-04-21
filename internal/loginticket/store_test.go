package loginticket

import (
	"errors"
	"reflect"
	"testing"
	"time"
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
